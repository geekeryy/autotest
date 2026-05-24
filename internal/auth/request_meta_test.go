package auth

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestRequestAPIOrigin(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "http://localhost:8080/api/v1/auth/login-providers", nil)
	req.Host = "localhost:8080"
	if got := RequestAPIOrigin(req); got != "http://localhost:8080" {
		t.Fatalf("got %q", got)
	}

	reqTLS := httptest.NewRequest("GET", "https://api.example.com/api/v1/auth/login-providers", nil)
	reqTLS.Host = "api.example.com"
	reqTLS.TLS = &tls.ConnectionState{}
	if got := RequestAPIOrigin(reqTLS); got != "https://api.example.com" {
		t.Fatalf("got %q", got)
	}

	reqFwd := httptest.NewRequest("GET", "http://127.0.0.1/api/v1/auth/login-providers", nil)
	reqFwd.Host = "127.0.0.1"
	reqFwd.Header.Set("X-Forwarded-Proto", "https")
	reqFwd.Header.Set("X-Forwarded-Host", "api.example.com")
	if got := RequestAPIOrigin(reqFwd); got != "https://api.example.com" {
		t.Fatalf("got %q", got)
	}
}
