package genagent

import (
	"testing"

	"autotest/internal/aiconfig"
)

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
