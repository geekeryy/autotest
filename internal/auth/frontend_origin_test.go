package auth

import (
	"testing"
	"time"
)

func TestOAuthStateRoundTripWithFrontendURL(t *testing.T) {
	t.Parallel()

	svc := &Service{secret: []byte("test-secret")}
	state, err := svc.OAuthStateForTest(time.Now().Add(5*time.Minute), "https://app.example.com")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := svc.verifyOAuthState(state)
	if err != nil {
		t.Fatalf("verifyOAuthState: %v", err)
	}
	if payload.FrontendURL != "https://app.example.com" {
		t.Fatalf("frontendUrl=%q", payload.FrontendURL)
	}
}

func TestOAuthStateExpiredWithFrontendURL(t *testing.T) {
	t.Parallel()

	svc := &Service{secret: []byte("test-secret")}
	state, err := svc.OAuthStateForTest(time.Now().Add(-time.Minute), "https://app.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.verifyOAuthState(state); err == nil {
		t.Fatal("expected expired state error")
	}
}

func TestNormalizeOrigin(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://app.example.com/", "https://app.example.com", true},
		{"http://localhost:5173", "http://localhost:5173", true},
		{"https://app.example.com/path", "", false},
		{"ftp://app.example.com", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, err := NormalizeOrigin(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("NormalizeOrigin(%q): %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("NormalizeOrigin(%q) expected error", tc.in)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("NormalizeOrigin(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidateFrontendURLForProvider(t *testing.T) {
	t.Parallel()

	svc := &Service{isDevelopment: false}

	got, err := svc.validateFrontendURLForProvider("https://app.example.com/", []string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://app.example.com" {
		t.Fatalf("got %q", got)
	}

	if _, err := svc.validateFrontendURLForProvider("https://evil.example.com", []string{"https://app.example.com"}); err == nil {
		t.Fatal("expected untrusted origin error")
	}
}

func TestValidateFrontendURLDevelopmentDefaults(t *testing.T) {
	t.Parallel()

	svc := &Service{isDevelopment: true}
	got, err := svc.validateFrontendURLForProvider("http://localhost:5173", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://localhost:5173" {
		t.Fatalf("got %q", got)
	}
}
