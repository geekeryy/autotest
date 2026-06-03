package spec

import (
	"context"
	"fmt"
	"testing"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryPruneSpecs(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Skipf("open postgres: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("ping postgres: %v", err)
	}

	projectID := uuid.New()
	serviceID := uuid.New()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, `delete from projects where id = $1`, projectID); err != nil {
			t.Logf("cleanup project: %v", err)
		}
	})

	if _, err := pool.Exec(ctx, `
		insert into projects (id, name) values ($1, 'prune specs test project')
	`, projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into services (id, project_id, name) values ($1, $2, 'prune specs test service')
	`, serviceID, projectID); err != nil {
		t.Fatalf("insert service: %v", err)
	}

	for version := 1; version <= 7; version++ {
		if _, err := pool.Exec(ctx, `
			insert into api_specs (service_id, version, content_hash, raw_content, normalized_snapshot, status)
			values ($1, $2, $3, '{}'::bytea, '{}'::jsonb, 'imported')
		`, serviceID, version, fmt.Sprintf("hash-v%d", version)); err != nil {
			t.Fatalf("insert api spec v%d: %v", version, err)
		}
	}

	repo := NewRepository(store.Repository{DB: pool})
	if err := repo.PruneSpecs(ctx, serviceID, 5); err != nil {
		t.Fatalf("prune specs: %v", err)
	}

	var activeCount int
	if err := pool.QueryRow(ctx, `
		select count(*) from api_specs where service_id = $1 and deleted_at is null
	`, serviceID).Scan(&activeCount); err != nil {
		t.Fatalf("count active specs: %v", err)
	}
	if activeCount != 5 {
		t.Fatalf("active specs = %d, want 5", activeCount)
	}

	var maxVersion int
	if err := pool.QueryRow(ctx, `
		select coalesce(max(version), 0) from api_specs where service_id = $1 and deleted_at is null
	`, serviceID).Scan(&maxVersion); err != nil {
		t.Fatalf("max active version: %v", err)
	}
	if maxVersion != 7 {
		t.Fatalf("max active version = %d, want 7", maxVersion)
	}

	specs, err := repo.ListSpecs(ctx, projectID, serviceID)
	if err != nil {
		t.Fatalf("list specs: %v", err)
	}
	if len(specs) != 5 {
		t.Fatalf("listed specs = %d, want 5", len(specs))
	}
}

func TestParseSyncMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want SyncMode
	}{
		{"", SyncModeMerge},
		{"merge", SyncModeMerge},
		{"overwrite", SyncModeOverwrite},
		{"invalid", SyncModeMerge},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			if got := ParseSyncMode(tc.raw); got != tc.want {
				t.Fatalf("ParseSyncMode(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
