package aisession

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// Service wraps the repository and applies user/project isolation.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateSession opens a new conversation for (projectID, userID).
func (s *Service) CreateSession(ctx context.Context, projectID, userID uuid.UUID, input CreateSessionInput) (*Session, error) {
	if projectID == uuid.Nil || userID == uuid.Nil {
		return nil, errors.New("projectId 与 userId 不能为空")
	}
	title := strings.TrimSpace(input.Title)
	return s.repo.CreateSession(ctx, projectID, userID, title)
}

// ListSessions returns sessions owned by the user inside the project.
func (s *Service) ListSessions(ctx context.Context, projectID, userID uuid.UUID) ([]Session, error) {
	if projectID == uuid.Nil || userID == uuid.Nil {
		return nil, errors.New("projectId 与 userId 不能为空")
	}
	return s.repo.ListSessions(ctx, projectID, userID)
}

// GetTokenUsageSummary aggregates usage_details for the user's assistant
// messages. When projectID is uuid.Nil, all projects for the user are included.
func (s *Service) GetTokenUsageSummary(ctx context.Context, projectID, userID uuid.UUID) (SessionTokenUsageStats, error) {
	if userID == uuid.Nil {
		return SessionTokenUsageStats{}, errors.New("userId 不能为空")
	}
	var scope *uuid.UUID
	if projectID != uuid.Nil {
		scope = &projectID
	}
	raw, err := s.repo.ListAssistantUsageDetails(ctx, userID, scope)
	if err != nil {
		return SessionTokenUsageStats{}, err
	}
	out := SessionTokenUsageStats{}
	for _, body := range raw {
		var detail messageUsageDetail
		if err := json.Unmarshal(body, &detail); err != nil {
			continue
		}
		accumulateUsageDetail(&out, detail)
	}
	if !out.HasUsageData {
		out.Hint = tokenUsageEmptyHint
	}
	return out, nil
}

// GetSessionDetail returns session metadata with conversation and token stats.
func (s *Service) GetSessionDetail(ctx context.Context, projectID, userID, sessionID uuid.UUID) (*SessionDetail, error) {
	if projectID == uuid.Nil || userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, errors.New("projectId / userId / sessionId 不能为空")
	}
	session, err := s.repo.GetSession(ctx, projectID, userID, sessionID)
	if err != nil {
		return nil, err
	}
	messages, err := s.repo.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	detail := BuildSessionDetail(*session, messages)
	return &detail, nil
}

// GetSession returns a session + its messages enforcing ownership.
func (s *Service) GetSession(ctx context.Context, projectID, userID, sessionID uuid.UUID) (*Session, []Message, error) {
	if projectID == uuid.Nil || userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, nil, errors.New("projectId / userId / sessionId 不能为空")
	}
	session, err := s.repo.GetSession(ctx, projectID, userID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	messages, err := s.repo.ListMessages(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	return session, messages, nil
}

// SessionMeta returns session metadata without loading messages.
func (s *Service) SessionMeta(ctx context.Context, projectID, userID, sessionID uuid.UUID) (*Session, error) {
	if projectID == uuid.Nil || userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, errors.New("projectId / userId / sessionId 不能为空")
	}
	return s.repo.GetSession(ctx, projectID, userID, sessionID)
}

// RenameSession updates a session title.
func (s *Service) RenameSession(ctx context.Context, projectID, userID, sessionID uuid.UUID, input UpdateSessionInput) (*Session, error) {
	if sessionID == uuid.Nil {
		return nil, errors.New("sessionId 不能为空")
	}
	title := strings.TrimSpace(input.Title)
	return s.repo.UpdateSessionTitle(ctx, projectID, userID, sessionID, title)
}

// DeleteSession soft-deletes a session.
func (s *Service) DeleteSession(ctx context.Context, projectID, userID, sessionID uuid.UUID) error {
	if sessionID == uuid.Nil {
		return errors.New("sessionId 不能为空")
	}
	return s.repo.DeleteSession(ctx, projectID, userID, sessionID)
}

// AppendMessage writes a single message to a session. The caller is
// responsible for ensuring it has already verified session ownership.
func (s *Service) AppendMessage(ctx context.Context, sessionID uuid.UUID, input AppendMessageInput) (*Message, error) {
	if sessionID == uuid.Nil {
		return nil, errors.New("sessionId 不能为空")
	}
	msg, err := s.repo.AppendMessage(ctx, sessionID, input)
	if err != nil {
		return nil, err
	}
	if err := s.repo.TouchSession(ctx, sessionID); err != nil {
		// 触发更新失败不影响主流程，列表排序最多滞后一次刷新。
		_ = err
	}
	return msg, nil
}

