package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

const defaultListLimit = 20

type repository interface {
	Create(ctx context.Context, input createInput) (*Notification, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]Notification, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	DeleteAllByUser(ctx context.Context, userID uuid.UUID) error
	LookupProjectServiceNames(ctx context.Context, projectID, serviceID uuid.UUID) (projectName, serviceName string, err error)
}

// Service 封装通知业务逻辑。
type Service struct {
	repo repository
	hub  *Hub
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo, hub: NewHub()}
}

// List 返回当前用户的通知列表与未读计数。
func (s *Service) List(ctx context.Context, userID uuid.UUID, limit int) (*ListResponse, error) {
	if limit <= 0 {
		limit = defaultListLimit
	}
	items, err := s.repo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	unread, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Notification{}
	}
	return &ListResponse{Items: items, UnreadCount: unread}, nil
}

// MarkRead 将单条通知标为已读。
func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.MarkRead(ctx, userID, id)
}

// MarkAllRead 将当前用户全部未读通知标为已读。
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}

// ClearAll 删除当前用户的全部通知。
func (s *Service) ClearAll(ctx context.Context, userID uuid.UUID) error {
	if err := s.repo.DeleteAllByUser(ctx, userID); err != nil {
		return err
	}
	s.publishSnapshot(userID, []Notification{}, 0)
	return nil
}

func (s *Service) publishSnapshot(userID uuid.UUID, items []Notification, unread int) {
	s.hub.Publish(userID, StreamEvent{
		Kind:        "snapshot",
		UnreadCount: unread,
		Items:       items,
	})
}

// CreateSpecImportNotification 在 API Key 成功导入 Swagger 后为 Key 所属用户写入通知。
func (s *Service) CreateSpecImportNotification(
	ctx context.Context,
	userID, projectID, serviceID uuid.UUID,
	summary *spec.ImportSummary,
) error {
	if summary == nil {
		return fmt.Errorf("import summary is required")
	}

	projectName, serviceName, err := s.repo.LookupProjectServiceNames(ctx, projectID, serviceID)
	if err != nil {
		return err
	}
	if projectName == "" {
		projectName = projectID.String()
	}
	if serviceName == "" {
		serviceName = serviceID.String()
	}

	changed := summary.CreatedEndpoints + summary.UpdatedEndpoints
	body := fmt.Sprintf(
		"项目「%s」服务「%s」已通过 API Key 导入：本次变动 %d 个接口（新增 %d、更新 %d），生成模板 %d 条。",
		projectName, serviceName,
		changed, summary.CreatedEndpoints, summary.UpdatedEndpoints, summary.GeneratedCases,
	)

	payload, err := json.Marshal(SpecImportPayload{
		ProjectID:        projectID,
		ServiceID:        serviceID,
		SpecID:           summary.SpecID,
		ApiCount:         summary.ApiCount,
		ChangedEndpoints: changed,
		CreatedEndpoints: summary.CreatedEndpoints,
		UpdatedEndpoints: summary.UpdatedEndpoints,
		GeneratedCases:   summary.GeneratedCases,
	})
	if err != nil {
		return fmt.Errorf("marshal spec import payload: %w", err)
	}

	return s.createAndPublish(ctx, createInput{
		UserID:  userID,
		Type:    TypeSpecImport,
		Title:   specImportTitle,
		Body:    body,
		Payload: payload,
	})
}

func (s *Service) createAndPublish(ctx context.Context, input createInput) error {
	n, err := s.repo.Create(ctx, input)
	if err != nil {
		return err
	}
	unread, err := s.repo.CountUnread(ctx, input.UserID)
	if err != nil {
		return err
	}
	s.hub.Publish(input.UserID, StreamEvent{
		Kind:         "notification",
		UnreadCount:  unread,
		Notification: n,
	})
	return nil
}

// Subscribe 为 SSE 流注册当前用户的实时推送。
func (s *Service) Subscribe(userID uuid.UUID) (<-chan StreamEvent, func()) {
	return s.hub.Subscribe(userID)
}

// StreamSnapshot 返回连接建立时的列表与未读计数。
func (s *Service) StreamSnapshot(ctx context.Context, userID uuid.UUID, limit int) (*ListResponse, error) {
	return s.List(ctx, userID, limit)
}
