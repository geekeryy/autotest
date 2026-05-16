package aiprovider

import "strings"

// xiaomiVisionModelPreferred is the default upstream model id for image turns.
// Gateways may expose mimo-v2.5 or mimo-v2-5; pickXiaomiVisionModel prefers a
// match from the caller's hint list when present.
const xiaomiVisionModelPreferred = "mimo-v2.5"

var xiaomiVisionModelCandidates = []string{
	"mimo-v2.5",
	"mimo-v2-5",
	"mimo-v2.5-preview",
	"mimo-v2-omni",
}

// isXiaomiVisionModel reports whether an upstream model id accepts image_url input.
func isXiaomiVisionModel(model string) bool {
	id := strings.ToLower(strings.TrimSpace(model))
	if id == "" {
		return false
	}
	if strings.Contains(id, "omni") {
		return true
	}
	return strings.Contains(id, "2.5") || strings.Contains(id, "v2-5")
}

// pickXiaomiVisionModel chooses the model id for a vision turn. When the user
// already selected a vision-capable 2.5-family model, it is kept; otherwise we
// prefer MiMo-V2.5 and fall back to omni if hints suggest only that endpoint.
func pickXiaomiVisionModel(current string, hints ...string) string {
	cur := strings.TrimSpace(current)
	if isXiaomiVisionModel(cur) {
		lower := strings.ToLower(cur)
		if strings.Contains(lower, "2.5") || strings.Contains(lower, "v2-5") {
			return cur
		}
	}
	for _, hint := range hints {
		h := strings.TrimSpace(hint)
		if h == "" {
			continue
		}
		if isXiaomiVisionModel(h) {
			lower := strings.ToLower(h)
			if strings.Contains(lower, "2.5") || strings.Contains(lower, "v2-5") {
				return h
			}
		}
	}
	for _, want := range xiaomiVisionModelCandidates {
		for _, hint := range hints {
			if strings.EqualFold(strings.TrimSpace(hint), want) {
				return strings.TrimSpace(hint)
			}
		}
	}
	return xiaomiVisionModelPreferred
}

func applyXiaomiVisionRouting(cfg *streamConfig, history []StoredMessage) {
	if cfg == nil || cfg.Provider == nil || !assistantAllowsImages(cfg.Provider) {
		return
	}
	if !historyHasImageAttachments(history) {
		return
	}
	cfg.BaseOpts.Model = pickXiaomiVisionModel(
		cfg.BaseOpts.Model,
		cfg.Provider.DefaultModel,
	)
	cfg.VisionEnabled = true
}
