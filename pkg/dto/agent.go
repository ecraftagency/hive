package dto

import "hive/pkg/store"

// Request DTOs
type SubmitTicketRequest struct {
	PlayerID string `json:"player_id" binding:"required"`
}

type CancelTicketRequest struct {
	// Empty body for now, could add reason later
}

// Response DTOs
type SubmitTicketResponse struct {
	TicketID string `json:"ticket_id"`
	Status   string `json:"status"`
}

type TicketStatusResponse struct {
	Status string `json:"status"`
	RoomID string `json:"room_id,omitempty"`
}

type CancelTicketResponse struct {
	Status string `json:"status"`
}

type AdminOverviewResponse struct {
	OpenTickets    []store.Ticket    `json:"open_tickets"`
	OpenedRooms    []store.RoomState `json:"opened_rooms"`
	FulfilledRooms []store.RoomState `json:"fulfilled_rooms"`
	DeadRooms      []store.RoomState `json:"dead_rooms"`
}

// Error responses
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Error     string `json:"error"`
}

// Custom error type with error code
type AgentError struct {
	Code    string
	Message string
}

func (e *AgentError) Error() string {
	return e.Message
}

func NewAgentError(code, message string) *AgentError {
	return &AgentError{Code: code, Message: message}
}

// Helper function to convert AgentError to ErrorResponse
func ToErrorResponse(err error) ErrorResponse {
	if agentErr, ok := err.(*AgentError); ok {
		return ErrorResponse{
			ErrorCode: agentErr.Code,
			Error:     agentErr.Message,
		}
	}
	// Default error response for unknown errors
	return ErrorResponse{
		ErrorCode: ErrCodeInternalError,
		Error:     err.Error(),
	}
}

// Error codes constants
const (
	// Validation errors (400)
	ErrCodeMissingPlayerID = "MISSING_PLAYER_ID"
	ErrCodeMissingRoomID   = "MISSING_ROOM_ID"
	ErrCodeInvalidRequest  = "INVALID_REQUEST"

	// Not found errors (404)
	ErrCodeTicketNotFound = "TICKET_NOT_FOUND"
	ErrCodeRoomNotFound   = "ROOM_NOT_FOUND"
	ErrCodeRoomNotReady   = "ROOM_NOT_READY"

	// Business logic errors (400)
	ErrCodeTicketRejected     = "TICKET_REJECTED"
	ErrCodeTicketCancelFailed = "TICKET_CANCEL_FAILED"

	// Server errors (500)
	ErrCodeInternalError = "INTERNAL_ERROR"
	ErrCodeRedisError    = "REDIS_ERROR"
	ErrCodeNomadError    = "NOMAD_ERROR"

	// Gateway errors (502)
	ErrCodeGatewayError = "GATEWAY_ERROR"

	// Additional error codes
	ErrCodeNoRunningAllocation = "NO_RUNNING_ALLOCATION"
	ErrCodeAllocationTimeout   = "ALLOCATION_TIMEOUT"
)

// Legacy proxy response
type ProxyHeartbeatResponse struct {
	OK bool `json:"ok"`
}
