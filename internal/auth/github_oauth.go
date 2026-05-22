package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const (
	githubOAuthStateTTL = 10 * time.Minute
)

var (
	ErrGitHubOAuthDisabled       = errors.New("GitHub OAuth is not configured")
	ErrInvalidOAuthState         = errors.New("invalid OAuth state")
	ErrGitHubMissingCode         = errors.New("missing authorization code")
	ErrGitHubOAuthExchangeFailed = errors.New("github oauth exchange failed")
	ErrGitHubProfileFetchFailed  = errors.New("github profile fetch failed")
)

type githubOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	FrontendURL  string
}

type oauthStatePayload struct {
	Nonce string `json:"nonce"`
	Exp   int64  `json:"exp"`
}

type githubUserProfile struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// PendingUserNotifier 在新 GitHub 用户待审核时回调（可选注入）。
type PendingUserNotifier func(ctx context.Context, user *User) error

func githubOAuthConfigFromEnv() githubOAuthConfig {
	redirectURI := strings.TrimSpace(os.Getenv("GITHUB_REDIRECT_URI"))
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/api/v1/auth/github/callback"
	}
	frontendURL := strings.TrimSpace(os.Getenv("ADMIN_FRONTEND_URL"))
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	return githubOAuthConfig{
		ClientID:     strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
		RedirectURI:  redirectURI,
		FrontendURL:  strings.TrimRight(frontendURL, "/"),
	}
}

func (s *Service) WithPendingUserNotifier(fn PendingUserNotifier) {
	s.pendingUserNotifier = fn
}

func (s *Service) GitHubOAuthEnabled() bool {
	cfg := githubOAuthConfigFromEnv()
	return cfg.ClientID != "" && cfg.ClientSecret != ""
}

func (s *Service) githubOAuth2Config(cfg githubOAuthConfig) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint:     github.Endpoint,
	}
}

func (s *Service) GitHubLoginURL() (string, error) {
	if !s.GitHubOAuthEnabled() {
		return "", ErrGitHubOAuthDisabled
	}
	cfg := githubOAuthConfigFromEnv()
	state, err := s.signOAuthState()
	if err != nil {
		return "", err
	}
	return s.githubOAuth2Config(cfg).AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

func (s *Service) HandleGitHubCallback(ctx context.Context, code, state string) (string, error) {
	if !s.GitHubOAuthEnabled() {
		return "", ErrGitHubOAuthDisabled
	}
	if err := s.verifyOAuthState(state); err != nil {
		return "", err
	}
	if strings.TrimSpace(code) == "" {
		return "", ErrGitHubMissingCode
	}

	cfg := githubOAuthConfigFromEnv()
	oauthCfg := s.githubOAuth2Config(cfg)
	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrGitHubOAuthExchangeFailed, err)
	}

	profile, err := s.fetchGitHubProfile(ctx, token.AccessToken)
	if err != nil {
		return "", err
	}

	user, created, err := s.resolveGithubUser(ctx, profile)
	if err != nil {
		return "", err
	}
	if created && s.pendingUserNotifier != nil {
		if notifyErr := s.pendingUserNotifier(ctx, user); notifyErr != nil {
			// 通知失败不阻断登录
		}
	}

	jwtToken, err := s.tokenForUser(user)
	if err != nil {
		return "", err
	}
	redirectURL := fmt.Sprintf("%s/login/github/callback#token=%s", cfg.FrontendURL, url.QueryEscape(jwtToken))
	return redirectURL, nil
}

func (s *Service) FrontendLoginURL(query string) string {
	cfg := githubOAuthConfigFromEnv()
	if query == "" {
		return cfg.FrontendURL + "/login"
	}
	return cfg.FrontendURL + "/login?" + query
}

// GitHubCallbackErrorQuery maps callback errors to stable login-page query codes.
func GitHubCallbackErrorQuery(err error) string {
	switch {
	case errors.Is(err, ErrGitHubOAuthDisabled):
		return "oauth_disabled"
	case errors.Is(err, ErrInvalidOAuthState):
		return "invalid_state"
	case errors.Is(err, ErrGitHubMissingCode):
		return "missing_code"
	case errors.Is(err, ErrGitHubOAuthExchangeFailed):
		return "exchange_failed"
	case errors.Is(err, ErrGitHubProfileFetchFailed):
		return "profile_failed"
	default:
		return "oauth_failed"
	}
}

