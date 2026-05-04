package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"autotest/internal/httpx"
)

type contextKey string

const principalKey contextKey = "authPrincipal"

func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			httpx.Error(w, http.StatusUnauthorized, errors.New("missing bearer token"))
			return
		}

		principal, err := s.ValidateToken(r.Context(), token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, err)
			return
		}

		ctx := context.WithValue(r.Context(), principalKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) RequirePermission(code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := PrincipalFromContext(r.Context())
			if principal == nil {
				httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
				return
			}
			if _, ok := principal.Permissions[code]; !ok {
				httpx.Error(w, http.StatusForbidden, errors.New("permission denied"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func PrincipalFromContext(ctx context.Context) *Principal {
	principal, _ := ctx.Value(principalKey).(*Principal)
	return principal
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
