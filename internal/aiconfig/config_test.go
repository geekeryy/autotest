package aiconfig

import (
	"errors"
	"testing"
)

func TestDefaultConfigAndEnvMerge(t *testing.T) {
	t.Setenv("AI_MAX_TOOL_HOPS", "8")
	t.Setenv("AI_TOOL_ROUTING_MODE", "dynamic")
	t.Setenv("AI_SCENARIO_AUTORUN", "on")

	cfg := MergeEnv(DefaultConfig())
	if cfg.MaxToolHops != 8 {
		t.Fatalf("maxToolHops = %d, want 8", cfg.MaxToolHops)
	}
	if cfg.ToolRoutingMode != "dynamic" {
		t.Fatalf("toolRoutingMode = %q", cfg.ToolRoutingMode)
	}
	if !cfg.ScenarioAutorunEnabled {
		t.Fatal("expected scenarioAutorunEnabled")
	}
}

func TestApplyStoredOverridesEnv(t *testing.T) {
	t.Setenv("AI_MAX_TOOL_HOPS", "8")
	base := MergeEnv(DefaultConfig())
	stored := storedConfig{
		MaxToolHops: intPtr(4),
		ToolRoutingMode: strPtr("shadow"),
	}
	cfg := ApplyStored(base, stored)
	if cfg.MaxToolHops != 4 {
		t.Fatalf("maxToolHops = %d, want 4", cfg.MaxToolHops)
	}
}

func TestUpdateValidation(t *testing.T) {
	svc := NewService(nil, NewStore())
	bad := 0
	_, err := svc.Update(t.Context(), UpdateInput{MaxToolHops: &bad})
	if !errors.Is(err, ErrInvalidMaxToolHops) {
		t.Fatalf("expected ErrInvalidMaxToolHops, got %v", err)
	}
	mode := "invalid-mode"
	_, err = svc.Update(t.Context(), UpdateInput{ToolRoutingMode: &mode})
	if !errors.Is(err, ErrInvalidToolRoutingMode) {
		t.Fatalf("expected ErrInvalidToolRoutingMode, got %v", err)
	}
}

func TestUpdateKeepsSecrets(t *testing.T) {
	store := NewStore()
	store.Set(Config{
		MaxToolHops:     6,
		ToolRoutingMode: "shadow",
		ToolEmbedAPIKey: "sk-secret-key",
		ToolEmbedModel:  DefaultToolEmbedModel,
	})
	svc := NewService(nil, store)

	hops := 7
	view, err := svc.Update(t.Context(), UpdateInput{MaxToolHops: &hops})
	if err != nil {
		t.Fatal(err)
	}
	if view.Settings.MaxToolHops != 7 {
		t.Fatalf("maxToolHops = %d", view.Settings.MaxToolHops)
	}
	if !view.Settings.ToolEmbedAPIKeyPresent {
		t.Fatal("expected tool embed key present")
	}
	if view.Settings.ToolEmbedAPIKeyMasked == "" {
		t.Fatal("expected masked key")
	}
}

func TestMaskSecret(t *testing.T) {
	if MaskSecret("") != "" {
		t.Fatal("empty mask")
	}
	if MaskSecret("ab") != "••••" {
		t.Fatalf("short mask = %q", MaskSecret("ab"))
	}
}
