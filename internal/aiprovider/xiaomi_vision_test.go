package aiprovider

import "testing"

func TestPickXiaomiVisionModel_Prefers25(t *testing.T) {
	got := pickXiaomiVisionModel("mimo-v2-pro")
	if got != xiaomiVisionModelPreferred {
		t.Fatalf("expected %q, got %q", xiaomiVisionModelPreferred, got)
	}
}

func TestPickXiaomiVisionModel_Keeps25Family(t *testing.T) {
	got := pickXiaomiVisionModel("mimo-v2-5", "mimo-v2-pro")
	if got != "mimo-v2-5" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestPickXiaomiVisionModel_HintFromList(t *testing.T) {
	got := pickXiaomiVisionModel("mimo-v2-pro", "mimo-v2.5", "mimo-v2-pro")
	if got != "mimo-v2.5" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestIsXiaomiVisionModel(t *testing.T) {
	if !isXiaomiVisionModel("mimo-v2.5") || !isXiaomiVisionModel("mimo-v2-omni") {
		t.Fatal("expected vision models")
	}
	if isXiaomiVisionModel("mimo-v2-pro") {
		t.Fatal("pro should not be vision")
	}
}
