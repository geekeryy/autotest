package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("notification not found")

// Repository 负责 notifications 表的持久化访问。
type Repository struct {
	store.Repository
}

// NewRepository wraps the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

type createInput struct {
	UserID  uuid.UUID
	Type    string
	Title   string
	Body    string
	Payload json.RawMessage
}

// Create 写入一条通知。
func (r *Repository) Create(ctx context.Context, input createInput) (*Notification, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		insert into notifications (user_id, type, title, body, payload)
		values ($1, $2, $3, $4, $5)
		returning id, user_id, type, title, body, payload, read_at, created_at
	`, input.UserID, input.Type, input.Title, input.Body, input.Payload)
	return scanNotification(row)
}

// ListByUser 返回指定用户最近的通知，按创建时间倒序。
func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]Notification, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.DB.Query(ctx, `
		select id, user_id, type, title, body, payload, read_at, created_at
		from notifications
		where user_id = $1
		order by created_at desc
		limit $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	out := []Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

// CountUnread 统计用户未读通知数量。
func (r *Repository) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("database unavailable")
	}
	var count int
	if err := r.DB.QueryRow(ctx, `
		select count(*)
		from notifications
		where user_id = $1 and read_at is null
	`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// MarkRead 将单条通知标为已读；仅当通知属于该用户时生效。
func (r *Repository) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update notifications
		set read_at = now()
		where id = $1 and user_id = $2 and read_at is null
	`, id, userID)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := r.DB.QueryRow(ctx, `
			select exists(
				select 1 from notifications where id = $1 and user_id = $2
			)
		`, id, userID).Scan(&exists); err != nil {
			return fmt.Errorf("check notification ownership: %w", err)
		}
		if !exists {
			return ErrNotFound
		}
	}
	return nil
}

// MarkAllRead 将用户全部未读通知标为已读。
func (r *Repository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	if _, err := r.DB.Exec(ctx, `
		update notifications
		set read_at = now()
		where user_id = $1 and read_at is null
	`, userID); err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// DeleteAllByUser 删除指定用户的全部通知。
func (r *Repository) DeleteAllByUser(ctx context.Context, userID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	if _, err := r.DB.Exec(ctx, `
		delete from notifications
		where user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("delete all notifications: %w", err)
	}
	return nil
}

// LookupProjectServiceNames 解析项目与服务展示名，用于通知正文。
func (r *Repository) LookupProjectServiceNames(ctx context.Context, projectID, serviceID uuid.UUID) (projectName, serviceName string, err error) {
	if r.DB == nil {
		return "", "", fmt.Errorf("database unavailable")
	}
	err = r.DB.QueryRow(ctx, `
		select coalesce(p.name, ''), coalesce(s.name, '')
		from projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		where p.id = $1 and p.deleted_at is null
	`, projectID, serviceID).Scan(&projectName, &serviceName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("lookup project service names: %w", err)
	}
	return projectName, serviceName, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner) (*Notification, error) {
	var n Notification
	var readAt *time.Time
	if err := row.Scan(
		&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.Payload, &readAt, &n.CreatedAt,
	); err != nil {
		return nil, err
	}
	n.ReadAt = readAt
	return &n, nil
}
