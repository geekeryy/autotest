package logx

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// RequestLogger logs each HTTP request with a level derived from the status code.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(ww, r)

		status := ww.status
		if status == 0 {
			status = http.StatusOK
		}

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", ww.bytes,
			"duration", time.Since(start),
			"remote", r.RemoteAddr,
			"request_id", middleware.GetReqID(r.Context()),
		}
		if q := r.URL.RawQuery; q != "" {
			attrs = append(attrs, "query", q)
		}

		msg := "http request"
		switch {
		case status >= http.StatusInternalServerError:
			logger.Error(msg, attrs...)
		case status >= http.StatusBadRequest:
			logger.Warn(msg, attrs...)
		case r.URL.Path == "/healthz":
			logger.Debug(msg, attrs...)
		default:
			logger.Info(msg, attrs...)
		}
	})
}
