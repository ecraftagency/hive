package mm

import (
	"context"
	"errors"
	"time"

	"hive/pkg/store"
	"hive/pkg/svrmgr"
)

type Manager struct {
	store *store.Manager
	svr   *svrmgr.Manager
}

func New(storeMgr *store.Manager, svrMgr *svrmgr.Manager) *Manager {
	return &Manager{store: storeMgr, svr: svrMgr}
}

// CreateRoom chỉ enqueue yêu cầu tạo phòng, chưa liên hệ Nomad
func (m *Manager) CreateRoom(ctx context.Context, roomName, playerID string) error {
	return m.store.EnqueuePending(ctx, store.PendingCreate{RoomName: roomName, PlayerID: playerID, EnqueueAt: time.Now().Unix()})
}

// JoinRoom tìm pending request để ghép
// Sau khi submit job, trả về ngay room_id và players; một goroutine sẽ poll và cập nhật RoomState khi alloc sẵn sàng
func (m *Manager) JoinRoom(ctx context.Context, playerID string) (*store.RoomState, error) {
	p, err := m.store.DequeuePending(ctx)
	if err != nil {
		// không có pending
		return nil, errors.New("no pending rooms")
	}
	if p == nil {
		return nil, errors.New("no pending rooms")
	}
	roomID := p.RoomName
	// chạy job trên Nomad
	if err := m.svr.RunGameServer(roomID); err != nil {
		return nil, err
	}
	players := []string{p.PlayerID, playerID}
	// Trả về ngay; thông tin server sẽ được cập nhật sau
	partial := &store.RoomState{
		RoomID:    roomID,
		Players:   players,
		CreatedAt: time.Now().Unix(),
	}
	// Lưu partial để vòng sync không xóa ngay
	_ = m.store.SaveRoomState(context.Background(), *partial)
	// Nền: poll nomad lấy alloc/server info và lưu vào Redis khi sẵn sàng
	go func(rid string, plist []string, created int64) {
		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			info, err := m.svr.GetRoomInfo(rid)
			if err == nil && info != nil && info.HostIP != "" && len(info.Ports) > 0 {
				port := 0
				if v, ok := info.Ports["http"]; ok {
					port = v
				} else {
					for _, vv := range info.Ports {
						port = vv
						break
					}
				}
				_ = m.store.SaveRoomState(context.Background(), store.RoomState{
					RoomID:       rid,
					AllocationID: info.AllocationID,
					ServerIP:     info.HostIP,
					Port:         port,
					Players:      plist,
					CreatedAt:    created,
				})
				return
			}
			time.Sleep(2 * time.Second)
		}
	}(roomID, players, partial.CreatedAt)

	return partial, nil
}
