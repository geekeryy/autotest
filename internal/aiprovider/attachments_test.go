package aiprovider

import (
	"encoding/base64"
	"strings"
	"testing"

	"autotest/internal/aiprovider/client"
)

func TestNormalizeUserImages_RejectsOversizedDataURL(t *testing.T) {
	blob := make([]byte, maxAssistantImageBytes+1)
	encoded := base64.StdEncoding.EncodeToString(blob)
	url := "data:image/png;base64," + encoded
	_, err := normalizeUserImages([]UserImageInput{{URL: url}})
	if err == nil {
		t.Fatal("expected size limit error")
	}
}

func TestNormalizeUserImages_AcceptsHTTPS(t *testing.T) {
	got, err := normalizeUserImages([]UserImageInput{{URL: "https://example.com/a.jpg", Name: "a"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Type != "image" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestModelSupportsModality_FromGateway(t *testing.T) {
	gateway := []client.ModelInfo{
		{ID: "mm", Capabilities: []string{client.ModalityImage}},
	}
	if !modelSupportsModality("xiaomi", "mm", client.ModalityImage, gateway) {
		t.Fatal("expected image capability")
	}
	if modelSupportsModality("xiaomi", "text-only", client.ModalityImage, gateway) {
		t.Fatal("text-only should not match")
	}
}

func TestValidateAssistantImageURL_RejectsUnknownMIME(t *testing.T) {
	err := validateAssistantImageURL("data:image/bmp;base64,AA==")
	if err == nil || !strings.Contains(err.Error(), "JPEG") {
		t.Fatalf("unexpected err: %v", err)
	}
}
