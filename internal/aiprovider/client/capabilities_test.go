package client

import (
	"encoding/json"
	"testing"
)

func TestModelCapabilitiesFromMetadata_OpenAIStyle(t *testing.T) {
	raw := json.RawMessage(`{"id":"gpt-4o","capabilities":{"vision":true}}`)
	got := ModelCapabilitiesFromMetadata(raw)
	if !ModelHasCapability(got, ModalityImage) {
		t.Fatalf("expected image, got %v", got)
	}
}

func TestInferModelCapabilities_KeywordHints(t *testing.T) {
	got := InferModelCapabilities("vendor-vision-large", nil)
	if !ModelHasCapability(got, ModalityImage) {
		t.Fatalf("expected image from id hint, got %v", got)
	}
}

func TestInferModelCapabilities_WhisperAudio(t *testing.T) {
	got := InferModelCapabilities("whisper-1", nil)
	if !ModelHasCapability(got, ModalityAudio) {
		t.Fatalf("expected audio, got %v", got)
	}
}
