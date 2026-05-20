package spec

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositorySyncEndpointsUnchangedDoesNotError(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for repository integration test")
	}

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
	specID := uuid.New()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, `delete from projects where id = $1`, projectID); err != nil {
			t.Logf("cleanup project: %v", err)
		}
	})

	if _, err := pool.Exec(ctx, `
		insert into projects (id, name) values ($1, 'sync endpoints test project')
	`, projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into services (id, project_id, name) values ($1, $2, 'sync endpoints test service')
	`, serviceID, projectID); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into api_specs (id, service_id, version, content_hash, raw_content, normalized_snapshot, status)
		values ($1, $2, 1, 'hash-v1', '{}'::bytea, '{}'::jsonb, 'imported')
	`, specID, serviceID); err != nil {
		t.Fatalf("insert api spec: %v", err)
	}

	endpoint := Endpoint{
		Method:         "GET",
		Path:           "/api/admin/v1/audit_log/detail",
		OperationID:    "getAuditLogDetail",
		Summary:        "Audit log detail",
		Tags:           []string{"audit"},
		RequestSchema:  json.RawMessage(`{}`),
		ResponseSchema: json.RawMessage(`{}`),
		Fingerprint:    "fp-unchanged",
	}

	repo := NewRepository(store.Repository{DB: pool})

	synced, created, updated, err := repo.SyncEndpoints(ctx, projectID, serviceID, specID, []Endpoint{endpoint})
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("first sync counts: created=%d updated=%d, want 1/0", created, updated)
	}
	if len(synced) != 1 {
		t.Fatalf("first sync returned %d endpoints, want 1", len(synced))
	}

	synced, created, updated, err = repo.SyncEndpoints(ctx, projectID, serviceID, specID, []Endpoint{endpoint})
	if err != nil {
		t.Fatalf("second sync unchanged endpoint: %v", err)
	}
	if created != 0 || updated != 0 {
		t.Fatalf("second sync counts: created=%d updated=%d, want 0/0", created, updated)
	}
	if len(synced) != 1 {
		t.Fatalf("second sync returned %d endpoints, want 1", len(synced))
	}
	if synced[0].ID == uuid.Nil {
		t.Fatalf("expected existing endpoint id")
	}
}

func TestRepositorySyncEndpointsCountsFingerprintChange(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for repository integration test")
	}

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
	specID := uuid.New()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, `delete from projects where id = $1`, projectID); err != nil {
			t.Logf("cleanup project: %v", err)
		}
	})

	if _, err := pool.Exec(ctx, `
		insert into projects (id, name) values ($1, 'sync endpoints fingerprint test project')
	`, projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into services (id, project_id, name) values ($1, $2, 'sync endpoints fingerprint test service')
	`, serviceID, projectID); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into api_specs (id, service_id, version, content_hash, raw_content, normalized_snapshot, status)
		values ($1, $2, 1, 'hash-v1', '{}'::bytea, '{}'::jsonb, 'imported')
	`, specID, serviceID); err != nil {
		t.Fatalf("insert api spec: %v", err)
	}

	endpoint := Endpoint{
		Method:         "POST",
		Path:           "/items",
		OperationID:    "createItem",
		Summary:        "Create item",
		Tags:           []string{"items"},
		RequestSchema:  json.RawMessage(`{}`),
		ResponseSchema: json.RawMessage(`{}`),
		Fingerprint:    "fp-v1",
	}

	repo := NewRepository(store.Repository{DB: pool})

	if _, created, updated, err := repo.SyncEndpoints(ctx, projectID, serviceID, specID, []Endpoint{endpoint}); err != nil {
		t.Fatalf("first sync: %v", err)
	} else if created != 1 || updated != 0 {
		t.Fatalf("first sync counts: created=%d updated=%d, want 1/0", created, updated)
	}

	endpoint.Summary = "Create item v2"
	endpoint.Fingerprint = endpointFingerprint(endpoint)
	if _, created, updated, err := repo.SyncEndpoints(ctx, projectID, serviceID, specID, []Endpoint{endpoint}); err != nil {
		t.Fatalf("second sync fingerprint change: %v", err)
	} else if created != 0 || updated != 1 {
		t.Fatalf("second sync counts: created=%d updated=%d, want 0/1", created, updated)
	}
}
