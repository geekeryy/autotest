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

func TestModelHasCapability_EmptyTagsTextOnly(t *testing.T) {
	if !ModelHasCapability(nil, ModalityText) {
		t.Fatal("empty capabilities should allow text")
	}
	if ModelHasCapability(nil, ModalityImage) {
		t.Fatal("empty capabilities should not imply image")
	}
}
