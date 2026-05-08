package aiprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"autotest/internal/aiprovider/client"

	"github.com/google/uuid"
)

// Service applies validation, defaulting and orchestrates client-side AI calls.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SupportedTypes returns metadata about each provider type for the frontend create form.
func (s *Service) SupportedTypes() []ProviderTypeMeta {
	return []ProviderTypeMeta{
		{
			Type:           ProviderTypeDeepSeek,
			Label:          "DeepSeek",
			DefaultBaseURL: "https://api.deepseek.com/v1",
			DefaultModel:   "deepseek-chat",
			Models:         []string{"deepseek-chat", "deepseek-reasoner"},
			APIKeyRequired: true,
			Notes:          "OpenAI 兼容协议，使用官方 https://api.deepseek.com/v1。",
		},
		{
			Type:           ProviderTypeXiaomi,
			Label:          "Xiaomi（小米大模型）",
			DefaultBaseURL: "",
			DefaultModel:   "",
			APIKeyRequired: true,
			Notes:          "小米大模型网关，需要在企业内部确认 base URL（OpenAI 兼容）。",
		},
		{
			Type:           ProviderTypeOpenAI,
			Label:          "OpenAI",
			DefaultBaseURL: "https://api.openai.com/v1",
			DefaultModel:   "gpt-4o-mini",
			Models:         []string{"gpt-4o-mini", "gpt-4o", "gpt-4.1-mini"},
			APIKeyRequired: true,
			Notes:          "可在 extraConfig 中通过 organization 或 headers 字段附加自定义请求头。",
		},
		{
			Type:           ProviderTypeAnthropic,
			Label:          "Anthropic",
			DefaultBaseURL: "https://api.anthropic.com/v1",
			DefaultModel:   "claude-3-5-sonnet-latest",
			Models:         []string{"claude-3-5-sonnet-latest", "claude-3-5-haiku-latest", "claude-3-opus-latest"},
			APIKeyRequired: true,
			Notes:          "默认使用 anthropic-version: 2023-06-01，可在 extraConfig.anthropicVersion 覆盖。",
		},
		{
			Type:           ProviderTypeKimi,
			Label:          "Kimi（月之暗面）",
			DefaultBaseURL: "https://api.moonshot.cn/v1",
			DefaultModel:   "moonshot-v1-8k",
			Models:         []string{"moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"},
			APIKeyRequired: true,
			Notes:          "OpenAI 兼容协议，访问 Moonshot AI。",
		},
		{
			Type:           ProviderTypeOllama,
			Label:          "Ollama（本地）",
			DefaultBaseURL: "http://localhost:11434/v1",
			DefaultModel:   "llama3.1:8b",
			APIKeyRequired: false,
			Notes:          "本地部署的 Ollama，需以 OpenAI 兼容端点访问（路径 /v1/chat/completions）。",
		},
	}
}

// List returns all providers visible to the project (with masked API keys).
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]Provider, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	rows, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]Provider, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r.toAPI())
	}
	return out, nil
}

// Create validates and inserts a new provider record.
func (s *Service) Create(ctx context.Context, projectID uuid.UUID, input CreateInput) (*Provider, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if err := normalizeCreate(&input); err != nil {
		return nil, err
	}
	row, err := s.repo.Create(ctx, projectID, input)
	if err != nil {
		return nil, err
	}
	return row.toAPI(), nil
}

// Update mutates an existing provider record.
func (s *Service) Update(ctx context.Context, projectID, providerID uuid.UUID, input UpdateInput) (*Provider, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if providerID == uuid.Nil {
		return nil, errors.New("providerId is required")
	}
	existing, err := s.repo.Get(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	if err := normalizeUpdate(&input, existing); err != nil {
		return nil, err
	}
	row, err := s.repo.Update(ctx, projectID, providerID, input)
	if err != nil {
		return nil, err
	}
	return row.toAPI(), nil
}

// Delete soft-deletes a provider.
func (s *Service) Delete(ctx context.Context, projectID, providerID uuid.UUID) error {
	if projectID == uuid.Nil || providerID == uuid.Nil {
		return errors.New("projectId and providerId are required")
	}
	return s.repo.Delete(ctx, projectID, providerID)
}

// TestConnection sends a minimal probe message and returns a short snippet of the model reply.
func (s *Service) TestConnection(ctx context.Context, projectID, providerID uuid.UUID) (*ChatResponse, error) {
	row, err := s.repo.Get(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, ErrProviderDisabled
	}
	cli, err := s.buildClient(row)
	if err != nil {
		return nil, err
	}
	temperature := 0.0
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	result, err := cli.Chat(timeoutCtx, []client.Message{
		{Role: "system", Content: "你是一个回声助手，仅回答用户的最后一句话。"},
		{Role: "user", Content: "ping"},
	}, client.Options{Temperature: &temperature, MaxTokens: 64})
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}
	return &ChatResponse{
		ProviderID:    row.ID,
		Action:        "test",
		Model:         result.Model,
		Text:          result.Text,
		ElapsedMillis: elapsed,
	}, nil
}

