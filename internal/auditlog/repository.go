package auditlog

import (
	"context"
	"encoding/json"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
)

// Repository 负责 audit_logs 表的持久化访问。
type Repository struct {
	store.Repository
}

// NewRepository wraps the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

type createInput struct {
	Action        string
	Success       bool
	ActorUserID   *uuid.UUID
	ActorUsername string
	Resource      string
	ClientIP      string
	UserAgent     string
	Detail        json.RawMessage
}

// Create 写入一条审计日志。
func (r *Repository) Create(ctx context.Context, input createInput) (*Entry, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	detail := input.Detail
	if detail == nil {
		detail = json.RawMessage(`{}`)
	}
	row := r.DB.QueryRow(ctx, `
		insert into audit_logs (
			action, success, actor_user_id, actor_username,
			resource, client_ip, user_agent, detail
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		returning id, action, success, actor_user_id, actor_username,
		          resource, client_ip, user_agent, detail, created_at
	`, input.Action, input.Success, input.ActorUserID, input.ActorUsername,
		input.Resource, input.ClientIP, input.UserAgent, detail)
	return scanEntry(row)
}

// List 按条件分页查询审计日志，按创建时间倒序。
func (r *Repository) List(ctx context.Context, filter ListFilter) ([]Entry, int, error) {
	if r.DB == nil {
		return nil, 0, fmt.Errorf("database unavailable")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	where := "where 1=1"
	args := []any{}
	argN := 1

	if filter.Action != "" {
		where += fmt.Sprintf(" and action = $%d", argN)
		args = append(args, filter.Action)
		argN++
	}
	if filter.Success != nil {
		where += fmt.Sprintf(" and success = $%d", argN)
		args = append(args, *filter.Success)
		argN++
	}
	if filter.ActorUsername != "" {
		where += fmt.Sprintf(" and actor_username ilike $%d", argN)
		args = append(args, "%"+filter.ActorUsername+"%")
		argN++
	}

	var total int
	countQ := "select count(*) from audit_logs " + where
	if err := r.DB.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	listQ := `
		select id, action, success, actor_user_id, actor_username,
		       resource, client_ip, user_agent, detail, created_at
		from audit_logs
	` + where + fmt.Sprintf(" order by created_at desc limit $%d offset $%d", argN, argN+1)
	args = append(args, limit, offset)

	rows, err := r.DB.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	items := []Entry{}
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *entry)
	}
	return items, total, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEntry(row scanner) (*Entry, error) {
	var entry Entry
	var actorUserID *uuid.UUID
	if err := row.Scan(
		&entry.ID,
		&entry.Action,
		&entry.Success,
		&actorUserID,
		&entry.ActorUsername,
		&entry.Resource,
		&entry.ClientIP,
		&entry.UserAgent,
		&entry.Detail,
		&entry.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan audit log: %w", err)
	}
	entry.ActorUserID = actorUserID
	if entry.Detail == nil {
		entry.Detail = json.RawMessage(`{}`)
	}
	return &entry, nil
}
