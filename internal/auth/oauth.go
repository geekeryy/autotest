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
	"strconv"
	"strings"
	"time"

	"autotest/internal/authprovider"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
)

const oauthStateTTL = 10 * time.Minute

var (
	ErrOAuthDisabled           = errors.New("OAuth is not configured")
	ErrInvalidOAuthState       = errors.New("invalid OAuth state")
	ErrOAuthMissingCode        = errors.New("missing authorization code")
	ErrOAuthExchangeFailed     = errors.New("oauth exchange failed")
	ErrOAuthProfileFetchFailed = errors.New("oauth profile fetch failed")
	ErrRegistrationDisabled    = errors.New("oauth registration disabled")

	// Legacy GitHub error aliases for tests and backward compatibility.
	ErrGitHubOAuthDisabled       = ErrOAuthDisabled
	ErrGitHubMissingCode         = ErrOAuthMissingCode
	ErrGitHubOAuthExchangeFailed = ErrOAuthExchangeFailed
	ErrGitHubProfileFetchFailed  = ErrOAuthProfileFetchFailed
)

type oauthStatePayload struct {
	Nonce       string `json:"nonce"`
	Exp         int64  `json:"exp"`
	FrontendURL string `json:"frontendUrl,omitempty"`
}

// PendingUserNotifier 在新 OAuth 用户待审核时回调（可选注入）。
type PendingUserNotifier func(ctx context.Context, user *User) error

type authProviderResolver interface {
	ResolveProvider(ctx context.Context, slug string) (*authprovider.ResolvedProvider, error)
	ResolveProviderForCallback(ctx context.Context, slug string) (*authprovider.ResolvedProvider, error)
	ListLoginProviders(ctx context.Context, apiOrigin string) ([]authprovider.LoginProviderItem, error)
	GitHubOAuthConfigured(ctx context.Context) (bool, error)
	FirstGitHubProviderID(ctx context.Context) (string, error)
}

type oauthRegistry interface {
	Get(providerType string) (authprovider.OAuthAdapter, error)
}

func (s *Service) WithAuthProvider(resolver authProviderResolver, registry oauthRegistry) {
	s.authProviders = resolver
	s.oauthRegistry = registry
}

func (s *Service) WithPendingUserNotifier(fn PendingUserNotifier) {
	s.pendingUserNotifier = fn
}

func (s *Service) FrontendLoginURL(origin, query string) string {
	return frontendLoginURL(origin, query)
}

func (s *Service) ListLoginProviders(ctx context.Context, apiOrigin string) ([]authprovider.LoginProviderItem, error) {
	if s.authProviders == nil {
		return []authprovider.LoginProviderItem{}, nil
	}
	return s.authProviders.ListLoginProviders(ctx, apiOrigin)
}

