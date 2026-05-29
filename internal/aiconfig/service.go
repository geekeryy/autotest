package aiconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const clearSecretSentinel = "__clear__"

var (
	ErrInvalidMaxToolHops       = errors.New("maxToolHops 须在 1–20 之间")
	ErrInvalidMaxRounds         = errors.New("scenarioGenDefaultMaxRounds 须在 1–10 之间")
	ErrInvalidConfidence        = errors.New("routerConfidenceThreshold 须在 0.1–1.0 之间")
	ErrInvalidToolRoutingMode   = errors.New("toolRoutingMode 无效")
)

// Service manages AI assistant platform settings.
type Service struct {
	repo  *Repository
	store *Store
}

// NewService constructs a Service.
func NewService(repo *Repository, store *Store) *Service {
	if store == nil {
		store = globalStore
	}
	return &Service{repo: repo, store: store}
}

// Load reads persisted settings into the store. Missing row is not an error.
func (s *Service) Load(ctx context.Context) error {
	if s.repo == nil {
		return nil
	}
	raw, updatedAt, err := s.repo.Get(ctx)
	if err != nil {
		return err
	}
	if raw == nil {
		s.store.LoadFromStored(storedConfig{})
		return nil
	}
	s.store.LoadFromStored(*raw)
	if !updatedAt.IsZero() {
		_ = updatedAt
	}
	return nil
}

// GetView returns settings plus field metadata for the admin UI.
func (s *Service) GetView(ctx context.Context) (*SettingsView, error) {
	if err := s.Load(ctx); err != nil {
		return nil, err
	}
	cfg := s.store.Get()
	view := &SettingsView{
		Settings: cfg.ToPayload(),
		Fields:   FieldDefinitions(),
	}
	if s.repo != nil {
		if _, updatedAt, err := s.repo.Get(ctx); err == nil && !updatedAt.IsZero() {
			view.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		}
	}
	return view, nil
}

// Update applies partial changes and persists them.
func (s *Service) Update(ctx context.Context, input UpdateInput) (*SettingsView, error) {
	if err := s.Load(ctx); err != nil {
		return nil, err
	}
	current := s.store.Get()
	next := current

	if input.MaxToolHops != nil {
		if *input.MaxToolHops < 1 || *input.MaxToolHops > 20 {
			return nil, ErrInvalidMaxToolHops
		}
		next.MaxToolHops = *input.MaxToolHops
	}
	if input.ToolRoutingMode != nil {
		trimmed := strings.ToLower(strings.TrimSpace(*input.ToolRoutingMode))
		if !validRoutingMode(trimmed) {
			return nil, ErrInvalidToolRoutingMode
		}
		next.ToolRoutingMode = normalizeRoutingMode(trimmed)
	}
	if input.ScenarioAutorunEnabled != nil {
		next.ScenarioAutorunEnabled = *input.ScenarioAutorunEnabled
	}
	if input.ScenarioGenDefaultMaxRounds != nil {
		if *input.ScenarioGenDefaultMaxRounds < 1 || *input.ScenarioGenDefaultMaxRounds > 10 {
			return nil, ErrInvalidMaxRounds
		}
		next.ScenarioGenDefaultMaxRounds = *input.ScenarioGenDefaultMaxRounds
	}
	if input.RouterConfidenceThreshold != nil {
		if *input.RouterConfidenceThreshold < 0.1 || *input.RouterConfidenceThreshold > 1.0 {
			return nil, ErrInvalidConfidence
		}
		next.RouterConfidenceThreshold = *input.RouterConfidenceThreshold
	}
	if input.ToolEmbedBaseURL != nil {
		next.ToolEmbedBaseURL = strings.TrimSpace(*input.ToolEmbedBaseURL)
	}
	if input.ToolEmbedAPIKey != nil {
		v := strings.TrimSpace(*input.ToolEmbedAPIKey)
		switch v {
		case "":
			// keep
		case clearSecretSentinel:
			next.ToolEmbedAPIKey = ""
		default:
			next.ToolEmbedAPIKey = v
		}
	}
	if input.ToolEmbedModel != nil {
		v := strings.TrimSpace(*input.ToolEmbedModel)
		if v != "" {
			next.ToolEmbedModel = v
		}
	}

	if s.repo == nil {
		s.store.Set(next)
		return s.GetView(ctx)
	}
	if err := s.repo.Save(ctx, next.ToStored()); err != nil {
		return nil, fmt.Errorf("save ai assistant settings: %w", err)
	}
	s.store.Set(next)
	return s.GetView(ctx)
}
