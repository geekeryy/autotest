package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"autotest/internal/httpx"
)

type contextKey string

const principalKey contextKey = "authPrincipal"

// APIKeyPrefix 是合法 API Key 的固定前缀。Authenticate 通过该前缀判别
// 当前请求走 JWT 还是 API Key 校验链路。
const APIKeyPrefix = "at-"

func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			httpx.Error(w, http.StatusUnauthorized, errors.New("missing bearer token"))
			return
		}

		var (
			principal *Principal
			err       error
		)
		if strings.HasPrefix(token, APIKeyPrefix) {
			if s.apiKey == nil {
				httpx.Error(w, http.StatusUnauthorized, errors.New("API Key 认证未启用"))
				return
			}
			principal, err = s.apiKey.Authenticate(r.Context(), token)
		} else {
			principal, err = s.ValidateToken(r.Context(), token)
		}
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, err)
			return
		}
		if principal == nil {
			httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
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

// AllowAPIKeyScope 默认放行 JWT；当 Principal 来自 API Key 时，
// 必须具备指定 scope 才允许通过。用于挂在白名单接口（如 OpenAPI 导入）上。
func (s *Service) AllowAPIKeyScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := PrincipalFromContext(r.Context())
			if principal == nil {
				httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
				return
			}
			if principal.Source == SourceAPIKey {
				if _, ok := principal.Scopes[scope]; !ok {
					httpx.Error(w, http.StatusForbidden, fmt.Errorf("API Key 缺少 %s 作用域", scope))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RejectAPIKey 默认放行 JWT；只要 Principal 来自 API Key 就直接 403。
// 用作通用路由组兜底，避免 API Key 调用未显式开放的接口。
func (s *Service) RejectAPIKey() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := PrincipalFromContext(r.Context())
			if principal == nil {
				httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
				return
			}
			if principal.Source == SourceAPIKey {
				httpx.Error(w, http.StatusForbidden, errors.New("当前接口不支持 API Key 调用"))
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

// WithPrincipal 仅供测试与受信任内部链路注入 Principal 到 context。
func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
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
