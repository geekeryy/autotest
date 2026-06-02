package aisession

// adapter.go bridges aisession.Service to aiprovider.SessionStore so the
// streaming chat engine can persist conversation state without importing
// this package directly. We keep the adapter here (rather than in
// aiprovider) because:
//
//   - aiprovider already defines the SessionStore interface and would
//     otherwise pull aisession into its package graph, causing a cycle
//     between AI orchestration and persistence layers.
//   - The mapping between aisession.Message and aiprovider.StoredMessage
//     is mechanical; centralising it here keeps the conversion in one place.

import (
	"context"
	"encoding/json"

	"autotest/internal/aiprovider"

	"github.com/google/uuid"
)

// StoreAdapter satisfies aiprovider.SessionStore on top of Service.
type StoreAdapter struct {
	svc *Service
}

// NewStoreAdapter wraps Service so it can be plugged into
// aiprovider.Handler.WithAssistant().
func NewStoreAdapter(svc *Service) *StoreAdapter {
	return &StoreAdapter{svc: svc}
}

// compile-time interface assertion — flips a hard build error if the
// adapter and the consumer-side contract ever drift.
var _ aiprovider.SessionStore = (*StoreAdapter)(nil)

// ListMessages returns the persisted history of a session in seq order.
func (a *StoreAdapter) ListMessages(ctx context.Context, sessionID uuid.UUID) ([]aiprovider.StoredMessage, error) {
	msgs, err := a.svc.repo.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]aiprovider.StoredMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toStored(m))
	}
	return out, nil
}

// AppendMessage persists a new message and returns the saved record.
func (a *StoreAdapter) AppendMessage(ctx context.Context, sessionID uuid.UUID, input aiprovider.StoredMessageInput) (*aiprovider.StoredMessage, error) {
	msg, err := a.svc.AppendMessage(ctx, sessionID, AppendMessageInput{
		Role:             input.Role,
		Content:          input.Content,
		Attachments:      input.Attachments,
		ReasoningContent: input.ReasoningContent,
		ToolCallID:       input.ToolCallID,
		ToolCalls:        input.ToolCalls,
		Status:           input.Status,
		Model:            input.Model,
		ElapsedMillis:    input.ElapsedMillis,
		UsageDetails:     input.UsageDetails,
	})
	if err != nil {
		return nil, err
	}
	stored := toStored(*msg)
	return &stored, nil
}

// FindPendingToolCall locates a pending mutating call inside a session.
func (a *StoreAdapter) FindPendingToolCall(ctx context.Context, sessionID uuid.UUID, callID string) (*aiprovider.StoredMessage, *aiprovider.StoredToolCall, error) {
	msg, call, err := a.svc.FindPendingToolCall(ctx, sessionID, callID)
	if err != nil {
		return nil, nil, err
	}
	stored := toStored(*msg)
	return &stored, &aiprovider.StoredToolCall{
		ID:        call.ID,
		Name:      call.Name,
		Arguments: call.Arguments,
		Mutating:  call.Mutating,
	}, nil
}

// MarkPendingFinal moves a pending assistant message to final after the
// user confirmed the underlying mutating tool call.
func (a *StoreAdapter) MarkPendingFinal(ctx context.Context, messageID uuid.UUID, toolCalls json.RawMessage) error {
	return a.svc.MarkPendingFinal(ctx, messageID, toolCalls)
}

// MarkPendingRejected records user rejection of a pending call.
func (a *StoreAdapter) MarkPendingRejected(ctx context.Context, messageID uuid.UUID) error {
	return a.svc.MarkPendingRejected(ctx, messageID)
}

// GetSession returns session metadata for the owner.
func (a *StoreAdapter) GetSession(ctx context.Context, projectID, userID, sessionID uuid.UUID) (*aiprovider.StoredSession, error) {
	s, err := a.svc.SessionMeta(ctx, projectID, userID, sessionID)
	if err != nil {
		return nil, err
	}
	return &aiprovider.StoredSession{ID: s.ID, Title: s.Title}, nil
}

// UpdateSessionTitle renames a session.
func (a *StoreAdapter) UpdateSessionTitle(ctx context.Context, projectID, userID, sessionID uuid.UUID, title string) (*aiprovider.StoredSession, error) {
	s, err := a.svc.RenameSession(ctx, projectID, userID, sessionID, UpdateSessionInput{Title: title})
	if err != nil {
		return nil, err
	}
	return &aiprovider.StoredSession{ID: s.ID, Title: s.Title}, nil
}

// AppendRoutingLog persists one routing observation row, mapping the
// aiprovider-level input onto the aisession repository. Telemetry only —
// failures here must not be allowed to break the live conversation flow,
// but the adapter simply surfaces the error and lets the caller decide.
func (a *StoreAdapter) AppendRoutingLog(ctx context.Context, input aiprovider.RoutingLogInput) error {
	return a.svc.repo.AppendRoutingLog(ctx, input.SessionID, RoutingLogInput{
		MessageSeq:    input.MessageSeq,
		PlannerOutput: input.PlannerOutput,
		RouterOutput:  input.RouterOutput,
		PerHop:        input.PerHop,
		Outcome:       input.Outcome,
	})
}

// AppendToolOutcome persists one tool call outcome row for analytics.
func (a *StoreAdapter) AppendToolOutcome(ctx context.Context, input aiprovider.ToolOutcomeInput) error {
	return a.svc.repo.AppendToolOutcome(ctx, input.SessionID, ToolOutcomeInput{
		MessageSeq: input.MessageSeq,
		ToolName:   input.ToolName,
		Domain:     input.Domain,
		Intent:     input.Intent,
		Success:    input.Success,
		ErrorType:  input.ErrorType,
		HopIndex:   input.HopIndex,
		DurationMs: input.DurationMs,
	})
}

func toStored(m Message) aiprovider.StoredMessage {
	return aiprovider.StoredMessage{
		ID:               m.ID,
		SessionID:        m.SessionID,
		Seq:              m.Seq,
		Role:             m.Role,
		Content:          m.Content,
		Attachments:      m.Attachments,
		ReasoningContent: m.ReasoningContent,
		ToolCallID:       m.ToolCallID,
		ToolCalls:        m.ToolCalls,
		Status:           m.Status,
		Model:            m.Model,
		UsageDetails:     m.UsageDetails,
		CreatedAt:        m.CreatedAt,
	}
}
