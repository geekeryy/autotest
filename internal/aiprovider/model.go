package aiprovider

import (
	"encoding/json"
	"errors"
	"time"

	"autotest/internal/aiprovider/client"

	"github.com/google/uuid"
)

// Errors returned by the aiprovider service.
var (
	ErrProviderNotFound      = errors.New("ai provider not found")
	ErrProviderTypeInvalid   = errors.New("ai provider type is invalid")
	ErrProviderDisabled      = errors.New("ai provider is disabled")
	ErrProviderActionInvalid = errors.New("ai action is invalid")
	ErrProviderEmptyKey      = errors.New("ai provider api key is required")
	// ErrAssertionIntentRequired is returned when generate_assertion is called without a user intent (prompt).
	ErrAssertionIntentRequired = errors.New("generate_assertion requires non-empty intent (prompt)")
)

// Supported provider types. The set is closed and validated against the DB CHECK.
const (
	ProviderTypeDeepSeek  = "deepseek"
	ProviderTypeXiaomi    = "xiaomi"
	ProviderTypeOpenAI    = "openai"
	ProviderTypeAnthropic = "anthropic"
	ProviderTypeKimi      = "kimi"
	ProviderTypeOllama    = "ollama"
)

// Supported actions. They map to internal prompt templates.
const (
	ActionGenerateParams     = "generate_params"
	ActionGenerateAssertion  = "generate_assertion"
	ActionGenerateCaseData   = "generate_case_data"
	ActionAnalyzeFailure     = "analyze_failure"
	ActionAnalyzeSpecChanges = "analyze_spec_changes"
	ActionAssistantChat      = "assistant_chat"
	ActionRaw                = "raw"
)

// Provider is the API-facing representation of an AI provider configuration.
// API keys are returned in masked form only; the plaintext never leaves the server.
type Provider struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	ProviderType  string          `json:"providerType"`
	BaseURL       string          `json:"baseUrl"`
	APIKeyMasked  string          `json:"apiKeyMasked"`
	APIKeyPresent bool            `json:"apiKeyPresent"`
	DefaultModel   string                  `json:"defaultModel,omitempty"`
	ModalityModels ProviderModalityModels `json:"modalityModels,omitempty"`
	ExtraConfig    json.RawMessage         `json:"extraConfig,omitempty"`
	Enabled       bool            `json:"enabled"`
	IsDefault     bool            `json:"isDefault"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

// CreateInput is the payload for POST /ai-providers.
type CreateInput struct {
	Name         string          `json:"name"`
	ProviderType string          `json:"providerType"`
	BaseURL      string          `json:"baseUrl"`
	APIKey       string          `json:"apiKey"`
	DefaultModel   string                   `json:"defaultModel"`
	ModalityModels *ProviderModalityModels `json:"modalityModels,omitempty"`
	ExtraConfig    json.RawMessage          `json:"extraConfig"`
	Enabled        *bool                    `json:"enabled"`
	IsDefault      bool                     `json:"isDefault"`
}

// UpdateInput is the payload for PUT /ai-providers/{providerID}.
// APIKey is optional: empty string means "keep previous"; sentinel "__clear__" wipes it.
type UpdateInput struct {
	Name         string          `json:"name"`
	ProviderType string          `json:"providerType"`
	BaseURL      string          `json:"baseUrl"`
	APIKey       *string         `json:"apiKey"`
	DefaultModel   string                   `json:"defaultModel"`
	ModalityModels *ProviderModalityModels `json:"modalityModels,omitempty"`
	ExtraConfig    json.RawMessage          `json:"extraConfig"`
	Enabled        *bool                    `json:"enabled"`
	IsDefault      bool                     `json:"isDefault"`
}

// ChatRequest is the payload for POST /projects/{projectID}/ai/chat.
type ChatRequest struct {
	ProviderID           uuid.UUID       `json:"providerId"`
	Action               string          `json:"action"`
	Prompt               string          `json:"prompt"`
	Context              json.RawMessage `json:"context"`
	Model                string          `json:"model,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	MaxTokens            int             `json:"maxTokens,omitempty"`
	ResponseHint         string          `json:"responseHint,omitempty"`
	SystemPromptOverride string          `json:"-"`
}

// ChatResponse describes the model output and any structured fragment we could parse out.
type ChatResponse struct {
	ProviderID    uuid.UUID       `json:"providerId"`
	Action        string          `json:"action"`
	Model         string          `json:"model"`
	Text          string          `json:"text"`
	Parsed        json.RawMessage `json:"parsed,omitempty"`
	ParseWarnings string          `json:"parseWarnings,omitempty"`
	ElapsedMillis int64           `json:"elapsedMillis"`
}

// ProviderTypeMeta is returned to the frontend so the create form can suggest defaults.
type ProviderTypeMeta struct {
	Type           string   `json:"type"`
	Label          string   `json:"label"`
	DefaultBaseURL string   `json:"defaultBaseUrl"`
	DefaultModel   string   `json:"defaultModel"`
	Models         []string `json:"models,omitempty"` // offline fallback when upstream list fails
	APIKeyRequired bool     `json:"apiKeyRequired"`
	Notes          string   `json:"notes,omitempty"`
}

// ListModelsResult is returned when querying models from the upstream provider.
type ListModelsResult struct {
	Models  []client.ModelInfo `json:"models"`
	Source  string             `json:"source"` // "api" or "fallback"
	Warning string             `json:"warning,omitempty"`
}

// DiscoverModelsInput probes an upstream provider before a record is saved.
type DiscoverModelsInput struct {
	ProviderType string          `json:"providerType"`
	BaseURL      string          `json:"baseUrl"`
	APIKey       string          `json:"apiKey"`
	ProviderID   *uuid.UUID      `json:"providerId,omitempty"`
	ExtraConfig  json.RawMessage `json:"extraConfig,omitempty"`
}

// providerRow is the internal struct used while scanning rows; it carries the plaintext key.
type providerRow struct {
	ID           uuid.UUID
	Name         string
	ProviderType string
	BaseURL      string
	APIKey       string
	DefaultModel string
	ExtraConfig  []byte
	Enabled      bool
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (p providerRow) toAPI() *Provider {
	out := &Provider{
		ID:            p.ID,
		Name:          p.Name,
		ProviderType:  p.ProviderType,
		BaseURL:       p.BaseURL,
		APIKeyMasked:  maskKey(p.APIKey),
		APIKeyPresent: p.APIKey != "",
		DefaultModel:  p.DefaultModel,
		Enabled:       p.Enabled,
		IsDefault:     p.IsDefault,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
	if len(p.ExtraConfig) > 0 {
		out.ExtraConfig = json.RawMessage(p.ExtraConfig)
	}
	out.ModalityModels = providerModalityModels(&p)
	return out
}

func maskKey(key string) string {
	if key == "" {
		return ""
	}
	runes := []rune(key)
	if len(runes) <= 4 {
		return "••••"
	}
	if len(runes) <= 10 {
		return "••••" + string(runes[len(runes)-2:])
	}
	return string(runes[:3]) + "••••" + string(runes[len(runes)-4:])
}
