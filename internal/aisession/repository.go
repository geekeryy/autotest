package aisession

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository persists AI assistant sessions and their messages.
type Repository struct {
	store.Repository
}

// NewRepository wraps the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// CreateSession inserts a new session for (projectID, userID).
func (r *Repository) CreateSession(ctx context.Context, projectID, userID uuid.UUID, title string) (*Session, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		insert into ai_sessions (project_id, user_id, title)
		values ($1, $2, $3)
		returning id, project_id, user_id, title, created_at, updated_at
	`, projectID, userID, title)
	return scanSession(row)
}

// ListSessions returns sessions owned by user within project, newest-updated first.
func (r *Repository) ListSessions(ctx context.Context, projectID, userID uuid.UUID) ([]Session, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select id, project_id, user_id, title, created_at, updated_at
		from ai_sessions
		where project_id = $1 and user_id = $2 and deleted_at is null
		order by updated_at desc
	`, projectID, userID)
	if err != nil {
		return nil, fmt.Errorf("list ai sessions: %w", err)
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// GetSession fetches a single session enforcing user ownership.
func (r *Repository) GetSession(ctx context.Context, projectID, userID, sessionID uuid.UUID) (*Session, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, project_id, user_id, title, created_at, updated_at
		from ai_sessions
		where id = $3 and project_id = $1 and user_id = $2 and deleted_at is null
	`, projectID, userID, sessionID)
	s, err := scanSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return s, nil
}

// UpdateSessionTitle changes a session title.
func (r *Repository) UpdateSessionTitle(ctx context.Context, projectID, userID, sessionID uuid.UUID, title string) (*Session, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		update ai_sessions
		set title = $4, updated_at = now()
		where id = $3 and project_id = $1 and user_id = $2 and deleted_at is null
		returning id, project_id, user_id, title, created_at, updated_at
	`, projectID, userID, sessionID, title)
	s, err := scanSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return s, nil
}