// EnsureOwner returns ErrSessionForbidden if userID does not own sessionID
// inside projectID, or ErrSessionNotFound if the session does not exist.
func (s *Service) EnsureOwner(ctx context.Context, projectID, userID, sessionID uuid.UUID) error {
	if _, err := s.repo.GetSession(ctx, projectID, userID, sessionID); err != nil {
		return err
	}
	return nil
}

// FindPendingToolCall locates a pending mutating tool call within a session.
func (s *Service) FindPendingToolCall(ctx context.Context, sessionID uuid.UUID, callID string) (*Message, *PendingToolCall, error) {
	if sessionID == uuid.Nil || strings.TrimSpace(callID) == "" {
		return nil, nil, errors.New("sessionId 与 callId 不能为空")
	}
	return s.repo.FindPendingToolCall(ctx, sessionID, callID)
}

// MarkPendingFinal commits a previously-pending assistant message after
// the user has confirmed the mutating tool call. toolCalls is the same
// slice originally persisted, kept as JSON for forward compatibility.
func (s *Service) MarkPendingFinal(ctx context.Context, messageID uuid.UUID, toolCalls json.RawMessage) error {
	return s.repo.MarkMessageFinal(ctx, messageID, toolCalls)
}

// MarkPendingRejected records user rejection of a pending mutating call.
func (s *Service) MarkPendingRejected(ctx context.Context, messageID uuid.UUID) error {
	return s.repo.MarkMessageRejected(ctx, messageID)
}

// CreateCheckpoint creates a new checkpoint for a session.
func (s *Service) CreateCheckpoint(ctx context.Context, sessionID uuid.UUID, input CreateCheckpointInput) (*Checkpoint, error) {
	if sessionID == uuid.Nil {
		return nil, errors.New("sessionId 不能为空")
	}
	return s.repo.CreateCheckpoint(ctx, sessionID, input)
}

// ListCheckpoints returns all checkpoints for a session.
func (s *Service) ListCheckpoints(ctx context.Context, sessionID uuid.UUID) ([]Checkpoint, error) {
	if sessionID == uuid.Nil {
		return nil, errors.New("sessionId 不能为空")
	}
	return s.repo.ListCheckpoints(ctx, sessionID)
}

// GetCheckpoint fetches a single checkpoint.
func (s *Service) GetCheckpoint(ctx context.Context, sessionID, checkpointID uuid.UUID) (*Checkpoint, error) {
	if sessionID == uuid.Nil || checkpointID == uuid.Nil {
		return nil, errors.New("sessionId 与 checkpointId 不能为空")
	}
	return s.repo.GetCheckpoint(ctx, sessionID, checkpointID)
}

// DeleteCheckpoint removes a checkpoint.
func (s *Service) DeleteCheckpoint(ctx context.Context, sessionID, checkpointID uuid.UUID) error {
	if sessionID == uuid.Nil || checkpointID == uuid.Nil {
		return errors.New("sessionId 与 checkpointId 不能为空")
	}
	return s.repo.DeleteCheckpoint(ctx, sessionID, checkpointID)
}

// ExportSession exports a session with all messages and checkpoints.
func (s *Service) ExportSession(ctx context.Context, projectID, userID, sessionID uuid.UUID) (*SessionExport, error) {
	if projectID == uuid.Nil || userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, errors.New("projectId / userId / sessionId 不能为空")
	}
	// Verify ownership
	if err := s.EnsureOwner(ctx, projectID, userID, sessionID); err != nil {
		return nil, err
	}
	return s.repo.ExportSession(ctx, sessionID)
}

// ImportSession imports a session from an export into the given project.
func (s *Service) ImportSession(ctx context.Context, projectID, userID uuid.UUID, data *SessionExport) (*Session, error) {
	if projectID == uuid.Nil || userID == uuid.Nil {
		return nil, errors.New("projectId 与 userId 不能为空")
	}
	if data == nil {
		return nil, errors.New("导入数据为空")
	}
	return s.repo.ImportSession(ctx, projectID, userID, data)
}