func (s *Service) OAuthLoginURL(ctx context.Context, slug, frontendURL string) (string, error) {
	if s.oauthRegistry == nil {
		return "", ErrOAuthDisabled
	}
	resolved, err := s.resolveOAuthProvider(ctx, slug, true)
	if err != nil {
		return "", err
	}
	origin, err := s.validateFrontendURLForProvider(frontendURL, resolved.TrustedFrontendOrigins)
	if err != nil {
		return "", err
	}
	if err := s.validateOAuthCredentials(resolved); err != nil {
		return "", err
	}
	adapter, err := s.oauthRegistry.Get(resolved.ProviderType)
	if err != nil {
		return "", err
	}
	oauthCfg, err := s.buildOAuth2Config(resolved, adapter)
	if err != nil {
		return "", err
	}
	state, err := s.signOAuthState(origin)
	if err != nil {
		return "", err
	}
	return oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

func (s *Service) HandleOAuthCallback(ctx context.Context, slug, code, state string, meta RequestMeta) (redirectURL string, frontendURL string, err error) {
	method := oauthLoginMethod(slug)
	if s.oauthRegistry == nil {
		s.auditLoginFailure(ctx, meta, "", method, nil, "oauth_disabled", "")
		return "", "", ErrOAuthDisabled
	}
	payload, err := s.verifyOAuthState(state)
	if err != nil {
		s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(err), "")
		return "", "", err
	}
	frontendURL = strings.TrimSpace(payload.FrontendURL)
	if strings.TrimSpace(code) == "" {
		s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(ErrOAuthMissingCode), "")
		return "", frontendURL, ErrOAuthMissingCode
	}

	resolved, err := s.resolveOAuthProvider(ctx, slug, false)
	if err != nil {
		s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(err), "")
		return "", frontendURL, err
	}
	if frontendURL != "" {
		if origin, validateErr := s.validateFrontendURLForProvider(frontendURL, resolved.TrustedFrontendOrigins); validateErr != nil {
			s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(validateErr), "")
			return "", "", validateErr
		} else {
			frontendURL = origin
		}
	}
	if err := s.validateOAuthCredentials(resolved); err != nil {
		s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(err), "")
		return "", frontendURL, err
	}

	adapter, err := s.oauthRegistry.Get(resolved.ProviderType)
	if err != nil {
		s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(err), "")
		return "", frontendURL, err
	}
	oauthCfg, err := s.buildOAuth2Config(resolved, adapter)
	if err != nil {
		s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(err), "")
		return "", frontendURL, err
	}

	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(ErrOAuthExchangeFailed), "")
		return "", frontendURL, fmt.Errorf("%w: %w", ErrOAuthExchangeFailed, err)
	}

	profile, err := adapter.FetchProfile(ctx, *resolved, token)
	if err != nil {
		s.auditLoginFailure(ctx, meta, "", method, nil, OAuthCallbackErrorQuery(ErrOAuthProfileFetchFailed), "")
		return "", frontendURL, fmt.Errorf("%w: %w", ErrOAuthProfileFetchFailed, err)
	}

	user, created, err := s.resolveExternalUser(ctx, resolved.ProviderType, profile, resolved.DefaultActive, resolved.AutoRegister)
	if err != nil {
		s.auditLoginFailure(ctx, meta, profile.Login, method, nil, OAuthCallbackErrorQuery(err), "")
		return "", frontendURL, err
	}
	if created && !resolved.DefaultActive && s.pendingUserNotifier != nil {
		_ = s.pendingUserNotifier(ctx, user)
	}

	jwtToken, err := s.tokenForUser(user)
	if err != nil {
		s.auditLoginFailure(ctx, meta, user.Username, method, &user.ID, "token_issue_failed", "")
		return "", frontendURL, err
	}
	if frontendURL == "" {
		s.auditLoginFailure(ctx, meta, user.Username, method, &user.ID, OAuthCallbackErrorQuery(ErrInvalidFrontendURL), "")
		return "", "", ErrInvalidFrontendURL
	}
	s.auditLoginSuccess(ctx, meta, user.Username, method, &user.ID)
	return oauthSuccessRedirect(frontendURL, jwtToken), frontendURL, nil
}

func (s *Service) resolveOAuthProvider(ctx context.Context, slug string, requireEnabled bool) (*authprovider.ResolvedProvider, error) {
	if s.authProviders == nil {
		return nil, ErrOAuthDisabled
	}
	if requireEnabled {
		return s.authProviders.ResolveProvider(ctx, slug)
	}
	return s.authProviders.ResolveProviderForCallback(ctx, slug)
}

func (s *Service) GitHubOAuthConfigured(ctx context.Context) bool {
	if s.authProviders == nil {
		return false
	}
	ok, err := s.authProviders.GitHubOAuthConfigured(ctx)
	return err == nil && ok
}

func (s *Service) LegacyGitHubLoginURL(ctx context.Context, frontendURL string) (string, error) {
	if s.authProviders == nil {
		return "", ErrOAuthDisabled
	}
	providerSlug, err := s.authProviders.FirstGitHubProviderID(ctx)
	if err != nil {
		return "", ErrOAuthDisabled
	}
	return s.OAuthLoginURL(ctx, providerSlug, frontendURL)
}

