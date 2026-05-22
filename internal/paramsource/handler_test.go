package paramsource

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"autotest/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestDataSourcesRejectMissingPrincipal(t *testing.T) {
	t.Parallel()

	authSvc := &auth.Service{}
	h := NewHandler(&Service{}, authSvc)
	r := chi.NewRouter()
	h.Register(r)

	req := httptest.NewRequest(http.MethodGet, "/data-sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCreateDataSourceRequiresProjectsWrite(t *testing.T) {
	t.Parallel()

	authSvc := &auth.Service{}
	h := NewHandler(&Service{}, authSvc)
	r := chi.NewRouter()
	h.Register(r)

	principal := &auth.Principal{
		UserID:      uuid.New(),
		Active:      true,
		Permissions: map[string]struct{}{auth.PermissionProjectsRead: {}},
	}
	req := httptest.NewRequest(http.MethodPost, "/data-sources", nil)
	req = req.WithContext(auth.WithPrincipal(req.Context(), principal))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusForbidden)
	}
}
