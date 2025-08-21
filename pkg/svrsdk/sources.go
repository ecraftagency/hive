package svrsdk

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// SignalSource phát tín hiệu shutdown khi nhận SIGINT/SIGTERM
type SignalSource struct{}

func (s *SignalSource) Start(emit func(ShutdownEvent)) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	stopped := make(chan struct{})
	go func() {
		select {
		case sig := <-ch:
			_ = sig // chỉ để log nếu cần
			emit(NewEvent(ReasonSignal))
		case <-stopped:
			return
		}
	}()
	return func() { close(stopped); signal.Stop(ch) }
}

// HeartbeatSource kiểm tra theo dõi số players để quyết định shutdown
type HeartbeatSource struct {
	InitialGrace time.Duration
	HeartbeatTTL time.Duration // TTL chỉ để tham khảo, logic cụ thể nằm ở GetStats
	PollInterval time.Duration
	// GetStats trả: tổng số người chơi hiện tại, và có ai bị disconnect theo TTL hay không
	GetStats func() (size int, anyDisconnected bool)
}

func (h *HeartbeatSource) Start(emit func(ShutdownEvent)) (stop func()) {
	if h.PollInterval <= 0 {
		h.PollInterval = time.Second
	}
	stopCh := make(chan struct{})
	go func() {
		// Đợi initial grace
		if h.InitialGrace > 0 {
			t := time.NewTimer(h.InitialGrace)
			select {
			case <-t.C:
			case <-stopCh:
				return
			}
		}
		// Nếu không có người chơi nào
		if h.GetStats != nil {
			size, _ := h.GetStats()
			if size == 0 {
				emit(NewEvent(ReasonNoClients))
				return
			}
		}
		ticker := time.NewTicker(h.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if h.GetStats != nil {
					_, any := h.GetStats()
					if any {
						emit(NewEvent(ReasonClientDisconnected))
						return
					}
				}
			case <-stopCh:
				return
			}
		}
	}()
	return func() { close(stopCh) }
}
