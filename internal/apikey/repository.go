package apikey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository 负责 api_keys 表的持久化访问，包括软删除过滤与创建者名称联表。
type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Create 写入一条 api_keys 记录；token_hash 唯一冲突等错误原样返回，
// 由 Service 转化为用户可读错误。
func (r *Repository) Create(ctx context.Context, rec record) (*record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		insert into api_keys (
			name, token_hash, token_prefix, token_suffix,
			scopes, created_by, enabled, expires_at
		)
		values ($1, $2, $3, $4, $5::text[], $6, $7, $8)
		returning id, name, token_hash, token_prefix, token_suffix,
		          scopes, created_by, enabled, expires_at, last_used_at,
		          created_at, updated_at
	`, rec.Name, rec.TokenHash, rec.TokenPrefix, rec.TokenSuffix,
		rec.Scopes, rec.CreatedBy, rec.Enabled, rec.ExpiresAt)
	out, err := scanRecord(row)
	if err != nil {
		return nil, fmt.Errorf("创建 API Key 记录失败: %w", err)
	}
	if err := r.attachCreatorName(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetByHash 按 SHA-256 hash 查询未删除且启用的 API Key，用于令牌校验路径。
// 命中后才进一步校验 expires_at；命中失败统一返回 ErrTokenInvalid 以避免泄露存在性。
func (r *Repository) GetByHash(ctx context.Context, tokenHash string) (*record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, name, token_hash, token_prefix, token_suffix,
		       scopes, created_by, enabled, expires_at, last_used_at,
		       created_at, updated_at
		from api_keys
		where token_hash = $1 and deleted_at is null
	`, tokenHash)
	out, err := scanRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenInvalid
		}
		return nil, fmt.Errorf("查询 API Key 失败: %w", err)
	}
	return out, nil
}

// Get 按 ID 查询未软删的 API Key，用于管理后台编辑/删除场景。
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, name, token_hash, token_prefix, token_suffix,
		       scopes, created_by, enabled, expires_at, last_used_at,
		       created_at, updated_at
		from api_keys
		where id = $1 and deleted_at is null
	`, id)
	out, err := scanRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("查询 API Key 失败: %w", err)
	}
	if err := r.attachCreatorName(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// List 返回未软删的全部 API Key，最近创建在前；附带创建者展示名以便前端展示。
func (r *Repository) List(ctx context.Context) ([]record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select k.id, k.name, k.token_hash, k.token_prefix, k.token_suffix,
		       k.scopes, k.created_by, k.enabled, k.expires_at, k.last_used_at,
		       k.created_at, k.updated_at,
		       coalesce(nullif(u.display_name, ''), u.username, '')
		from api_keys k
		left join users u on u.id = k.created_by
		where k.deleted_at is null
		order by k.created_at desc
	`)
	if err != nil {
		return nil, fmt.Errorf("查询 API Key 列表失败: %w", err)
	}
	defer rows.Close()

	out := make([]record, 0)
	for rows.Next() {
		var rec record
		var (
			expiresAt   *time.Time
			lastUsedAt  *time.Time
			creatorName string
		)
		if err := rows.Scan(
			&rec.ID, &rec.Name, &rec.TokenHash, &rec.TokenPrefix, &rec.TokenSuffix,
			&rec.Scopes, &rec.CreatedBy, &rec.Enabled, &expiresAt, &lastUsedAt,
			&rec.CreatedAt, &rec.UpdatedAt, &creatorName,
		); err != nil {
			return nil, fmt.Errorf("解析 API Key 行失败: %w", err)
		}
		rec.ExpiresAt = expiresAt
		rec.LastUsedAt = lastUsedAt
		rec.CreatorName = creatorName
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 API Key 列表失败: %w", err)
	}
	return out, nil
}

// Update 部分字段更新；nil 字段保留原值，CoalesceExpiresAt 与 ClearExpires 由调用方组合。
type updatePatch struct {
	Name         *string
	Enabled      *bool
	ExpiresAt    *time.Time
	ClearExpires bool
}

func (r *Repository) update(ctx context.Context, id uuid.UUID, patch updatePatch) (*record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		update api_keys
		set name       = coalesce($2, name),
		    enabled    = coalesce($3, enabled),
		    expires_at = case
		        when $5::boolean then null
		        when $4::timestamptz is not null then $4::timestamptz
		        else expires_at
		    end,
		    updated_at = now()
		where id = $1 and deleted_at is null
		returning id, name, token_hash, token_prefix, token_suffix,
		          scopes, created_by, enabled, expires_at, last_used_at,
		          created_at, updated_at
	`, id, patch.Name, patch.Enabled, patch.ExpiresAt, patch.ClearExpires)
	out, err := scanRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("更新 API Key 失败: %w", err)
	}
	if err := r.attachCreatorName(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Rotate 在一次原子 UPDATE 中替换 token_hash / token_prefix / token_suffix 并清空
// last_used_at；name / scopes / enabled / expires_at 保持不变。原 token 在 hash 被替换
// 后立即不再匹配 GetByHash，从而失效。
func (r *Repository) Rotate(ctx context.Context, id uuid.UUID, hash, prefix, suffix string) (*record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		update api_keys
		set token_hash   = $2,
		    token_prefix = $3,
		    token_suffix = $4,
		    last_used_at = null,
		    updated_at   = now()
		where id = $1 and deleted_at is null
		returning id, name, token_hash, token_prefix, token_suffix,
		          scopes, created_by, enabled, expires_at, last_used_at,
		          created_at, updated_at
	`, id, hash, prefix, suffix)
	out, err := scanRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("重置 API Key 失败: %w", err)
	}
	if err := r.attachCreatorName(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// SoftDelete 标记 deleted_at 为 now()；token_hash 唯一索引带有 where deleted_at is null，
// 故同 hash 复用不会冲突（虽然实际上 token 是随机生成，碰撞概率可忽略）。
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update api_keys
		set deleted_at = now(), enabled = false, updated_at = now()
		where id = $1 and deleted_at is null
	`, id)
	if err != nil {
		return fmt.Errorf("软删除 API Key 失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchLastUsed 异步刷新 last_used_at，仅在校验成功后调用；失败仅记录，不影响请求。
func (r *Repository) TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	if _, err := r.DB.Exec(ctx, `
		update api_keys
		set last_used_at = $2
		where id = $1 and deleted_at is null
	`, id, at); err != nil {
		return fmt.Errorf("更新 API Key 最近使用时间失败: %w", err)
	}
	return nil
}

func (r *Repository) attachCreatorName(ctx context.Context, rec *record) error {
	if rec == nil || rec.CreatedBy == uuid.Nil {
		return nil
	}
	var name string
	err := r.DB.QueryRow(ctx, `
		select coalesce(nullif(display_name, ''), username, '')
		from users where id = $1
	`, rec.CreatedBy).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("查询 API Key 创建者失败: %w", err)
	}
	rec.CreatorName = name
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row rowScanner) (*record, error) {
	var rec record
	var (
		expiresAt  *time.Time
		lastUsedAt *time.Time
	)
	if err := row.Scan(
		&rec.ID, &rec.Name, &rec.TokenHash, &rec.TokenPrefix, &rec.TokenSuffix,
		&rec.Scopes, &rec.CreatedBy, &rec.Enabled, &expiresAt, &lastUsedAt,
		&rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return nil, err
	}
	rec.ExpiresAt = expiresAt
	rec.LastUsedAt = lastUsedAt
	return &rec, nil
}
