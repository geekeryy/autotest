package authprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type googleAdapter struct {
	client *http.Client
}

func newGoogleAdapter(client *http.Client) *googleAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &googleAdapter{client: client}
}

func (a *googleAdapter) Type() string        { return TypeGoogle }
func (a *googleAdapter) DisplayName() string { return "Google" }
func (a *googleAdapter) DefaultScopes() []string {
	return []string{"openid", "profile", "email"}
}
func (a *googleAdapter) ExtraConfigFields() []ConfigFieldMeta { return nil }

func (a *googleAdapter) OAuth2Config(cfg ResolvedProvider) (*oauth2.Config, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("google oauth credentials are incomplete")
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.CallbackURL,
		Scopes:       effectiveScopes(cfg, a),
		Endpoint:     google.Endpoint,
	}, nil
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

func (a *googleAdapter) FetchProfile(ctx context.Context, cfg ResolvedProvider, token *oauth2.Token) (*ExternalProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("google profile: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var profile googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	if strings.TrimSpace(profile.Sub) == "" {
		return nil, fmt.Errorf("google profile: missing sub")
	}
	displayName := strings.TrimSpace(profile.Name)
	if displayName == "" {
		displayName = profile.Email
	}
	login := profile.Email
	if login == "" {
		login = profile.Sub
	}
	return &ExternalProfile{
		ExternalID:  profile.Sub,
		Login:       login,
		DisplayName: displayName,
		Email:       profile.Email,
		AvatarURL:   profile.Picture,
	}, nil
}
