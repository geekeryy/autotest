package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	testcase "autotest/internal/case"
	"autotest/internal/project"
	"autotest/internal/report"

	"github.com/google/uuid"
)

// caseTargeting builds a minimal test case whose request points at url.
func caseTargeting(t *testing.T, url, method string) testcase.TestCase {
	t.Helper()
	raw, err := json.Marshal(RequestDefinition{Method: method, URL: url})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return testcase.TestCase{ID: uuid.New(), Method: method, Request: raw}
}

func TestExecuteCaseRetriesIdempotentOnStatus(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := New(nil, nil, nil)
	res, err := r.ExecuteCaseWithStepID(context.Background(), uuid.New(), uuid.New(),
		caseTargeting(t, srv.URL, http.MethodGet), project.Environment{}, nil, nil, nil,
		WithStepRetry(3, time.Millisecond, []int{http.StatusServiceUnavailable}, false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("expected 3 attempts (2 retries), got %d", got)
	}
	if res.Status != report.ResultPassed {
		t.Fatalf("expected final result passed, got %s", res.Status)
	}
}

func TestExecuteCaseDoesNotRetryPOSTByDefault(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	r := New(nil, nil, nil)
	_, err := r.ExecuteCaseWithStepID(context.Background(), uuid.New(), uuid.New(),
		caseTargeting(t, srv.URL, http.MethodPost), project.Environment{}, nil, nil, nil,
		WithStepRetry(3, time.Millisecond, []int{http.StatusServiceUnavailable}, false))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("POST must not retry without opt-in: expected 1 attempt, got %d", got)
	}
}

func TestExecuteCaseRetriesPOSTWhenAllowed(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := New(nil, nil, nil)
	_, err := r.ExecuteCaseWithStepID(context.Background(), uuid.New(), uuid.New(),
		caseTargeting(t, srv.URL, http.MethodPost), project.Environment{}, nil, nil, nil,
		WithStepRetry(3, time.Millisecond, []int{http.StatusServiceUnavailable}, true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected POST to retry once when allowed, got %d attempts", got)
	}
}

func TestExecuteCaseStepTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := New(nil, nil, nil)
	res, err := r.ExecuteCaseWithStepID(context.Background(), uuid.New(), uuid.New(),
		caseTargeting(t, srv.URL, http.MethodGet), project.Environment{}, nil, nil, nil,
		WithStepTimeout(30*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != report.ResultError {
		t.Fatalf("expected timeout to yield error result, got %s", res.Status)
	}
}

func TestExecuteCaseNoRetryByDefault(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	r := New(nil, nil, nil)
	if _, err := r.ExecuteCaseWithStepID(context.Background(), uuid.New(), uuid.New(),
		caseTargeting(t, srv.URL, http.MethodGet), project.Environment{}, nil, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected exactly 1 attempt with no retry option, got %d", got)
	}
}
