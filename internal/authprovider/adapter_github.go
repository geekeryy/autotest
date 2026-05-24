package authprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type githubAdapter struct {
	client *http.Client
}

func newGitHubAdapter(client *http.Client) *githubAdapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &githubAdapter{client: client}
}

func (a *githubAdapter) Type() string        { return TypeGitHub }
func (a *githubAdapter) DisplayName() string { return "GitHub" }
func (a *githubAdapter) DefaultScopes() []string {
	return []string{"read:user", "user:email"}
}
func (a *githubAdapter) ExtraConfigFields() []ConfigFieldMeta { return nil }

func (a *githubAdapter) OAuth2Config(cfg ResolvedProvider) (*oauth2.Config, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, fmt.Errorf("github oauth credentials are incomplete")
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.CallbackURL,
		Scopes:       effectiveScopes(cfg, a),
		Endpoint:     github.Endpoint,
	}, nil
}

type githubUserProfile struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func (a *githubAdapter) FetchProfile(ctx context.Context, cfg ResolvedProvider, token *oauth2.Token) (*ExternalProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github profile: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var profile githubUserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	if profile.ID == 0 || strings.TrimSpace(profile.Login) == "" {
		return nil, fmt.Errorf("github profile: invalid profile")
	}
	if strings.TrimSpace(profile.Email) == "" {
		if email, err := fetchGitHubPrimaryEmail(ctx, token.AccessToken, a.client); err == nil {
			profile.Email = email
		}
	}
	displayName := strings.TrimSpace(profile.Name)
	if displayName == "" {
		displayName = profile.Login
	}
	return &ExternalProfile{
		ExternalID:  fmt.Sprintf("%d", profile.ID),
		Login:       profile.Login,
		DisplayName: displayName,
		Email:       profile.Email,
		AvatarURL:   profile.AvatarURL,
	}, nil
}

func fetchGitHubPrimaryEmail(ctx context.Context, accessToken string, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch github emails: status %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, item := range emails {
		if item.Primary && item.Verified && strings.TrimSpace(item.Email) != "" {
			return item.Email, nil
		}
	}
	for _, item := range emails {
		if item.Verified && strings.TrimSpace(item.Email) != "" {
			return item.Email, nil
		}
	}
	return "", fmt.Errorf("no verified github email")
}
