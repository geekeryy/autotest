package aiprovider

import (
	"encoding/base64"
	"strings"
	"testing"
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

func TestModelSupportsVision_Xiaomi(t *testing.T) {
	if !modelSupportsVision(ProviderTypeXiaomi, "mimo-v2.5") {
		t.Fatal("2.5 model should support vision")
	}
	if !modelSupportsVision(ProviderTypeXiaomi, "mimo-v2-omni") {
		t.Fatal("omni model should support vision")
	}
	if modelSupportsVision(ProviderTypeXiaomi, "mimo-v2-pro") {
		t.Fatal("pro model should not support vision")
	}
}

func TestValidateAssistantImageURL_RejectsUnknownMIME(t *testing.T) {
	err := validateAssistantImageURL("data:image/bmp;base64,AA==")
	if err == nil || !strings.Contains(err.Error(), "JPEG") {
		t.Fatalf("unexpected err: %v", err)
	}
}