func (s *Service) GitHubOAuthEnabled() bool {
	return s.GitHubOAuthConfigured(context.Background())
}

func (s *Service) GitHubLoginURL(ctx context.Context, frontendURL string) (string, error) {
	return s.LegacyGitHubLoginURL(ctx, frontendURL)
}

func (s *Service) HandleGitHubCallback(ctx context.Context, code, state string, meta RequestMeta) (string, string, error) {
	if s.authProviders == nil {
		return "", "", ErrOAuthDisabled
	}
	providerSlug, err := s.authProviders.FirstGitHubProviderID(ctx)
	if err != nil {
		return "", "", ErrOAuthDisabled
	}
	return s.HandleOAuthCallback(ctx, providerSlug, code, state, meta)
}

func (s *Service) validateOAuthCredentials(resolved *authprovider.ResolvedProvider) error {
	if strings.TrimSpace(resolved.ClientID) == "" || strings.TrimSpace(resolved.ClientSecret) == "" {
		return ErrOAuthDisabled
	}
	return nil
}

func (s *Service) buildOAuth2Config(resolved *authprovider.ResolvedProvider, adapter authprovider.OAuthAdapter) (*oauth2.Config, error) {
	scopes := resolved.Scopes
	if len(scopes) == 0 {
		scopes = adapter.DefaultScopes()
	}
	cfg := *resolved
	cfg.Scopes = scopes
	return adapter.OAuth2Config(cfg)
}

// OAuthCallbackErrorQuery maps callback errors to stable login-page query codes.
func OAuthCallbackErrorQuery(err error) string {
	switch {
	case errors.Is(err, ErrOAuthDisabled):
		return "oauth_disabled"
	case errors.Is(err, ErrInvalidOAuthState):
		return "invalid_state"
	case errors.Is(err, ErrOAuthMissingCode):
		return "missing_code"
	case errors.Is(err, ErrOAuthExchangeFailed):
		return "exchange_failed"
	case errors.Is(err, ErrOAuthProfileFetchFailed):
		return "profile_failed"
	case errors.Is(err, ErrRegistrationDisabled):
		return "registration_disabled"
	case errors.Is(err, ErrInvalidFrontendURL):
		return "invalid_frontend_url"
	default:
		return "oauth_failed"
	}
}

// GitHubCallbackErrorQuery is kept for backward compatibility.
func GitHubCallbackErrorQuery(err error) string {
	return OAuthCallbackErrorQuery(err)
}

func (s *Service) signOAuthState(frontendURL string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	payload := oauthStatePayload{
		Nonce:       hex.EncodeToString(nonce),
		Exp:         time.Now().Add(oauthStateTTL).Unix(),
		FrontendURL: strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	sig := signPayload(s.secret, encoded)
	return encoded + "." + sig, nil
}

func (s *Service) verifyOAuthState(state string) (oauthStatePayload, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return oauthStatePayload{}, ErrInvalidOAuthState
	}
	expected := signPayload(s.secret, parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return oauthStatePayload{}, ErrInvalidOAuthState
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oauthStatePayload{}, ErrInvalidOAuthState
	}
	var payload oauthStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return oauthStatePayload{}, ErrInvalidOAuthState
	}
	if payload.Exp < time.Now().Unix() {
		return oauthStatePayload{}, ErrInvalidOAuthState
	}
	if payload.Nonce == "" {
		return oauthStatePayload{}, ErrInvalidOAuthState
	}
	return payload, nil
}

