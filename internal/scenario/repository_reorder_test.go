package scenario

import (
	"context"
	"testing"
	"time"

	"autotest/internal/config"
	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func integrationDatabaseURL(t *testing.T) string {
	t.Helper()
	databaseURL, err := config.DatabaseURLFromEnv()
	if err != nil {
		t.Skipf("POSTGRES_* is required for repository integration test: %v", err)
	}
	return databaseURL
}

func TestRepositoryReorderStepsAvoidsScratchAndSoftDeleteConflicts(t *testing.T) {
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
	scenarioID := uuid.New()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, `delete from projects where id = $1`, projectID); err != nil {
			t.Logf("cleanup project: %v", err)
		}
	})

	if _, err := pool.Exec(ctx, `
		insert into projects (id, name) values ($1, 'reorder test project')
	`, projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into services (id, project_id, name) values ($1, $2, 'reorder test service')
	`, serviceID, projectID); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into test_scenarios (id, project_id, service_id, name)
		values ($1, $2, $3, 'reorder test scenario')
	`, scenarioID, projectID, serviceID); err != nil {
		t.Fatalf("insert scenario: %v", err)
	}

	firstID := uuid.New()
	secondID := uuid.New()
	thirdID := uuid.New()
	deletedID := uuid.New()
	for _, step := range []struct {
		id        uuid.UUID
		seq       int
		order     int
		deletedAt any
	}{
		{id: firstID, seq: 1, order: 1},
		{id: secondID, seq: 2, order: 1_000_001},
		{id: thirdID, seq: 3, order: 1_000_002},
		{id: deletedID, seq: 4, order: 2, deletedAt: time.Now()},
	} {
		if _, err := pool.Exec(ctx, `
			insert into test_scenario_steps
				(id, scenario_id, step_seq, step_order, step_type, config, request_override, deleted_at)
			values ($1, $2, $3, $4, 'script', '{}'::jsonb, '{}'::jsonb, $5)
		`, step.id, scenarioID, step.seq, step.order, step.deletedAt); err != nil {
			t.Fatalf("insert step order %d: %v", step.order, err)
		}
	}

	repo := NewRepository(store.Repository{DB: pool})
	wantOrder := []uuid.UUID{thirdID, secondID, firstID}
	if err := repo.ReorderSteps(ctx, scenarioID, wantOrder); err != nil {
		t.Fatalf("reorder steps: %v", err)
	}

	steps, err := repo.ListSteps(ctx, scenarioID)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != len(wantOrder) {
		t.Fatalf("expected %d active steps, got %d", len(wantOrder), len(steps))
	}
	for i, step := range steps {
		if step.ID != wantOrder[i] {
			t.Fatalf("step %d id = %s, want %s", i, step.ID, wantOrder[i])
		}
		if step.StepOrder != i+1 {
			t.Fatalf("step %s order = %d, want %d", step.ID, step.StepOrder, i+1)
		}
	}
}
