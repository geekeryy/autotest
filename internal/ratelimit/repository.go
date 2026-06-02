package ratelimit

import (
	"context"
	"fmt"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
)

// Repository persists rate limit data.
type Repository struct {
	store.Repository
}

// NewRepository wraps the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// RecordRequest atomically inserts a new request entry and returns the count
// of non-expired entries for the given key (including the newly inserted one).
// Uses a CTE so the INSERT and COUNT happen in a single atomic statement.
func (r *Repository) RecordRequest(ctx context.Context, key string, windowSeconds int) (int, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("database unavailable")
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(windowSeconds) * time.Second)

	var count int
	err := r.DB.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO ai_rate_limit_entries (key, created_at, expires_at)
			VALUES ($1, $2, $3)
		)
		SELECT COUNT(*) FROM ai_rate_limit_entries
		WHERE key = $1 AND expires_at > $2
	`, key, now, expiresAt).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("record rate limit request: %w", err)
	}

	return count, nil
}

// GetEarliestExpiry returns the earliest expiry time of non-expired entries
// for the given key. Used to calculate accurate resetAt/retryAfter.
func (r *Repository) GetEarliestExpiry(ctx context.Context, key string) (*time.Time, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	var expiresAt time.Time
	err := r.DB.QueryRow(ctx, `
		SELECT MIN(expires_at) FROM ai_rate_limit_entries
		WHERE key = $1 AND expires_at > $2
	`, key, time.Now()).Scan(&expiresAt)
	if err != nil {
		return nil, err
	}
	return &expiresAt, nil
}

// CountCurrent returns the number of non-expired entries for the key.
func (r *Repository) CountCurrent(ctx context.Context, key string) (int, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("database unavailable")
	}

	var count int
	err := r.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM ai_rate_limit_entries
		WHERE key = $1 AND expires_at > $2
	`, key, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count rate limit entries: %w", err)
	}
	return count, nil
}

// Cleanup removes expired entries. Returns the number of rows deleted.
func (r *Repository) Cleanup(ctx context.Context) (int64, error) {
	if r.DB == nil {
		return 0, fmt.Errorf("database unavailable")
	}

	tag, err := r.DB.Exec(ctx, `
		DELETE FROM ai_rate_limit_entries
		WHERE expires_at < $1
	`, time.Now())
	if err != nil {
		return 0, fmt.Errorf("cleanup rate limit entries: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetRule returns a rate limit rule by name.
func (r *Repository) GetRule(ctx context.Context, name string) (*RateLimitRule, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	var rule RateLimitRule
	err := r.DB.QueryRow(ctx, `
		SELECT id, name, key, "limit", "window", enabled, created_at, updated_at
		FROM ai_rate_limit_rules
		WHERE name = $1 AND enabled = true
	`, name).Scan(&rule.ID, &rule.Name, &rule.Key, &rule.Limit, &rule.Window, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get rate limit rule: %w", err)
	}
	return &rule, nil
}

// ListRules returns all enabled rules.
func (r *Repository) ListRules(ctx context.Context) ([]RateLimitRule, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	rows, err := r.DB.Query(ctx, `
		SELECT id, name, key, "limit", "window", enabled, created_at, updated_at
		FROM ai_rate_limit_rules
		WHERE enabled = true
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list rate limit rules: %w", err)
	}
	defer rows.Close()

	var rules []RateLimitRule
	for rows.Next() {
		var rule RateLimitRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Key, &rule.Limit, &rule.Window, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// CreateRule inserts a new rate limit rule.
func (r *Repository) CreateRule(ctx context.Context, rule RateLimitRule) (*RateLimitRule, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	now := time.Now()
	err := r.DB.QueryRow(ctx, `
		INSERT INTO ai_rate_limit_rules (id, name, key, "limit", "window", enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, key, "limit", "window", enabled, created_at, updated_at
	`, uuid.New(), rule.Name, rule.Key, rule.Limit, rule.Window, true, now, now,
	).Scan(&rule.ID, &rule.Name, &rule.Key, &rule.Limit, &rule.Window, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create rate limit rule: %w", err)
	}
	return &rule, nil
}
