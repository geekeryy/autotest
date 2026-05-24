package authprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
)

// Service manages auth provider CRUD, resolution, and validation.
type Service struct {
	repo     *Repository
	registry *Registry
}

// NewService constructs a Service.
func NewService(repo *Repository, registry *Registry) *Service {
	return &Service{
		repo:     repo,
		registry: registry,
	}
}

// SupportedTypes returns metadata for the admin create form.
func (s *Service) SupportedTypes() []ProviderTypeMeta {
	return s.registry.SupportedTypes()
}

// ResolveProvider loads an enabled provider for login redirect.
func (s *Service) ResolveProvider(ctx context.Context, slug string) (*ResolvedProvider, error) {
	return s.resolveProvider(ctx, slug, true)
}

// ResolveProviderForCallback loads a provider for OAuth callback handling.
func (s *Service) ResolveProviderForCallback(ctx context.Context, slug string) (*ResolvedProvider, error) {
	return s.resolveProvider(ctx, slug, false)
}

func (s *Service) resolveProvider(ctx context.Context, slug string, requireEnabled bool) (*ResolvedProvider, error) {
	slug = normalizeSlug(slug)
	if slug == "" {
		return nil, ErrProviderNotFound
	}
	row, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if requireEnabled && !row.Enabled {
		return nil, ErrProviderDisabled
	}
	cfg := rowToResolved(row)
	return &cfg, nil
}

// ListLoginProviders returns enabled providers whose callback URL matches apiOrigin.
func (s *Service) ListLoginProviders(ctx context.Context, apiOrigin string) ([]LoginProvider, error) {
	rows, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LoginProvider, 0, len(rows))
	for _, row := range rows {
		if !CallbackURLMatchesOrigin(row.CallbackURL, apiOrigin) {
			continue
		}
		out = append(out, LoginProvider{
			ID:           row.ID.String(),
			Slug:         row.Slug,
			Name:         row.Name,
			ProviderType: row.ProviderType,
			CallbackURL:  row.CallbackURL,
			Source:       SourceDB,
		})
	}
	if out == nil {
		return []LoginProvider{}, nil
	}
	return out, nil
}

// GitHubOAuthConfigured reports whether a DB-enabled GitHub provider exists.
func (s *Service) GitHubOAuthConfigured(ctx context.Context) (bool, error) {
	return s.repo.HasEnabledProviderType(ctx, TypeGitHub)
}

// FirstGitHubProviderID returns the slug of the first enabled GitHub provider.
func (s *Service) FirstGitHubProviderID(ctx context.Context) (string, error) {
	row, err := s.repo.FirstEnabledByType(ctx, TypeGitHub)
	if err != nil {
		return "", err
	}
	return row.Slug, nil
}

// List returns all providers with masked secrets.
func (s *Service) List(ctx context.Context) ([]Provider, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Provider, 0, len(rows))
	for _, row := range rows {
		api := row.toAPI()
		out = append(out, *api)
	}
	return out, nil
}

func (s *Service) validateSlugUnique(ctx context.Context, slug string, excludeID *uuid.UUID) error {
	taken, err := s.repo.SlugTaken(ctx, slug, excludeID)
	if err != nil {
		return err
	}
	if taken {
		return ErrSlugDuplicate
	}
	return nil
}

