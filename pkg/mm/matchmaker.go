package mm

import (
	"context"
	"time"

	"hive/pkg/store"
	"hive/pkg/svrmgr"

	"github.com/google/uuid"
)

type Manager struct {
	store *store.Manager
	svr   *svrmgr.Manager
}

func New(storeMgr *store.Manager, svrMgr *svrmgr.Manager) *Manager {
	return &Manager{store: storeMgr, svr: svrMgr}
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
	_ = m.store.SaveRoomState(ctx, store.RoomState{RoomID: roomID, Players: players, CreatedAt: time.Now().Unix(), Status: "OPENED"})
	// allocate async
	go func(rid string, plist []string, created int64) {
		// allocation timeout handled by cron (per doc); here just attempt allocate
		if err := m.svr.RunGameServer(rid); err != nil {
			_ = m.store.SaveRoomState(context.Background(), store.RoomState{RoomID: rid, Players: plist, CreatedAt: created, Status: "DEAD", FailReason: err.Error()})
			return
		}
		// poll room info until allocated and server is running
		deadline := time.Now().Add(2 * time.Minute) // TODO: use config allocation timeout
		for time.Now().Before(deadline) {
			info, e := m.svr.GetRoomInfo(rid)
			if e == nil && info != nil && info.HostIP != "" && len(info.Ports) > 0 {
				port := 0
				if v, ok := info.Ports["http"]; ok && v > 0 {
					port = v
				} else {
					// find first valid port
					for _, vv := range info.Ports {
						if vv > 0 {
							port = vv
							break
						}
					}
				}
				// ensure we have valid IP and port before marking FULFILLED
				if port > 0 && info.HostIP != "" {
					_ = m.store.SaveRoomState(context.Background(), store.RoomState{
						RoomID:       rid,
						AllocationID: info.AllocationID,
						ServerIP:     info.HostIP,
						Port:         port,
						Players:      plist,
						CreatedAt:    created,
						Status:       "FULFILLED",
					})
					return
				}
			}
			time.Sleep(2 * time.Second) // TODO: use config poll delay
		}
		// leave OPENED for cron to timeout -> DEAD
	}(roomID, players, time.Now().Unix())

	return &store.RoomState{RoomID: roomID, Players: players, CreatedAt: time.Now().Unix(), Status: "OPENED"}, nil
}
