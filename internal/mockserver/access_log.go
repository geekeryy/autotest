package mockserver

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w}
}

func (w *responseRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.body.Len() < maxAccessLogBodyBytes {
		remaining := maxAccessLogBodyBytes - w.body.Len()
		if len(b) <= remaining {
			_, _ = w.body.Write(b)
		} else {
			_, _ = w.body.Write(b[:remaining])
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *responseRecorder) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func truncateAccessLogBody(body string) string {
	if len(body) <= maxAccessLogBodyBytes {
		return body
	}
	return body[:maxAccessLogBodyBytes] + "...(truncated)"
}

func captureRequestHeaders(headers http.Header) json.RawMessage {
	keys := []string{"Content-Type", "Authorization", "User-Agent", "Accept"}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value := headers.Get(key); value != "" {
			if strings.EqualFold(key, "Authorization") {
				out[key] = "***"
			} else {
				out[key] = value
			}
		}
	}
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "x-") {
			out[key] = values[0]
		}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

func captureResponseHeaders(headers http.Header) json.RawMessage {
	out := make(map[string]string)
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		if strings.HasPrefix(strings.ToLower(key), "access-control-") {
			continue
		}
		out[key] = values[0]
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

func clientIPFromRequest(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if idx := strings.Index(forwarded, ","); idx >= 0 {
			return strings.TrimSpace(forwarded[:idx])
		}
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