// Create inserts a new auth provider.
func (s *Service) Create(ctx context.Context, input CreateInput) (*Provider, error) {
	input.ProviderType = strings.TrimSpace(input.ProviderType)
	input.Slug = normalizeSlug(input.Slug)
	input.Name = strings.TrimSpace(input.Name)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.ClientSecret = strings.TrimSpace(input.ClientSecret)
	input.CallbackURL = strings.TrimSpace(input.CallbackURL)
	if input.ProviderType == "" || !isValidProviderType(input.ProviderType) {
		return nil, ErrProviderTypeInvalid
	}
	if _, err := s.registry.Get(input.ProviderType); err != nil {
		return nil, err
	}
	if err := validateSlug(input.Slug); err != nil {
		return nil, err
	}
	if err := validateCallbackURL(input.CallbackURL, input.Slug); err != nil {
		return nil, err
	}
	if err := s.validateSlugUnique(ctx, input.Slug, nil); err != nil {
		return nil, err
	}
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if input.ClientID == "" {
		return nil, fmt.Errorf("client id is required")
	}
	if input.ClientSecret == "" {
		return nil, ErrEmptyClientSecret
	}
	trustedOrigins, err := normalizeTrustedFrontendOrigins(input.TrustedFrontendOrigins)
	if err != nil {
		return nil, err
	}
	input.TrustedFrontendOrigins = trustedOrigins
	if len(input.ExtraConfig) == 0 {
		input.ExtraConfig = json.RawMessage("{}")
	}

	row, err := s.repo.Create(ctx, input)
	if err != nil {
		if isSlugUniqueViolation(err) {
			return nil, ErrSlugDuplicate
		}
		return nil, err
	}
	api := row.toAPI()
	return api, nil
}

// Update mutates an existing auth provider.
func (s *Service) Update(ctx context.Context, providerID uuid.UUID, input UpdateInput) (*Provider, error) {
	input.Slug = normalizeSlug(input.Slug)
	input.Name = strings.TrimSpace(input.Name)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.CallbackURL = strings.TrimSpace(input.CallbackURL)
	if err := validateSlug(input.Slug); err != nil {
		return nil, err
	}
	if err := validateCallbackURL(input.CallbackURL, input.Slug); err != nil {
		return nil, err
	}
	if err := s.validateSlugUnique(ctx, input.Slug, &providerID); err != nil {
		return nil, err
	}
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if input.ClientID == "" {
		return nil, fmt.Errorf("client id is required")
	}
	trustedOrigins, err := normalizeTrustedFrontendOrigins(input.TrustedFrontendOrigins)
	if err != nil {
		return nil, err
	}
	input.TrustedFrontendOrigins = trustedOrigins
	if len(input.ExtraConfig) == 0 {
		input.ExtraConfig = json.RawMessage("{}")
	}

	row, err := s.repo.Update(ctx, providerID, input)
	if err != nil {
		if isSlugUniqueViolation(err) {
			return nil, ErrSlugDuplicate
		}
		return nil, err
	}
	api := row.toAPI()
	return api, nil
}

// Delete soft-deletes a provider.
func (s *Service) Delete(ctx context.Context, providerID uuid.UUID) error {
	if err := s.repo.Delete(ctx, providerID); err != nil {
		return err
	}
	return nil
}

// Test validates credentials and builds an authorization URL without completing login.
func (s *Service) Test(ctx context.Context, providerID uuid.UUID) (*TestResult, error) {
	row, err := s.repo.GetByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.registry.Get(row.ProviderType)
	if err != nil {
		return nil, err
	}
	cfg := rowToResolved(row)
	oauthCfg, err := adapter.OAuth2Config(cfg)
	if err != nil {
		return &TestResult{OK: false, Message: err.Error()}, nil
	}
	authURL := oauthCfg.AuthCodeURL("test-state", oauth2.AccessTypeOnline)
	return &TestResult{
		OK:      true,
		Message: "配置校验通过，可构造授权 URL",
		AuthURL: authURL,
	}, nil
}

// ResolveByID loads a DB provider into ResolvedProvider (enabled only).
func (s *Service) ResolveByID(ctx context.Context, providerID uuid.UUID) (*ResolvedProvider, error) {
	row, err := s.repo.GetByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, ErrProviderDisabled
	}
	cfg := rowToResolved(row)
	return &cfg, nil
}

func isSlugUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return strings.Contains(pgErr.ConstraintName, "auth_providers_slug")
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") && strings.Contains(msg, "slug")
}
