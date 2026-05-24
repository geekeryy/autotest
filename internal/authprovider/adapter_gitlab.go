package authprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

const defaultGitLabBaseURL = "https://gitlab.com"

type gitlabAdapter struct {
	client *http.Client
}

func newGitLabAdapter(client *http.Client) *gitlabAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &gitlabAdapter{client: client}
}

func (a *gitlabAdapter) Type() string        { return TypeGitLab }
func (a *gitlabAdapter) DisplayName() string { return "GitLab" }
func (a *gitlabAdapter) DefaultScopes() []string {
	return []string{"read_user"}
}
func (a *gitlabAdapter) ExtraConfigFields() []ConfigFieldMeta {
	return []ConfigFieldMeta{{
		Key:          "baseUrl",
		Label:        "GitLab 地址",
		DefaultValue: defaultGitLabBaseURL,
		Required:     true,
	}}
}

func gitLabBaseURL(extra json.RawMessage) string {
	if len(extra) == 0 {
		return defaultGitLabBaseURL
	}
	var m map[string]string
	if err := json.Unmarshal(extra, &m); err == nil {
		if v := strings.TrimRight(strings.TrimSpace(m["baseUrl"]), "/"); v != "" {
			return v
		}
	}
	return defaultGitLabBaseURL
}

func gitlabBaseURLFromMap(extra map[string]string) string {
	if v := strings.TrimRight(strings.TrimSpace(extra["baseUrl"]), "/"); v != "" {
		return v
	}
	return defaultGitLabBaseURL
}

func (a *gitlabAdapter) OAuth2Config(cfg ResolvedProvider) (*oauth2.Config, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("gitlab oauth credentials are incomplete")
	}
	base := gitlabBaseURLFromMap(cfg.ExtraConfig)
	authURL, err := url.Parse(base + "/oauth/authorize")
	if err != nil {
		return nil, err
	}
	tokenURL, err := url.Parse(base + "/oauth/token")
	if err != nil {
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.CallbackURL,
		Scopes:       effectiveScopes(cfg, a),
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL.String(),
			TokenURL: tokenURL.String(),
		},
	}, nil
}

type gitlabUserProfile struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func (a *gitlabAdapter) FetchProfile(ctx context.Context, cfg ResolvedProvider, token *oauth2.Token) (*ExternalProfile, error) {
	base := gitlabBaseURLFromMap(cfg.ExtraConfig)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v4/user", nil)
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
		return nil, fmt.Errorf("gitlab profile: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var profile gitlabUserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	if profile.ID == 0 || strings.TrimSpace(profile.Username) == "" {
		return nil, fmt.Errorf("gitlab profile: invalid profile")
	}
	displayName := strings.TrimSpace(profile.Name)
	if displayName == "" {
		displayName = profile.Username
	}
	return &ExternalProfile{
		ExternalID:  fmt.Sprintf("%d", profile.ID),
		Login:       profile.Username,
		DisplayName: displayName,
		Email:       profile.Email,
		AvatarURL:   profile.AvatarURL,
	}, nil
}
