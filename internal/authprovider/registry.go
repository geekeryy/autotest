package authprovider

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// OAuthAdapter 内置 OAuth 类型适配器。
type OAuthAdapter interface {
	Type() string
	DisplayName() string
	DefaultScopes() []string
	ExtraConfigFields() []ConfigFieldMeta
	OAuth2Config(cfg ResolvedProvider) (*oauth2.Config, error)
	FetchProfile(ctx context.Context, cfg ResolvedProvider, token *oauth2.Token) (*ExternalProfile, error)
}

// Registry 持有已注册的 OAuth 适配器。
type Registry struct {
	byType map[string]OAuthAdapter
}

// NewRegistry 返回内置 github/gitlab/google 适配器注册表。
func NewRegistry(client *http.Client) *Registry {
	if client == nil {
		client = http.DefaultClient
	}
	r := &Registry{byType: map[string]OAuthAdapter{}}
	for _, a := range []OAuthAdapter{
		newGitHubAdapter(client),
		newGitLabAdapter(client),
		newGoogleAdapter(client),
	} {
		r.byType[a.Type()] = a
	}
	return r
}

// DefaultRegistry 使用 http.DefaultClient。
func DefaultRegistry() *Registry {
	return NewRegistry(http.DefaultClient)
}

func (r *Registry) Get(providerType string) (OAuthAdapter, error) {
	a, ok := r.byType[providerType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderTypeInvalid, providerType)
	}
	return a, nil
}

func (r *Registry) SupportedTypes() []ProviderTypeMeta {
	types := []string{TypeGitHub, TypeGitLab, TypeGoogle}
	out := make([]ProviderTypeMeta, 0, len(types))
	for _, t := range types {
		a := r.byType[t]
		out = append(out, ProviderTypeMeta{
			Type:              a.Type(),
			DisplayName:       a.DisplayName(),
			DefaultScopes:     a.DefaultScopes(),
			ExtraConfigFields: a.ExtraConfigFields(),
		})
	}
	return out
}

func effectiveScopes(cfg ResolvedProvider, adapter OAuthAdapter) []string {
	if len(cfg.Scopes) > 0 {
		return cfg.Scopes
	}
	return adapter.DefaultScopes()
}

func isValidProviderType(t string) bool {
	switch t {
	case TypeGitHub, TypeGitLab, TypeGoogle:
		return true
	default:
		return false
	}
}