func (s *Service) resolveExternalUser(ctx context.Context, providerType string, profile *authprovider.ExternalProfile, defaultActive, autoRegister bool) (*User, bool, error) {
	existing, err := s.repo.GetUserByExternalIdentity(ctx, providerType, profile.ExternalID)
	if err == nil {
		avatar := s.downloadAvatar(ctx, profile.AvatarURL)
		user, err := s.repo.UpdateOAuthUserProfile(ctx, existing.ID, profile.DisplayName, profile.Email, avatar)
		if err != nil {
			return nil, false, err
		}
		return user, false, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	if !autoRegister {
		return nil, false, ErrRegistrationDisabled
	}

	prefix := oauthUsernamePrefix(providerType)
	base := sanitizeOAuthLogin(profile.Login)
	username, err := s.repo.AllocateOAuthUsername(ctx, prefix, base)
	if err != nil {
		return nil, false, err
	}

	var avatar []byte
	if jpeg := s.downloadAvatar(ctx, profile.AvatarURL); len(jpeg) > 0 {
		avatar = jpeg
	}

	var githubID *int64
	if providerType == authprovider.ProviderTypeGithub {
		if id, parseErr := strconv.ParseInt(profile.ExternalID, 10, 64); parseErr == nil {
			githubID = &id
		}
	}

	user, err := s.repo.CreateOAuthUser(ctx, OAuthUserInput{
		Username:     username,
		DisplayName:  profile.DisplayName,
		Email:        profile.Email,
		AuthProvider: providerType,
		GithubID:     githubID,
		Active:       defaultActive,
		AvatarJPEG:   avatar,
		ProviderType: providerType,
		ExternalID:   profile.ExternalID,
	})
	if err != nil {
		if isUniqueExternalIdentityViolation(err) || isUniqueGithubIDViolation(err) {
			existing, lookupErr := s.repo.GetUserByExternalIdentity(ctx, providerType, profile.ExternalID)
			if lookupErr == nil {
				avatar := s.downloadAvatar(ctx, profile.AvatarURL)
				user, err := s.repo.UpdateOAuthUserProfile(ctx, existing.ID, profile.DisplayName, profile.Email, avatar)
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

func (s *Service) resolveGithubUser(ctx context.Context, profile *githubUserProfile) (*User, bool, error) {
	ext := &authprovider.ExternalProfile{
		ExternalID:  fmt.Sprintf("%d", profile.ID),
		Login:       profile.Login,
		DisplayName: githubDisplayName(profile),
		Email:       profile.Email,
		AvatarURL:   profile.AvatarURL,
	}
	return s.resolveExternalUser(ctx, authprovider.ProviderTypeGithub, ext, false, true)
}

func oauthUsernamePrefix(providerType string) string {
	switch providerType {
	case authprovider.ProviderTypeGitlab:
		return "gl_"
	case authprovider.ProviderTypeGoogle:
		return "gg_"
	default:
		return "gh_"
	}
}

func sanitizeOAuthLogin(login string) string {
	login = strings.ToLower(strings.TrimSpace(login))
	login = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, login)
	login = strings.Trim(login, "._-")
	if login == "" {
		return "user"
	}
	return login
}

func githubUsername(login string) string {
	return "gh_" + sanitizeOAuthLogin(login)
}

func githubDisplayName(profile *githubUserProfile) string {
	if name := strings.TrimSpace(profile.Name); name != "" {
		return name
	}
	return profile.Login
}

func (s *Service) downloadAvatar(ctx context.Context, avatarURL string) []byte {
	return s.downloadGitHubAvatar(ctx, avatarURL)
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

func isUniqueExternalIdentityViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return strings.Contains(pgErr.ConstraintName, "user_external_identities")
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") && strings.Contains(msg, "user_external_identities")
}

// OAuthStateForTest 仅供单元测试构造合法 state。
func (s *Service) OAuthStateForTest(exp time.Time, frontendURL string) (string, error) {
	payload := oauthStatePayload{
		Nonce:       "test-nonce",
		Exp:         exp.Unix(),
		FrontendURL: strings.TrimRight(strings.TrimSpace(frontendURL), "/"),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return encoded + "." + signPayload(s.secret, encoded), nil
}

// githubUserProfile is kept for existing tests.
type githubUserProfile struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}