// DeleteSession soft-deletes a session and cascades via the FK on
// ai_messages.session_id (the table-level on delete cascade only applies
// when we hard-delete; soft delete keeps messages around for audit, but
// they become unreachable by the standard JOINs).
func (r *Repository) DeleteSession(ctx context.Context, projectID, userID, sessionID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update ai_sessions
		set deleted_at = now(), updated_at = now()
		where id = $3 and project_id = $1 and user_id = $2 and deleted_at is null
	`, projectID, userID, sessionID)
	if err != nil {
		return fmt.Errorf("delete ai session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// TouchSession updates the updated_at timestamp so list ordering reflects
// the most recently active conversation.
func (r *Repository) TouchSession(ctx context.Context, sessionID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	_, err := r.DB.Exec(ctx, `
		update ai_sessions
		set updated_at = now()
		where id = $1 and deleted_at is null
	`, sessionID)
	return err
}

// ListAssistantUsageDetails returns usage_details JSON for assistant messages
// owned by userID, optionally scoped to projectID when projectID is not nil.
func (r *Repository) ListAssistantUsageDetails(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) ([][]byte, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	query := `
		select m.usage_details
		from ai_messages m
		inner join ai_sessions s on s.id = m.session_id
		where s.user_id = $1
		  and s.deleted_at is null
		  and m.role = 'assistant'
		  and m.usage_details is not null
	`
	args := []any{userID}
	if projectID != nil && *projectID != uuid.Nil {
		query += ` and s.project_id = $2`
		args = append(args, *projectID)
	}
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list assistant usage details: %w", err)
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			out = append(out, raw)
		}
	}
	return out, rows.Err()
}

// ListMessages returns every message of a session in seq order.
func (r *Repository) ListMessages(ctx context.Context, sessionID uuid.UUID) ([]Message, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select id, session_id, seq, role, content, attachments, reasoning_content, tool_call_id, tool_calls,
		       status, model, elapsed_millis, usage_details, created_at
		from ai_messages
		where session_id = $1
		order by seq asc
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list ai messages: %w", err)
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// AppendMessage atomically appends a message to the session and returns the
// persisted record. The seq is computed inside the same statement as
// `coalesce(max(seq), 0) + 1` over the session, which races safely under
// PostgreSQL's row locking semantics for our usage (one writer per session).
func (r *Repository) AppendMessage(ctx context.Context, sessionID uuid.UUID, input AppendMessageInput) (*Message, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	status := input.Status
	if status == "" {
		status = StatusFinal
	}
	var toolCallID any
	if input.ToolCallID != "" {
		toolCallID = input.ToolCallID
	}
	var toolCalls any
	if len(input.ToolCalls) > 0 {
		toolCalls = input.ToolCalls
	}
	var model any
	if input.Model != "" {
		model = input.Model
	}
	var elapsed any
	if input.ElapsedMillis > 0 {
		elapsed = input.ElapsedMillis
	}
	var attachments any
	if len(input.Attachments) > 0 {
		attachments = input.Attachments
	}
	var usageDetails any
	if len(input.UsageDetails) > 0 {
		usageDetails = input.UsageDetails
	}
	row := r.DB.QueryRow(ctx, `
		insert into ai_messages (
			session_id, seq, role, content, attachments, reasoning_content, tool_call_id, tool_calls,
			status, model, elapsed_millis, usage_details
		)
		values (
			$1,
			(select coalesce(max(seq), 0) + 1 from ai_messages where session_id = $1),
			$2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		returning id, session_id, seq, role, content, attachments, reasoning_content, tool_call_id, tool_calls,
		          status, model, elapsed_millis, usage_details, created_at
	`, sessionID, input.Role, input.Content, attachments, input.ReasoningContent, toolCallID, toolCalls, status, model, elapsed, usageDetails)
	return scanMessage(row)
}

// AppendRoutingLog inserts one routing observation row tied to a session.
// This is write-only telemetry (Shadow compare / offline eval) and never
// participates in the live conversation flow. Optional jsonb / int columns
// are passed as nil when empty so PostgreSQL stores SQL NULL rather than an
// empty payload.
func (r *Repository) AppendRoutingLog(ctx context.Context, sessionID uuid.UUID, input RoutingLogInput) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	var messageSeq any
	if input.MessageSeq > 0 {
		messageSeq = input.MessageSeq
	}
	var plannerOutput any
	if len(input.PlannerOutput) > 0 {
		plannerOutput = input.PlannerOutput
	}
	var routerOutput any
	if len(input.RouterOutput) > 0 {
		routerOutput = input.RouterOutput
	}
	var perHop any
	if len(input.PerHop) > 0 {
		perHop = input.PerHop
	}
	var outcome any
	if input.Outcome != "" {
		outcome = input.Outcome
	}
	_, err := r.DB.Exec(ctx, `
		insert into ai_routing_logs (
			session_id, message_seq, planner_output, router_output, per_hop, outcome
		)
		values ($1, $2, $3, $4, $5, $6)
	`, sessionID, messageSeq, plannerOutput, routerOutput, perHop, outcome)
	if err != nil {
		return fmt.Errorf("append ai routing log: %w", err)
	}
	return nil
}

// MarkMessageFinal flips a pending_confirm message to final and updates
// its tool_calls payload (the latter is rewritten so the persisted history
// can drop the "pending" tool call entry once the user has decided).
func (r *Repository) MarkMessageFinal(ctx context.Context, messageID uuid.UUID, toolCalls json.RawMessage) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	var payload any
	if len(toolCalls) > 0 {
		payload = toolCalls
	}
	tag, err := r.DB.Exec(ctx, `
		update ai_messages
		set status = $2, tool_calls = $3
		where id = $1 and status = 'pending_confirm'
	`, messageID, StatusFinal, payload)
	if err != nil {
		return fmt.Errorf("mark ai message final: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPendingDecided
	}
	return nil
}

// MarkMessageRejected records the user rejecting a mutating tool call.
func (r *Repository) MarkMessageRejected(ctx context.Context, messageID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update ai_messages
		set status = $2
		where id = $1 and status = 'pending_confirm'
	`, messageID, StatusRejected)
	if err != nil {
		return fmt.Errorf("mark ai message rejected: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPendingDecided
	}
	return nil
}

// FindPendingToolCall locates a session-scoped pending message that holds
// the given tool_call id inside its tool_calls payload. Returns the message
// plus the targeted call so the caller can run / record it on confirmation.
//
// The function intentionally returns ErrPendingNotFound for both
// "no pending row" and "row exists but does not contain the requested
// call id"; the surface area for guessing call ids should be tiny.
func (r *Repository) FindPendingToolCall(ctx context.Context, sessionID uuid.UUID, callID string) (*Message, *PendingToolCall, error) {
	if r.DB == nil {
		return nil, nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select id, session_id, seq, role, content, attachments, reasoning_content, tool_call_id, tool_calls,
		       status, model, elapsed_millis, usage_details, created_at
		from ai_messages
		where session_id = $1 and status = 'pending_confirm'
		order by seq desc
	`, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("find pending tool call: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, nil, err
		}
		if len(m.ToolCalls) == 0 {
			continue
		}
		var calls []PendingToolCall
		if err := json.Unmarshal(m.ToolCalls, &calls); err != nil {
			continue
		}
		for i := range calls {
			if calls[i].ID == callID {
				return m, &calls[i], nil
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return nil, nil, ErrPendingNotFound
}

// PendingToolCall is the JSON shape we persist inside ai_messages.tool_calls
// for assistant turns that requested mutating tools. Mirrors
// client.ToolCall but adds the Mutating flag for completeness.
type PendingToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Mutating  bool            `json:"mutating,omitempty"`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(row rowScanner) (*Session, error) {
	var s Session
	if err := row.Scan(&s.ID, &s.ProjectID, &s.UserID, &s.Title, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func scanMessage(row rowScanner) (*Message, error) {
	var m Message
	var toolCallID pgtype.Text
	var toolCalls []byte
	var model pgtype.Text
	var elapsed pgtype.Int4
	var attachments []byte
	var usageDetails []byte
	if err := row.Scan(
		&m.ID, &m.SessionID, &m.Seq, &m.Role, &m.Content, &attachments,
		&m.ReasoningContent, &toolCallID, &toolCalls, &m.Status,
		&model, &elapsed, &usageDetails, &m.CreatedAt,
	); err != nil {
		return nil, err
	}
	if toolCallID.Valid {
		m.ToolCallID = toolCallID.String
	}
	if len(toolCalls) > 0 {
		m.ToolCalls = json.RawMessage(toolCalls)
	}
	if len(attachments) > 0 {
		m.Attachments = json.RawMessage(attachments)
	}
	if model.Valid {
		m.Model = model.String
	}
	if elapsed.Valid {
		m.ElapsedMillis = int(elapsed.Int32)
	}
	if len(usageDetails) > 0 {
		m.UsageDetails = json.RawMessage(usageDetails)
	}
	return &m, nil
}
