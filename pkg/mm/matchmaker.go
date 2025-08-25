package mm

import (
	"context"
	"time"

	"hive/pkg/store"
	"hive/pkg/svrmgr"

	"github.com/google/uuid"
)

type Manager struct {
	store          *store.Manager
	svr            *svrmgr.Manager
	allocTimeout   time.Duration
	pollInterval   time.Duration
	executablePath string
}

func New(storeMgr *store.Manager, svrMgr *svrmgr.Manager, executablePath string) *Manager {
	return &Manager{
		store:          storeMgr,
		svr:            svrMgr,
		allocTimeout:   2 * time.Minute,
		pollInterval:   2 * time.Second,
		executablePath: executablePath,
	}
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
		// allocate job với command mới
		command := m.executablePath
		// command := "/usr/local/bin/tanknarok/TanknarokServer"
		//args := []string{"-port", "${NOMAD_PORT_http}", "-serverId", rid, "-token", "1234abcd", "-nographics", "-batchmode", "-agentUrl", "https://agent.zensoftstudio.com"}
		args := []string{
			"-serverId", rid,
			"-token", "1234abcd",
			"-nographics",
			"-batchmode",
			"-agentUrl", "https://agent.zensoftstudio.com",
			"-serverPort", "${NOMAD_PORT_http}",
		}

		if err := m.svr.RunGameServerV2(rid, 400, 400, command, args); err != nil {
			// Plan có thể fail ở đây → DEAD ngay với lý do
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
					// Trước khi ACTIVED, enforce uniqueness: không cho phép player nằm ở 2 ACTIVED rooms
					conflict := false
					if ids, lerr := m.store.ListRooms(context.Background()); lerr == nil {
						for _, oid := range ids {
							if oid == rid {
								continue
							}
							if ost, ge := m.store.GetRoomState(context.Background(), oid); ge == nil && ost != nil && ost.Status == "ACTIVED" {
								for _, op := range ost.Players {
									for _, np := range plist {
										if op == np {
											conflict = true
											break
										}
									}
								}
							}
						}
					}
					if conflict {
						// Đánh dấu phòng mới DEAD để tránh trùng player
						_ = m.store.SaveRoomState(context.Background(), store.RoomState{RoomID: rid, Players: plist, CreatedAt: created, Status: "DEAD", FailReason: "duplicate_player_active"})
						_ = m.svr.DeregisterJob(rid, false)
						return
					}
					// Server is ready when Nomad job is running and we have IP/port
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
			time.Sleep(m.pollInterval)
		}
		// timeout → DEAD và dừng job (không purge để có thể inspect sau)
		_ = m.store.SaveRoomState(context.Background(), store.RoomState{RoomID: rid, Players: plist, CreatedAt: created, Status: "DEAD", FailReason: "alloc_timeout"})
		_ = m.svr.DeregisterJob(rid, false)
	}(roomID, players, createdAt)

	return &store.RoomState{RoomID: roomID, Players: players, CreatedAt: createdAt, Status: "OPENED"}, nil
}

// probeReady function removed - we trust Nomad job status instead
