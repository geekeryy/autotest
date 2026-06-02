// Package aisession persists conversational sessions between the global
// AI assistant and users. Each session belongs to a single (project, user)
// pair; cross-user reads/writes are rejected at the service layer.
//
// Why a dedicated package rather than tucking this under aiprovider?
//   - aiprovider focuses on provider/protocol concerns and is reused by
//     several callers (analysis, chat). Adding a sessions table to it would
//     blur that boundary.
//   - The assistant chat is the first feature that needs persisted
//     conversation state, so keeping it in its own package leaves room for
//     additional session-flavored features (e.g. saved prompts, transcripts).
package aisession

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors surfaced to the service / handler layers.
var (
	ErrSessionNotFound  = errors.New("ai 会话不存在或无权限访问")
	ErrPendingNotFound  = errors.New("待确认的工具调用不存在")
	ErrPendingDecided   = errors.New("该工具调用已被确认或拒绝")
	ErrInvalidSeq       = errors.New("消息序号冲突，请刷新会话")
	ErrSessionForbidden = errors.New("当前会话不属于本用户")
)

// Message status values mirroring the CHECK constraint on ai_messages.status.
const (
	StatusFinal          = "final"
	StatusPendingConfirm = "pending_confirm"
	StatusRejected       = "rejected"
)

// Session is the API-facing representation of ai_sessions.
type Session struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	UserID    uuid.UUID `json:"userId"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Message represents a single turn in an AI assistant conversation. Tool
// metadata is optional: it is populated only when role=='assistant' (with
// pending or final tool_calls) or role=='tool' (with tool_call_id).
type Message struct {
	ID               uuid.UUID       `json:"id"`
	SessionID        uuid.UUID       `json:"sessionId"`
	Seq              int             `json:"seq"`
	Role             string          `json:"role"`
	Content          string          `json:"content"`
	Attachments      json.RawMessage `json:"attachments,omitempty"`
	ReasoningContent string          `json:"reasoningContent,omitempty"`
	ToolCallID       string          `json:"toolCallId,omitempty"`
	ToolCalls        json.RawMessage `json:"toolCalls,omitempty"`
	Status           string          `json:"status"`
	Model            string          `json:"model,omitempty"`
	ElapsedMillis    int             `json:"elapsedMillis,omitempty"`
	UsageDetails     json.RawMessage `json:"usageDetails,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// HasPendingConfirm reports whether this message is an assistant turn
// blocked on a mutating-tool confirmation.
func (m Message) HasPendingConfirm() bool { return m.Status == StatusPendingConfirm }

// CreateSessionInput is the payload for POST /sessions.
type CreateSessionInput struct {
	Title string `json:"title"`
}

// UpdateSessionInput renames a session.
type UpdateSessionInput struct {
	Title string `json:"title"`
}

// AppendMessageInput is the internal projection used by the chat stream when
// persisting a single turn. Callers always provide Role + Content; Status
// defaults to "final" when empty.
type AppendMessageInput struct {
	Role             string
	Content          string
	Attachments      json.RawMessage
	ReasoningContent string
	ToolCallID       string
	ToolCalls        json.RawMessage
	Status           string
	Model            string
	ElapsedMillis    int
	UsageDetails     json.RawMessage
}

// RoutingLogInput is the internal projection used to persist one routing
// observation row (ai_routing_logs). It is write-only telemetry for Shadow
// comparison and offline evaluation; it does not affect the SSE schema or
// session isolation. PlannerOutput / RouterOutput / PerHop are stored as
// jsonb; MessageSeq optionally links the user message that triggered the
// route and may be zero when unknown.
type RoutingLogInput struct {
	MessageSeq    int
	PlannerOutput json.RawMessage
	RouterOutput  json.RawMessage
	PerHop        json.RawMessage
	Outcome       string
}

// ToolOutcomeInput records the outcome of a single tool call for analytics.
type ToolOutcomeInput struct {
	MessageSeq int
	ToolName   string
	Domain     string
	Intent     string
	Success    bool
	ErrorType  string
	HopIndex   int
	DurationMs int
}

// ConfirmDecision captures the user's choice for a pending mutating tool
// call. Reason is optional and surfaces in the tool result fed back to the
// model.
type ConfirmDecision struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason,omitempty"`
}

// Checkpoint represents a snapshot of session state at a specific point.
type Checkpoint struct {
	ID         uuid.UUID       `json:"id"`
	SessionID  uuid.UUID       `json:"sessionId"`
	MessageSeq int             `json:"messageSeq"`
	State      json.RawMessage `json:"state"`
	Label      string          `json:"label"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// CreateCheckpointInput is the payload for creating a checkpoint.
type CreateCheckpointInput struct {
	MessageSeq int             `json:"messageSeq"`
	State      json.RawMessage `json:"state"`
	Label      string          `json:"label"`
}

// SessionExport is the exported representation of a session.
type SessionExport struct {
	Session     Session     `json:"session"`
	Messages    []Message   `json:"messages"`
	Checkpoints []Checkpoint `json:"checkpoints"`
	ExportedAt  time.Time   `json:"exportedAt"`
}
