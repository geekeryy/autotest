package aiprovider

import (
	"encoding/json"
	"fmt"
	"strings"

	"autotest/internal/aiprovider/client"
)

// ProviderModalityModels maps input modalities to upstream model ids configured
// per AI provider (stored in extra_config.modalityModels).
type ProviderModalityModels struct {
	Image string `json:"image,omitempty"`
	Audio string `json:"audio,omitempty"`
	Video string `json:"video,omitempty"`
}

func (m ProviderModalityModels) IsZero() bool {
	return strings.TrimSpace(m.Image) == "" &&
		strings.TrimSpace(m.Audio) == "" &&
		strings.TrimSpace(m.Video) == ""
}

func parseModalityModels(extra json.RawMessage) ProviderModalityModels {
	var out ProviderModalityModels
	if len(extra) == 0 {
		return out
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(extra, &root); err != nil {
		return out
	}
	raw, ok := root["modalityModels"]
	if !ok || len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func mergeModalityModelsIntoExtra(extra json.RawMessage, models ProviderModalityModels) (json.RawMessage, error) {
	root := map[string]any{}
	if len(extra) > 0 {
		if err := json.Unmarshal(extra, &root); err != nil {
			return nil, fmt.Errorf("extraConfig must be a JSON object: %w", err)
		}
	}
	if models.IsZero() {
		delete(root, "modalityModels")
	} else {
		root["modalityModels"] = models
	}
	if len(root) == 0 {
		return nil, nil
	}
	return json.Marshal(root)
}

func providerModalityModels(row *providerRow) ProviderModalityModels {
	if row == nil {
		return ProviderModalityModels{}
	}
	return parseModalityModels(json.RawMessage(row.ExtraConfig))
}

func normalizeModelID(model string) string {
	s := strings.ToLower(strings.TrimSpace(model))
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// resolveConfiguredModelID picks the exact upstream spelling from gatewayModels
// when the configured id matches after normalization.
func resolveConfiguredModelID(configured string, gatewayModels []client.ModelInfo) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return ""
	}
	want := normalizeModelID(configured)
	for _, m := range gatewayModels {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if id == configured || normalizeModelID(id) == want {
			return id
		}
	}
	return configured
}
