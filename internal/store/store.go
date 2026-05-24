package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"autotest/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DatabaseURL string
	MaxConns    int32
}

func ConfigFromURL(url string) Config {
	if url == "" {
		url = "postgres://autotest:autotest@localhost:5432/autotest?sslmode=disable"
	}
	return Config{
		DatabaseURL: url,
		MaxConns:    10,
	}
}

// ConfigFromEnv reads POSTGRES_* from the environment.
// Prefer passing an explicit URL from config.Config in new code.
func ConfigFromEnv() Config {
	url, err := config.DatabaseURLFromEnv()
	if err != nil {
		return ConfigFromURL("")
	}
	return ConfigFromURL(url)
}

func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, errors.New("database url is required")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
