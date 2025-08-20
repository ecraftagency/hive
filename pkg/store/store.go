package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RoomState struct {
	RoomID       string   `json:"room_id"`
	AllocationID string   `json:"allocation_id"`
	ServerIP     string   `json:"server_ip"`
	Port         int      `json:"port"`
	Players      []string `json:"players"`
	CreatedAt    int64    `json:"created_at_unix"`
	Status       string   `json:"status,omitempty"`
	FailReason   string   `json:"fail_reason,omitempty"`
	EndReason    string   `json:"end_reason,omitempty"`
	FulfilledAt  int64    `json:"fulfilled_at_unix,omitempty"`
	DeadAt       int64    `json:"dead_at_unix,omitempty"`
	GracefulAt   int64    `json:"graceful_at_unix,omitempty"`
}

type PendingCreate struct {
	RoomName  string `json:"room_name"`
	PlayerID  string `json:"player_id"`
	EnqueueAt int64  `json:"enqueue_at_unix"`
}

type Ticket struct {
	TicketID  string `json:"ticket_id"`
	PlayerID  string `json:"player_id"`
	Status    string `json:"status"` // OPENED|MATCHED|EXPIRED|REJECTED
	RoomID    string `json:"room_id,omitempty"`
	EnqueueAt int64  `json:"enqueue_at_unix"`
}

type Manager struct {
	redis *redis.Client
}

func New(redisAddr string) *Manager {
	cli := redis.NewClient(&redis.Options{Addr: redisAddr})
	return &Manager{redis: cli}
}

// Legacy Pending queue operations (kept for compat)
const pendingQueueKey = "mm:pending_queue"
const pendingPlayersSet = "mm:pending_players"

// New Ticket keys
const (
	openedTicketsKey = "mm:tickets:opened"  // LIST of ticket_id
	ticketKeyPrefix  = "mm:ticket:"         // mm:ticket:<ticket_id>
	playersPending   = "mm:players:pending" // SET of player_id with OPENED tickets
)

// ticketTTL will be set from config, default 120s
var ticketTTL = 120 * time.Second

// allocationTimeout controls how long an OPENED room can live before being considered DEAD
var allocationTimeout = 90 * time.Second

// terminalTTL controls how long to retain terminal rooms (DEAD, FULFILLED)
var terminalTTL = 60 * time.Second

// SetTicketTTL sets the ticket TTL from config
func SetTicketTTL(ttl time.Duration) {
	ticketTTL = ttl
}

// SetAllocationTimeout sets how long an OPENED room is kept before timing out
func SetAllocationTimeout(ttl time.Duration) { allocationTimeout = ttl }

// SetTerminalTTL sets how long terminal room states are kept
func SetTerminalTTL(ttl time.Duration) { terminalTTL = ttl }

// CreateTicket (join): returns ticket
func (m *Manager) CreateTicket(ctx context.Context, playerID string) (*Ticket, error) {
	// prevent duplicate player
	added, err := m.redis.SAdd(ctx, playersPending, playerID).Result()
	if err != nil {
		return nil, err
	}
	if added == 0 {
		return nil, fmt.Errorf("duplicate ticket for player %s", playerID)
	}
	tid := uuid.New().String()
	t := &Ticket{TicketID: tid, PlayerID: playerID, Status: "OPENED", EnqueueAt: time.Now().Unix()}
	b, _ := json.Marshal(t)
	pipe := m.redis.TxPipeline()
	pipe.RPush(ctx, openedTicketsKey, tid)
	pipe.Set(ctx, ticketKeyPrefix+tid, string(b), ticketTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		_ = m.redis.SRem(ctx, playersPending, playerID).Err()
		return nil, err
	}
	return t, nil
}

func (m *Manager) GetTicket(ctx context.Context, ticketID string) (*Ticket, error) {
	v, err := m.redis.Get(ctx, ticketKeyPrefix+ticketID).Result()
	if err != nil {
		return nil, err
	}
	var t Ticket
	_ = json.Unmarshal([]byte(v), &t)
	return &t, nil
}

func (m *Manager) setTicket(ctx context.Context, t *Ticket) error {
	b, _ := json.Marshal(t)
	return m.redis.Set(ctx, ticketKeyPrefix+t.TicketID, string(b), ticketTTL).Err()
}

