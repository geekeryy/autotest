package aiconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"autotest/internal/store"

	"github.com/jackc/pgx/v5"
)

// Repository persists ai_assistant_settings.
type Repository struct {
	store.Repository
}

// NewRepository creates a Repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// Get loads stored config. Returns nil raw when row is empty/default only.
func (r *Repository) Get(ctx context.Context) (*storedConfig, time.Time, error) {
	if r.DB == nil {
		return nil, time.Time{}, fmt.Errorf("database unavailable")
	}
	var (
		raw       []byte
		updatedAt time.Time
	)
	err := r.DB.QueryRow(ctx, `
		select config, updated_at
		from ai_assistant_settings
		where id = 1
	`).Scan(&raw, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, fmt.Errorf("get ai assistant settings: %w", err)
	}
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return nil, updatedAt, nil
	}
	var out storedConfig
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, time.Time{}, fmt.Errorf("decode ai assistant settings: %w", err)
	}
	return &out, updatedAt, nil
}

// Save upserts the singleton settings row.
func (r *Repository) Save(ctx context.Context, cfg storedConfig) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode ai assistant settings: %w", err)
	}
	_, err = r.DB.Exec(ctx, `
		insert into ai_assistant_settings (id, config, updated_at)
		values (1, $1::jsonb, now())
		on conflict (id) do update
		set config = excluded.config, updated_at = now()
	`, body)
	if err != nil {
		return fmt.Errorf("save ai assistant settings: %w", err)
	}
	return nil
}
