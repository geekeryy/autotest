package mcp

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// LoopbackAPIBaseURL builds the REST base URL MCP tools use to call this API process
// (e.g. ADDR=:8080 → http://127.0.0.1:8080/api/v1).
func LoopbackAPIBaseURL(listenAddr string) string {
	addr := strings.TrimSpace(listenAddr)
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return fmt.Sprintf("http://127.0.0.1%s/api/v1", addr)
		}
		return "http://127.0.0.1:8080/api/v1"
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s/api/v1", net.JoinHostPort(host, port))
}

// ConfigFromRequest merges HTTP defaults with Authorization: Bearer at-... from the client.
func ConfigFromRequest(r *http.Request, defaults Config) (Config, error) {
	key, err := bearerAPIKey(r.Header)
	if err != nil {
		return Config{}, err
	}
	cfg := defaults
	cfg.APIKey = key
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	return cfg, nil
}

func bearerAPIKey(h http.Header) (string, error) {
	raw := strings.TrimSpace(h.Get("Authorization"))
	if raw == "" {
		return "", errorsMissingAPIKey()
	}
	const prefix = "bearer "
	if len(raw) < len(prefix) || !strings.EqualFold(raw[:len(prefix)], prefix) {
		return "", errorsMissingAPIKey()
	}
	key := strings.TrimSpace(raw[len(prefix):])
	if key == "" {
		return "", errorsMissingAPIKey()
	}
	if !strings.HasPrefix(key, "at-") {
		return "", fmt.Errorf("API Key must start with at-")
	}
	return key, nil
}

func errorsMissingAPIKey() error {
	return fmt.Errorf("Authorization: Bearer at-... is required")
}

// NewHTTPHandler serves MCP over Streamable HTTP. Each session uses the API Key from
// the initialize request's Authorization header; optional defaults come from defaults.
func NewHTTPHandler(defaults Config) http.Handler {
	inner := sdkmcp.NewStreamableHTTPHandler(func(r *http.Request) *sdkmcp.Server {
		cfg, err := ConfigFromRequest(r, defaults)
		if err != nil {
			return nil
		}
		return NewServer(cfg)
	}, &sdkmcp.StreamableHTTPOptions{})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := ConfigFromRequest(r, defaults); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	})
}
