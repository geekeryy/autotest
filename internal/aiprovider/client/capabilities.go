package client

import (
	"encoding/json"
	"strings"
)

// Modality constants describe input/output kinds for routing and UI tags.
const (
	ModalityText  = "text"
	ModalityImage = "image"
	ModalityAudio = "audio"
	ModalityVideo = "video"
)

var allModalities = []string{ModalityText, ModalityImage, ModalityAudio, ModalityVideo}

// ModelCapabilitiesFromMetadata extracts capability tags from an upstream model
// JSON object (OpenAI-compatible /models row or similar).
func ModelCapabilitiesFromMetadata(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(tags ...string) {
		for _, t := range tags {
			t = strings.ToLower(strings.TrimSpace(t))
			if t == "" {
				continue
			}
			switch t {
			case ModalityText, ModalityImage, ModalityAudio, ModalityVideo,
				"vision", "visual", "multimodal", "mm":
				if t == "vision" || t == "visual" || t == "multimodal" || t == "mm" {
					seen[ModalityImage] = struct{}{}
				} else {
					seen[t] = struct{}{}
				}
			}
		}
	}
	for _, key := range []string{
		"capabilities", "modalities", "input_modalities", "output_modalities",
		"supported_modalities", "modality",
	} {
		collectCapabilityValues(obj[key], add)
	}
	if arch, ok := obj["architecture"].(map[string]any); ok {
		collectCapabilityValues(arch["input_modalities"], add)
		collectCapabilityValues(arch["output_modalities"], add)
	}
	return capabilityKeys(seen)
}

func collectCapabilityValues(v any, add func(...string)) {
	switch t := v.(type) {
	case string:
		add(splitCapabilityTokens(t)...)
	case []any:
		for _, item := range t {
			collectCapabilityValues(item, add)
		}
	case map[string]any:
		for k, val := range t {
			if isTruthy(val) {
				add(k)
			}
			collectCapabilityValues(val, add)
		}
	}
}

func splitCapabilityTokens(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, ",", " ")
	s = strings.ReplaceAll(s, "|", " ")
	s = strings.ReplaceAll(s, ";", " ")
	return strings.Fields(s)
}

func isTruthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return false
	}
}

func capabilityKeys(seen map[string]struct{}) []string {
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for _, m := range allModalities {
		if _, ok := seen[m]; ok {
			out = append(out, m)
		}
	}
	return out
}

// ModelHasCapability reports whether tags include a modality (text is assumed
// when tags are empty).
func ModelHasCapability(capabilities []string, modality string) bool {
	modality = strings.ToLower(strings.TrimSpace(modality))
	if modality == ModalityText {
		if len(capabilities) == 0 {
			return true
		}
		for _, c := range capabilities {
			if c == ModalityText {
				return true
			}
		}
		return !hasNonTextOnlyCapabilities(capabilities)
	}
	for _, c := range capabilities {
		if c == modality {
			return true
		}
	}
	return false
}

// ModalityLabelZH returns a short Chinese label for user-facing errors.
func ModalityLabelZH(modality string) string {
	switch modality {
	case ModalityImage:
		return "图片"
	case ModalityAudio:
		return "音频"
	case ModalityVideo:
		return "视频"
	default:
		return modality
	}
}

func hasNonTextOnlyCapabilities(capabilities []string) bool {
	for _, c := range capabilities {
		if c != ModalityText {
			return true
		}
	}
	return false
}
