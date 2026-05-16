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
	ReasoningContent string          `json:"-"`
	ToolCallID       string          `json:"toolCallId,omitempty"`
	ToolCalls        json.RawMessage `json:"toolCalls,omitempty"`
	Status           string          `json:"status"`
	Model            string          `json:"model,omitempty"`
	ElapsedMillis    int             `json:"elapsedMillis,omitempty"`
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
}

// ConfirmDecision captures the user's choice for a pending mutating tool
// call. Reason is optional and surfaces in the tool result fed back to the
// model.
type ConfirmDecision struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason,omitempty"`
}
