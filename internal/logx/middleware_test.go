package logx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (r *flushRecorder) Flush() {
	r.flushed = true
}

func TestRequestLoggerPreservesFlusher(t *testing.T) {
	t.Parallel()

	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher on wrapped writer")
		}
		f.Flush()
	})

	req := httptest.NewRequest(http.MethodGet, "/projects/x/ai/chat/stream", nil)
	RequestLogger(next).ServeHTTP(rec, req)

	if !rec.flushed {
		t.Fatal("expected Flush to reach underlying writer")
	}
}
