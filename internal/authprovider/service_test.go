package authprovider

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestMaskKey(t *testing.T) {
	t.Parallel()

	if got := maskKey(""); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := maskKey("abcd"); got != "••••" {
		t.Fatalf("got %q", got)
	}
	if got := maskKey("sk-1234567890"); got != "sk-••••7890" {
		t.Fatalf("got %q", got)
	}
}

func TestGitLabBaseURLDefault(t *testing.T) {
	t.Parallel()

	if got := gitLabBaseURL(nil); got != defaultGitLabBaseURL {
		t.Fatalf("got %q", got)
	}
	if got := gitLabBaseURL(json.RawMessage(`{"baseUrl":"https://gitlab.example.com/"}`)); got != "https://gitlab.example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestGitHubAdapterOAuth2Config(t *testing.T) {
	t.Parallel()

	adapter := newGitHubAdapter(http.DefaultClient)
	cfg := ResolvedProvider{
		ClientID:     "id",
		ClientSecret: "secret",
		CallbackURL:  "http://localhost/callback",
		Scopes:       []string{"read:user"},
	}
	oauthCfg, err := adapter.OAuth2Config(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if oauthCfg.ClientID != "id" || oauthCfg.RedirectURL != cfg.CallbackURL {
		t.Fatalf("oauthCfg=%+v", oauthCfg)
	}
}

func TestGitLabAdapterOAuth2Config(t *testing.T) {
	t.Parallel()

	adapter := newGitLabAdapter(http.DefaultClient)
	cfg := ResolvedProvider{
		ClientID:     "id",
		ClientSecret: "secret",
		CallbackURL:  "http://localhost/callback",
		ExtraConfig:  map[string]string{"baseUrl": "https://gitlab.example.com"},
	}
	oauthCfg, err := adapter.OAuth2Config(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if oauthCfg.Endpoint.AuthURL != "https://gitlab.example.com/oauth/authorize" {
		t.Fatalf("auth url=%q", oauthCfg.Endpoint.AuthURL)
	}
}

func TestGoogleAdapterOAuth2Config(t *testing.T) {
	t.Parallel()

	adapter := newGoogleAdapter(http.DefaultClient)
	cfg := ResolvedProvider{
		ClientID:     "id",
		ClientSecret: "secret",
		CallbackURL:  "http://localhost/callback",
	}
	oauthCfg, err := adapter.OAuth2Config(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if oauthCfg.Endpoint.AuthURL == "" {
		t.Fatal("expected google auth url")
	}
}

func TestRegistrySupportedTypes(t *testing.T) {
	t.Parallel()

	reg := NewRegistry(http.DefaultClient)
	types := reg.SupportedTypes()
	if len(types) != 3 {
		t.Fatalf("got %d types", len(types))
	}
}

func TestValidateSlug(t *testing.T) {
	t.Parallel()

	if err := validateSlug("github"); err != nil {
		t.Fatalf("github: %v", err)
	}
	if err := validateSlug("company-github"); err != nil {
		t.Fatalf("company-github: %v", err)
	}
	if err := validateSlug(""); err == nil {
		t.Fatal("expected empty slug error")
	}
	if err := validateSlug("-bad"); err == nil {
		t.Fatal("expected leading hyphen error")
	}
	if err := validateSlug("bad-"); err == nil {
		t.Fatal("expected trailing hyphen error")
	}
	if err := validateSlug("bad_slug"); err == nil {
		t.Fatal("expected underscore error")
	}
}

func TestIsValidProviderType(t *testing.T) {
	t.Parallel()

	if !isValidProviderType(ProviderTypeGithub) {
		t.Fatal("expected github valid")
	}
	if isValidProviderType("oidc") {
		t.Fatal("expected oidc invalid")
	}
}
