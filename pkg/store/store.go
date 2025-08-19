package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RoomState struct {
	RoomID       string   `json:"room_id"`
	AllocationID string   `json:"allocation_id"`
	ServerIP     string   `json:"server_ip"`
	Port         int      `json:"port"`
	Players      []string `json:"players"`
	CreatedAt    int64    `json:"created_at_unix"`
}

type PendingCreate struct {
	RoomName  string `json:"room_name"`
	PlayerID  string `json:"player_id"`
	EnqueueAt int64  `json:"enqueue_at_unix"`
}

type Manager struct {
	redis *redis.Client
}

func New(redisAddr string) *Manager {
	cli := redis.NewClient(&redis.Options{Addr: redisAddr})
	return &Manager{redis: cli}
}

// Pending queue operations
const pendingQueueKey = "mm:pending_queue"
const pendingPlayersSet = "mm:pending_players"

// EnqueuePending: đảm bảo không trùng player_id đang có vé chờ
func (m *Manager) EnqueuePending(ctx context.Context, p PendingCreate) error {
	added, err := m.redis.SAdd(ctx, pendingPlayersSet, p.PlayerID).Result()
	if err != nil {
		return err
	}
	if added == 0 {
		return fmt.Errorf("duplicate ticket for player %s", p.PlayerID)
	}
	b, _ := json.Marshal(p)
	if err := m.redis.RPush(ctx, pendingQueueKey, string(b)).Err(); err != nil {
		// rollback set nếu push queue lỗi
		_ = m.redis.SRem(ctx, pendingPlayersSet, p.PlayerID).Err()
		return err
	}
	return nil
}

func (m *Manager) DequeuePending(ctx context.Context) (*PendingCreate, error) {
	v, err := m.redis.LPop(ctx, pendingQueueKey).Result()
	if err != nil {
		return nil, err
	}
	var p PendingCreate
	_ = json.Unmarshal([]byte(v), &p)
	// xóa đánh dấu player khỏi set pending
	_ = m.redis.SRem(ctx, pendingPlayersSet, p.PlayerID).Err()
	return &p, nil
}

// ListPending liệt kê toàn bộ pending mà không tiêu thụ
func (m *Manager) ListPending(ctx context.Context) ([]PendingCreate, error) {
	vals, err := m.redis.LRange(ctx, pendingQueueKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	res := make([]PendingCreate, 0, len(vals))
	for _, v := range vals {
		var p PendingCreate
		if err := json.Unmarshal([]byte(v), &p); err == nil {
			res = append(res, p)
		}
	}
	return res, nil
}

// Room state operations
func roomKey(roomID string) string { return "mm:room:" + roomID }
func roomsIndexKey() string        { return "mm:rooms" }

func (m *Manager) SaveRoomState(ctx context.Context, st RoomState) error {
	b, _ := json.Marshal(st)
	pipe := m.redis.TxPipeline()
	pipe.Set(ctx, roomKey(st.RoomID), string(b), 0)
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
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return m.redis.Ping(ctx).Err()
}
