package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// scopeSet 是构造 Principal.Scopes 的便捷函数。
func scopeSet(codes ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		out[code] = struct{}{}
	}
	return out
}

// runWithPrincipal 用给定 Principal 调用 middleware；返回响应状态码与 next 是否被调用。
func runWithPrincipal(t *testing.T, mw func(http.Handler) http.Handler, principal *Principal) (status int, nextCalled bool) {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if principal != nil {
		req = req.WithContext(WithPrincipal(req.Context(), principal))
	}
	rw := httptest.NewRecorder()
	mw(next).ServeHTTP(rw, req)
	return rw.Code, nextCalled
}

func TestAllowAPIKeyScope(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	mw := svc.AllowAPIKeyScope("specs:import")

	cases := []struct {
		name       string
		principal  *Principal
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "missing principal returns 401",
			principal:  nil,
			wantStatus: http.StatusUnauthorized,
			wantNext:   false,
		},
		{
			name:       "jwt principal always allowed",
			principal:  &Principal{UserID: uuid.New(), Source: SourceJWT},
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
		{
			name: "apikey with matching scope allowed",
			principal: &Principal{
				UserID: uuid.New(),
				Source: SourceAPIKey,
				Scopes: scopeSet("specs:import"),
			},
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
		{
			name: "apikey without scope rejected",
			principal: &Principal{
				UserID: uuid.New(),
				Source: SourceAPIKey,
				Scopes: scopeSet("cases:run"),
			},
			wantStatus: http.StatusForbidden,
			wantNext:   false,
		},
		{
			name: "apikey with empty scopes rejected",
			principal: &Principal{
				UserID: uuid.New(),
				Source: SourceAPIKey,
			},
			wantStatus: http.StatusForbidden,
			wantNext:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, called := runWithPrincipal(t, mw, tc.principal)
			if status != tc.wantStatus {
				t.Fatalf("status=%d want %d", status, tc.wantStatus)
			}
			if called != tc.wantNext {
				t.Fatalf("nextCalled=%v want %v", called, tc.wantNext)
			}
		})
	}
}

func TestRejectAPIKey(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	mw := svc.RejectAPIKey()

	cases := []struct {
		name       string
		principal  *Principal
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "missing principal returns 401",
			principal:  nil,
			wantStatus: http.StatusUnauthorized,
			wantNext:   false,
		},
		{
			name:       "jwt principal always allowed",
			principal:  &Principal{UserID: uuid.New(), Source: SourceJWT},
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
		{
			name:       "default empty source treated as jwt",
			principal:  &Principal{UserID: uuid.New()},
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
		{
			name: "apikey source rejected regardless of scope",
			principal: &Principal{
				UserID: uuid.New(),
				Source: SourceAPIKey,
				Scopes: scopeSet("specs:import"),
			},
			wantStatus: http.StatusForbidden,
			wantNext:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, called := runWithPrincipal(t, mw, tc.principal)
			if status != tc.wantStatus {
				t.Fatalf("status=%d want %d", status, tc.wantStatus)
			}
			if called != tc.wantNext {
				t.Fatalf("nextCalled=%v want %v", called, tc.wantNext)
			}
		})
	}
}

func TestRequireActiveUser(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	mw := svc.RequireActiveUser()

	cases := []struct {
		name       string
		principal  *Principal
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "missing principal returns 401",
			principal:  nil,
			wantStatus: http.StatusUnauthorized,
			wantNext:   false,
		},
		{
			name:       "active jwt allowed",
			principal:  &Principal{UserID: uuid.New(), Source: SourceJWT, Active: true},
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
		{
			name:       "inactive jwt rejected with 403",
			principal:  &Principal{UserID: uuid.New(), Source: SourceJWT, Active: false},
			wantStatus: http.StatusForbidden,
			wantNext:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, called := runWithPrincipal(t, mw, tc.principal)
			if status != tc.wantStatus {
				t.Fatalf("status=%d want %d", status, tc.wantStatus)
			}
			if called != tc.wantNext {
				t.Fatalf("nextCalled=%v want %v", called, tc.wantNext)
			}
		})
	}
}

func TestAuthenticateRejectsAPIKeyWhenNoAuthenticator(t *testing.T) {
	t.Parallel()

	svc := &Service{}
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatalf("next should not be called when API Key authentication is unavailable")
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer at-fake-token")
	rw := httptest.NewRecorder()
	svc.Authenticate(next).ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want %d", rw.Code, http.StatusUnauthorized)
	}
}
