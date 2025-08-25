package svrsdk

import (
	"context"
	"time"
)

// ShutdownReason định nghĩa các lý do shutdown chuẩn
type ShutdownReason string

const (
	ReasonNoClients          ShutdownReason = "no_clients"
	ReasonClientDisconnected ShutdownReason = "client_disconnected"
	ReasonSignal             ShutdownReason = "signal_received"
	ReasonAfkTimeout         ShutdownReason = "afk_timeout"
	ReasonGameCycleCompleted ShutdownReason = "game_cycle_completed"
)

// Config cấu hình chung cho SDK (được load từ ENV/flags)
type Config struct {
	Port         string
	RoomID       string
	Token        string
	AgentBaseURL string
	NoGraphics   bool
	BatchMode    bool
	ServerPort   string // Port for HTTP heartbeat server (optional)
}

// ShutdownEvent mô tả sự kiện shutdown có thể kèm payload chi tiết
// Details có thể chứa thông tin người chơi/điểm số khi endgame
type ShutdownEvent struct {
	Reason  ShutdownReason `json:"reason"`
	At      int64          `json:"at"`
	Details map[string]any `json:"details,omitempty"`
}

// ShutdownContext truyền vào pipeline xử lý
type ShutdownContext struct {
	Ctx    context.Context
	Config Config
	Event  *ShutdownEvent
}

// ShutdownDecision kết quả từ middleware/handler
type ShutdownDecision int

const (
	DecisionContinue ShutdownDecision = iota
	DecisionModify
	DecisionCancel
)

// ShutdownResult biểu diễn quyết định của middleware/handler
type ShutdownResult struct {
	Decision ShutdownDecision
	Event    *ShutdownEvent
	Err      error
}

// ShutdownSource phát sinh sự kiện shutdown (signal, heartbeat, custom)
type ShutdownSource interface {
	Start(emit func(ShutdownEvent)) (stop func())
}

// ShutdownNotifier gửi thông báo shutdown tới Agent (hoặc nơi khác)
type ShutdownNotifier interface {
	Notify(ctx context.Context, cfg Config, ev ShutdownEvent) error
}

// ShutdownHandler và Middleware cho phép override/pluggable pipeline
type ShutdownHandler func(*ShutdownContext) ShutdownResult

type Middleware func(next ShutdownHandler) ShutdownHandler

// helper tạo event
func NewEvent(reason ShutdownReason) ShutdownEvent {
	return ShutdownEvent{Reason: reason, At: time.Now().Unix()}
}
