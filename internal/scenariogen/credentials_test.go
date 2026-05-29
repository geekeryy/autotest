package scenariogen

import (
	"testing"

	"autotest/internal/spec"
)

func TestExtractCredentialsFromRequest(t *testing.T) {
	raw := []byte(`{"body":{"username":"demo","password":"secret"}}`)
	user, pass, ok := ExtractCredentialsFromRequest(raw)
	if !ok || user != "demo" || pass != "secret" {
		t.Fatalf("unexpected extract: ok=%v user=%q pass=%q", ok, user, pass)
	}
}

func TestResolveLoginCredentialsPriority(t *testing.T) {
	bundle := &LoginCredentialBundle{
		User: &CredentialPair{Username: "cfg", Password: "cfgpass"},
	}
	user, pass, need := ResolveLoginCredentials(false, bundle, "case", "casepass")
	if need || user != "case" || pass != "casepass" {
		t.Fatalf("case body should win: %q %q need=%v", user, pass, need)
	}
	user, pass, need = ResolveLoginCredentials(false, bundle, "", "")
	if need || user != "cfg" || pass != "cfgpass" {
		t.Fatalf("bundle fallback: %q %q need=%v", user, pass, need)
	}
	user, pass, need = ResolveLoginCredentials(false, nil, "", "")
	if !need || user != FillLoginUser {
		t.Fatalf("expected placeholders, got %q %q need=%v", user, pass, need)
	}
}

func TestLoginCredentialsPlaceholderWithoutConfig(t *testing.T) {
	user, pass, need := ResolveLoginCredentials(false, nil, "", "")
	if user != FillLoginUser || pass != FillLoginPassword || !need {
		t.Fatalf("expected placeholders, got %q / %q need=%v", user, pass, need)
	}
	_ = spec.Endpoint{Path: "/api/v1/auth/login"}
}
