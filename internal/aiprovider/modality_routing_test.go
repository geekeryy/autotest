package aiprovider

import (
	"encoding/json"
	"testing"

	"autotest/internal/aiprovider/client"
)

func TestPickModalityModel_UsesConfiguredImageModel(t *testing.T) {
	gateway := []client.ModelInfo{
		{ID: "text-pro", Capabilities: []string{client.ModalityText}},
		{ID: "vendor-v2-5", Capabilities: []string{client.ModalityText, client.ModalityImage}},
	}
	got, err := pickModalityModel("text-pro", "vendor-v2-5", client.ModalityImage, gateway)
	if err != nil {
		t.Fatal(err)
	}
	if got != "vendor-v2-5" {
		t.Fatalf("got %q", got)
	}
}

func TestPickModalityModel_KeepsCapableCurrent(t *testing.T) {
	gateway := []client.ModelInfo{
		{ID: "mm-large", Capabilities: []string{client.ModalityText, client.ModalityImage}},
	}
	got, err := pickModalityModel("mm-large", "", client.ModalityImage, gateway)
	if err != nil {
		t.Fatal(err)
	}
	if got != "mm-large" {
		t.Fatalf("got %q", got)
	}
}

func TestPickModalityModel_MissingConfig(t *testing.T) {
	_, err := pickModalityModel("text-pro", "", client.ModalityImage, nil)
	if err == nil {
		t.Fatal("expected error when image model not configured")
	}
}

func TestParseModalityModels_ExtraConfig(t *testing.T) {
	extra := json.RawMessage(`{"organization":"o","modalityModels":{"image":"img-1"}}`)
	mm := parseModalityModels(extra)
	if mm.Image != "img-1" {
		t.Fatalf("got %+v", mm)
	}
}

func TestResolveConfiguredModelID_Normalized(t *testing.T) {
	gateway := []client.ModelInfo{{ID: "MIMO-V2-5", Capabilities: []string{client.ModalityImage}}}
	got := resolveConfiguredModelID("mimo-v2.5", gateway)
	if got != "MIMO-V2-5" {
		t.Fatalf("got %q", got)
	}
}