// Chat runs a structured AI request for one of the supported actions.
func (s *Service) Chat(ctx context.Context, projectID uuid.UUID, req ChatRequest) (*ChatResponse, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if req.ProviderID == uuid.Nil {
		return nil, errors.New("providerId is required")
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		action = ActionRaw
	}
	if !validAction(action) {
		return nil, ErrProviderActionInvalid
	}
	if action == ActionGenerateAssertion && strings.TrimSpace(req.Prompt) == "" {
		return nil, ErrAssertionIntentRequired
	}

	row, err := s.repo.Get(ctx, projectID, req.ProviderID)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, ErrProviderDisabled
	}

	cli, err := s.buildClient(row)
	if err != nil {
		return nil, err
	}

	messages, jsonOnly := buildMessages(action, req.Prompt, req.Context, req.SystemPromptOverride)
	opts := client.Options{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		JSONOnly:    jsonOnly,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	start := time.Now()
	result, err := cli.Chat(timeoutCtx, messages, opts)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}

	resp := &ChatResponse{
		ProviderID:    row.ID,
		Action:        action,
		Model:         result.Model,
		Text:          result.Text,
		ElapsedMillis: elapsed,
	}
	if jsonOnly || action == ActionGenerateParams || action == ActionGenerateCaseData {
		parsed, warning := extractParsedJSON(result.Text)
		resp.Parsed = parsed
		resp.ParseWarnings = warning
	}
	return resp, nil
}

func (s *Service) buildClient(row *providerRow) (client.Client, error) {
	if row == nil {
		return nil, ErrProviderNotFound
	}
	extra, err := client.SpecFromExtra(row.ExtraConfig)
	if err != nil {
		return nil, err
	}
	spec := client.Spec{
		Type:         row.ProviderType,
		BaseURL:      row.BaseURL,
		APIKey:       row.APIKey,
		DefaultModel: row.DefaultModel,
		ExtraConfig:  extra,
	}
	if row.ProviderType != ProviderTypeOllama && row.APIKey == "" {
		return nil, ErrProviderEmptyKey
	}
	return client.New(spec)
}

func normalizeCreate(input *CreateInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.ProviderType = strings.ToLower(strings.TrimSpace(input.ProviderType))
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.DefaultModel = strings.TrimSpace(input.DefaultModel)
	input.APIKey = strings.TrimSpace(input.APIKey)

	if input.Name == "" {
		return errors.New("name is required")
	}
	if !validProviderType(input.ProviderType) {
		return ErrProviderTypeInvalid
	}
	if input.BaseURL == "" {
		return errors.New("baseUrl is required")
	}
	if input.ProviderType != ProviderTypeOllama && input.APIKey == "" {
		return ErrProviderEmptyKey
	}
	if err := validateExtra(input.ExtraConfig); err != nil {
		return err
	}
	return nil
}

func normalizeUpdate(input *UpdateInput, existing *providerRow) error {
	input.Name = strings.TrimSpace(input.Name)
	input.ProviderType = strings.ToLower(strings.TrimSpace(input.ProviderType))
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.DefaultModel = strings.TrimSpace(input.DefaultModel)

	if input.Name == "" {
		return errors.New("name is required")
	}
	if !validProviderType(input.ProviderType) {
		return ErrProviderTypeInvalid
	}
	if input.BaseURL == "" {
		return errors.New("baseUrl is required")
	}

	effectiveKey := existing.APIKey
	if input.APIKey != nil {
		trimmed := strings.TrimSpace(*input.APIKey)
		input.APIKey = &trimmed
		effectiveKey = trimmed
	}
	if input.ProviderType != ProviderTypeOllama && effectiveKey == "" {
		return ErrProviderEmptyKey
	}
	if err := validateExtra(input.ExtraConfig); err != nil {
		return err
	}
	return nil
}

func validProviderType(t string) bool {
	switch t {
	case ProviderTypeDeepSeek, ProviderTypeXiaomi, ProviderTypeOpenAI,
		ProviderTypeAnthropic, ProviderTypeKimi, ProviderTypeOllama:
		return true
	default:
		return false
	}
}

func validAction(action string) bool {
	switch action {
	case ActionGenerateParams, ActionGenerateAssertion, ActionGenerateCaseData, ActionRaw:
		return true
	default:
		return false
	}
}

func validateExtra(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("extraConfig must be a JSON object: %w", err)
	}
	return nil
}