// CancelTicket: only when OPENED
func (m *Manager) CancelTicket(ctx context.Context, ticketID string) error {
	t, err := m.GetTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if t.Status != "OPENED" {
		return fmt.Errorf("cannot cancel: status=%s", t.Status)
	}
	pipe := m.redis.TxPipeline()
	pipe.LRem(ctx, openedTicketsKey, 0, ticketID)
	pipe.Del(ctx, ticketKeyPrefix+ticketID)
	pipe.SRem(ctx, playersPending, t.PlayerID)
	_, err = pipe.Exec(ctx)
	return err
}

// TryMatchPair: naive (non-atomic) pop 2 tickets; caller must handle errors
func (m *Manager) TryMatchPair(ctx context.Context) (*Ticket, *Ticket, error) {
	tid1, err1 := m.redis.LPop(ctx, openedTicketsKey).Result()
	if err1 != nil {
		return nil, nil, err1
	}
	tid2, err2 := m.redis.LPop(ctx, openedTicketsKey).Result()
	if err2 != nil {
		// push back tid1 to head to avoid loss
		_ = m.redis.LPush(ctx, openedTicketsKey, tid1).Err()
		return nil, nil, err2
	}
	t1, e1 := m.GetTicket(ctx, tid1)
	t2, e2 := m.GetTicket(ctx, tid2)
	if e1 != nil || e2 != nil {
		return nil, nil, fmt.Errorf("failed to load tickets")
	}
	return t1, t2, nil
}

// MarkMatched updates ticket with room_id and status
func (m *Manager) MarkMatched(ctx context.Context, ticketID, roomID string) error {
	t, err := m.GetTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	t.Status = "MATCHED"
	t.RoomID = roomID
	if err := m.setTicket(ctx, t); err != nil {
		return err
	}
	// allow player submit new ticket later
	_ = m.redis.SRem(ctx, playersPending, t.PlayerID).Err()
	return nil
}

// Rooms helpers
func roomKey(roomID string) string { return "mm:room:" + roomID }
func roomsIndexKey() string        { return "mm:rooms" }

func (m *Manager) SaveRoomState(ctx context.Context, st RoomState) error {
	b, _ := json.Marshal(st)
	pipe := m.redis.TxPipeline()
	// TTL theo trạng thái
	var exp time.Duration
	switch st.Status {
	case "OPENED":
		exp = allocationTimeout
	case "DEAD", "FULFILLED":
		exp = terminalTTL
	default: // ACTIVED hoặc không xác định
		exp = 0
	}
	pipe.Set(ctx, roomKey(st.RoomID), string(b), exp)
	pipe.SAdd(ctx, roomsIndexKey(), st.RoomID)
	_, err := pipe.Exec(ctx)
	return err
}

func (m *Manager) DeleteRoomState(ctx context.Context, roomID string) error {
	pipe := m.redis.TxPipeline()
	pipe.Del(ctx, roomKey(roomID))
	pipe.SRem(ctx, roomsIndexKey(), roomID)
	_, err := pipe.Exec(ctx)
	return err
}

func (m *Manager) GetRoomState(ctx context.Context, roomID string) (*RoomState, error) {
	v, err := m.redis.Get(ctx, roomKey(roomID)).Result()
	if err != nil {
		return nil, err
	}
	var st RoomState
	_ = json.Unmarshal([]byte(v), &st)
	return &st, nil
}

func (m *Manager) ListRooms(ctx context.Context) ([]string, error) {
	return m.redis.SMembers(ctx, roomsIndexKey()).Result()
}

// Health check simple ping with timeout
func (m *Manager) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second) // TODO: use config timeout
	defer cancel()
	return m.redis.Ping(ctx).Err()
}

// ListOpenedTicketIDs: trả về danh sách ticket_id đang nằm trong queue OPENED
func (m *Manager) ListOpenedTicketIDs(ctx context.Context) ([]string, error) {
	return m.redis.LRange(ctx, openedTicketsKey, 0, -1).Result()
}

// ListOpenedTickets: lấy chi tiết ticket theo danh sách OPENED
func (m *Manager) ListOpenedTickets(ctx context.Context) ([]Ticket, error) {
	ids, err := m.ListOpenedTicketIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Ticket, 0, len(ids))
	for _, id := range ids {
		v, e := m.redis.Get(ctx, ticketKeyPrefix+id).Result()
		if e != nil {
			continue
		}
		var t Ticket
		if json.Unmarshal([]byte(v), &t) == nil && t.Status == "OPENED" {
			out = append(out, t)
		}
	}
	return out, nil
}
