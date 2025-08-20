package mm

import (
	"context"
	"fmt"
	"net"
	"time"

	"hive/pkg/store"
	"hive/pkg/svrmgr"

	"github.com/google/uuid"
)

type Manager struct {
	store        *store.Manager
	svr          *svrmgr.Manager
	allocTimeout time.Duration
	pollInterval time.Duration
}

func New(storeMgr *store.Manager, svrMgr *svrmgr.Manager) *Manager {
	return &Manager{store: storeMgr, svr: svrMgr, allocTimeout: 2 * time.Minute, pollInterval: 2 * time.Second}
}

// SubmitJoinTicket: tạo ticket OPENED cho player
func (m *Manager) SubmitJoinTicket(ctx context.Context, playerID string) (*store.Ticket, error) {
	return m.store.CreateTicket(ctx, playerID)
}

// GetTicket: trả ticket theo id
func (m *Manager) GetTicket(ctx context.Context, ticketID string) (*store.Ticket, error) {
	return m.store.GetTicket(ctx, ticketID)
}

// CancelTicket: hủy ticket OPENED
func (m *Manager) CancelTicket(ctx context.Context, ticketID string) error {
	return m.store.CancelTicket(ctx, ticketID)
}

// TryMatch: ghép 2 ticket và tạo room OPENED, allocate server async
func (m *Manager) TryMatch(ctx context.Context) (*store.RoomState, error) {
	t1, t2, err := m.store.TryMatchPair(ctx)
	if err != nil {
		return nil, err
	}
	roomID := uuid.New().String()
	players := []string{t1.PlayerID, t2.PlayerID}
	// mark matched
	_ = m.store.MarkMatched(ctx, t1.TicketID, roomID)
	_ = m.store.MarkMatched(ctx, t2.TicketID, roomID)
	// save OPENED room
	createdAt := time.Now().Unix()
	_ = m.store.SaveRoomState(ctx, store.RoomState{RoomID: roomID, Players: players, CreatedAt: createdAt, Status: "OPENED"})
	// allocate async
	go func(rid string, plist []string, created int64) {
		// allocate job
		if err := m.svr.RunGameServer(rid); err != nil {
			_ = m.store.SaveRoomState(context.Background(), store.RoomState{RoomID: rid, Players: plist, CreatedAt: created, Status: "DEAD", FailReason: err.Error()})
			return
		}
		// double-check allocation readiness within allocTimeout
		deadline := time.Now().Add(m.allocTimeout)
		for time.Now().Before(deadline) {
			info, e := m.svr.GetRoomInfo(rid)
			if e == nil && info != nil && info.HostIP != "" && len(info.Ports) > 0 {
				// choose port
				port := 0
				if v, ok := info.Ports["http"]; ok && v > 0 {
					port = v
				} else {
					for _, vv := range info.Ports {
						if vv > 0 {
							port = vv
							break
						}
					}
				}
				if port > 0 {
					if probeReady(info.HostIP, port) {
						_ = m.store.SaveRoomState(context.Background(), store.RoomState{
							RoomID:       rid,
							AllocationID: info.AllocationID,
							ServerIP:     info.HostIP,
							Port:         port,
							Players:      plist,
							CreatedAt:    created,
							Status:       "ACTIVED",
						})
						return
					}
				}
			}
			time.Sleep(m.pollInterval)
		}
		// timeout → DEAD
		_ = m.store.SaveRoomState(context.Background(), store.RoomState{RoomID: rid, Players: plist, CreatedAt: created, Status: "DEAD", FailReason: "alloc_timeout"})
	}(roomID, players, createdAt)

	return &store.RoomState{RoomID: roomID, Players: players, CreatedAt: createdAt, Status: "OPENED"}, nil
}

// probeReady: double-check readiness with two fast TCP dials
func probeReady(host string, port int) bool {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	c1, e1 := net.DialTimeout("tcp", addr, 2*time.Second)
	if e1 != nil {
		return false
	}
	_ = c1.Close()
	time.Sleep(500 * time.Millisecond)
	c2, e2 := net.DialTimeout("tcp", addr, 2*time.Second)
	if e2 != nil {
		return false
	}
	_ = c2.Close()
	return true
}