func (s *Service) signOAuthState() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	payload := oauthStatePayload{
		Nonce: hex.EncodeToString(nonce),
		Exp:   time.Now().Add(githubOAuthStateTTL).Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	sig := signPayload(s.secret, encoded)
	return encoded + "." + sig, nil
}

func (s *Service) verifyOAuthState(state string) error {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return ErrInvalidOAuthState
	}
	expected := signPayload(s.secret, parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return ErrInvalidOAuthState
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ErrInvalidOAuthState
	}
	var payload oauthStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ErrInvalidOAuthState
	}
	if payload.Exp < time.Now().Unix() {
		return ErrInvalidOAuthState
	}
	if payload.Nonce == "" {
		return ErrInvalidOAuthState
	}
	return nil
}

func (s *Service) fetchGitHubProfile(ctx context.Context, accessToken string) (*githubUserProfile, error) {
	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGitHubProfileFetchFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status %d: %s", ErrGitHubProfileFetchFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var profile githubUserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGitHubProfileFetchFailed, err)
	}
	if profile.ID == 0 || strings.TrimSpace(profile.Login) == "" {
		return nil, fmt.Errorf("%w: invalid profile", ErrGitHubProfileFetchFailed)
	}
	if strings.TrimSpace(profile.Email) == "" {
		email, err := s.fetchGitHubPrimaryEmail(ctx, accessToken, client)
		if err == nil {
			profile.Email = email
		}
	}
	return &profile, nil
}

func (s *Service) fetchGitHubPrimaryEmail(ctx context.Context, accessToken string, client *http.Client) (string, error) {
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
	return "", errors.New("no verified github email")
}

func (s *Service) resolveGithubUser(ctx context.Context, profile *githubUserProfile) (*User, bool, error) {
	existing, err := s.repo.GetUserByGithubID(ctx, profile.ID)
	if err == nil {
		user, err := s.repo.UpdateGithubUserProfile(ctx, existing.ID, githubDisplayName(profile), profile.Email, s.downloadGitHubAvatar(ctx, profile.AvatarURL))
		if err != nil {
			return nil, false, err
		}
		return user, false, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	username, err := s.repo.AllocateGithubUsername(ctx, githubUsername(profile.Login))
	if err != nil {
		return nil, false, err
	}
	displayName := githubDisplayName(profile)
	var avatar []byte
	if jpeg := s.downloadGitHubAvatar(ctx, profile.AvatarURL); len(jpeg) > 0 {
		avatar = jpeg
	}
	active := false
	user, err := s.repo.CreateGithubUser(ctx, GithubUserInput{
		Username:    username,
		DisplayName: displayName,
		Email:       profile.Email,
		GithubID:    profile.ID,
		Active:      active,
		AvatarJPEG:  avatar,
	})
	if err != nil {
		if isUniqueGithubIDViolation(err) {
			existing, lookupErr := s.repo.GetUserByGithubID(ctx, profile.ID)
			if lookupErr == nil {
				user, err := s.repo.UpdateGithubUserProfile(ctx, existing.ID, githubDisplayName(profile), profile.Email, s.downloadGitHubAvatar(ctx, profile.AvatarURL))
				if err != nil {
					return nil, false, err
				}
				return user, false, nil
			}
		}
		return nil, false, err
	}
	return user, true, nil
}

func githubUsername(login string) string {
	login = strings.ToLower(strings.TrimSpace(login))
	login = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, login)
	if login == "" {
		login = "user"
	}
	return "gh_" + login
}

func githubDisplayName(profile *githubUserProfile) string {
	if name := strings.TrimSpace(profile.Name); name != "" {
		return name
	}
	return profile.Login
}

func (s *Service) downloadGitHubAvatar(ctx context.Context, avatarURL string) []byte {
	avatarURL = strings.TrimSpace(avatarURL)
	if avatarURL == "" {
		return nil
	}
	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, avatarURL, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRawImageBytes))
	if err != nil {
		return nil
	}
	jpeg, err := ProcessAvatarBytes(raw)
	if err != nil {
		return nil
	}
	return jpeg
}

func (s *Service) tokenForUser(user *User) (string, error) {
	return signJWT(s.secret, user, s.ttl)
}

func isUniqueGithubIDViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return strings.Contains(pgErr.ConstraintName, "github_id")
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") && strings.Contains(msg, "github_id")
}

// OAuthStateForTest 仅供单元测试构造合法 state。
func (s *Service) OAuthStateForTest(exp time.Time) (string, error) {
	payload := oauthStatePayload{
		Nonce: "test-nonce",
		Exp:   exp.Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return encoded + "." + signPayload(s.secret, encoded), nil
}
