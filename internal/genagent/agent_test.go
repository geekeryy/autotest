package genagent

import (
	"context"
	"encoding/json"
	"testing"

	"autotest/internal/aiconfig"

	"github.com/google/uuid"
)

type fakeJobs struct {
	job *Job
}

func (f *fakeJobs) Create(_ context.Context, cfg RunConfig) (*Job, error) {
	f.job = &Job{
		ID:            uuid.New(),
		ProjectID:     cfg.ProjectID,
		ServiceID:     cfg.ServiceID,
		EnvironmentID: cfg.EnvironmentID,
		Status:        JobPending,
	}
	raw, _ := json.Marshal(cfg)
	f.job.Config = raw
	return f.job, nil
}

func (f *fakeJobs) Update(_ context.Context, jobID uuid.UUID, status JobStatus, rounds int, result json.RawMessage) error {
	f.job.Status = status
	f.job.Rounds = rounds
	f.job.Result = result
	return nil
}

func (f *fakeJobs) Get(_ context.Context, projectID, jobID uuid.UUID) (*Job, error) {
	return f.job, nil
}

func TestScenarioAutorunEnabled(t *testing.T) {
	store := aiconfig.NewStore()
	aiconfig.SetGlobalStore(store)
	t.Cleanup(func() { aiconfig.SetGlobalStore(nil) })

	cfg := aiconfig.DefaultConfig()
	cfg.ScenarioAutorunEnabled = true
	store.Set(cfg)
	if !ScenarioAutorunEnabled() {
		t.Fatal("expected enabled")
	}
	cfg.ScenarioAutorunEnabled = false
	store.Set(cfg)
	if ScenarioAutorunEnabled() {
		t.Fatal("expected disabled")
	}
}

func TestRunConfigDefaultMaxRounds(t *testing.T) {
	cfg := RunConfig{MaxRounds: 0}
	if cfg.normalizedMaxRounds() != DefaultMaxRounds {
		t.Fatalf("expected %d, got %d", DefaultMaxRounds, cfg.normalizedMaxRounds())
	}
}
