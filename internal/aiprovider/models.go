package aiprovider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"autotest/internal/aiprovider/client"

	"github.com/google/uuid"
)

const (
	modelsSourceAPI      = "api"
	modelsSourceFallback = "fallback"
)

// ListModels loads model ids from the upstream provider for a saved configuration.
func (s *Service) ListModels(ctx context.Context, projectID, providerID uuid.UUID) (*ListModelsResult, error) {
	if projectID == uuid.Nil || providerID == uuid.Nil {
		return nil, errors.New("projectId and providerId are required")
	}
	row, err := s.repo.Get(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	return s.listModelsForRow(ctx, row)
}

// DiscoverModels lists models using transient credentials (create form) or merges
// a stored API key when providerId is set and apiKey is empty (edit form).
func (s *Service) DiscoverModels(ctx context.Context, projectID uuid.UUID, input DiscoverModelsInput) (*ListModelsResult, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	input.ProviderType = strings.ToLower(strings.TrimSpace(input.ProviderType))
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.APIKey = strings.TrimSpace(input.APIKey)

	if !validProviderType(input.ProviderType) {
		return nil, ErrProviderTypeInvalid
	}
	if input.BaseURL == "" {
		return nil, errors.New("baseUrl is required")
	}

	apiKey := input.APIKey
	var extra []byte
	if input.ProviderID != nil && *input.ProviderID != uuid.Nil {
		row, err := s.repo.Get(ctx, projectID, *input.ProviderID)
		if err != nil {
			return nil, err
		}
		if apiKey == "" {
			apiKey = row.APIKey
		}
		if len(input.ExtraConfig) == 0 {
			extra = row.ExtraConfig
		}
	}
	if len(input.ExtraConfig) > 0 {
		if err := validateExtra(input.ExtraConfig); err != nil {
			return nil, err
		}
		extra = input.ExtraConfig
	}
	if input.ProviderType != ProviderTypeOllama && apiKey == "" {
		return nil, ErrProviderEmptyKey
	}

	spec, err := specFromParts(input.ProviderType, input.BaseURL, apiKey, extra)
	if err != nil {
		return nil, err
	}
	return s.listModelsForSpec(ctx, input.ProviderType, spec)
}

func (s *Service) listModelsForRow(ctx context.Context, row *providerRow) (*ListModelsResult, error) {
	if row == nil {
		return nil, ErrProviderNotFound
	}
	if row.ProviderType != ProviderTypeOllama && row.APIKey == "" {
		return nil, ErrProviderEmptyKey
	}
	spec, err := specFromParts(row.ProviderType, row.BaseURL, row.APIKey, row.ExtraConfig)
	if err != nil {
		return nil, err
	}
	return s.listModelsForSpec(ctx, row.ProviderType, spec)
}

func (s *Service) listModelsForSpec(ctx context.Context, providerType string, spec client.Spec) (*ListModelsResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	models, err := client.ListModels(timeoutCtx, spec)
	if err == nil && len(models) > 0 {
		return &ListModelsResult{Models: models, Source: modelsSourceAPI}, nil
	}

	// Upstream returned empty list without error — merge built-in suggestions.
	if err == nil && len(models) == 0 {
		if merged := mergeFallbackModels(providerType, models); len(merged) > 0 {
			return &ListModelsResult{
				Models:  merged,
				Source:  modelsSourceFallback,
				Warning: "上游未返回模型列表，已使用内置建议",
			}, nil
		}
	}

	fallback := fallbackModels(providerType)
	out := make([]client.ModelInfo, 0, len(fallback))
	for _, id := range fallback {
		out = append(out, client.ModelInfo{ID: id})
	}
	warning := ""
	if err != nil {
		warning = fmt.Sprintf("无法从上游获取模型列表：%v", err)
	} else {
		warning = "上游未返回可用模型，已使用内置建议列表"
	}
	return &ListModelsResult{
		Models:  out,
		Source:  modelsSourceFallback,
		Warning: warning,
	}, nil
}

func specFromParts(providerType, baseURL, apiKey string, extra []byte) (client.Spec, error) {
	extraMap, err := client.SpecFromExtra(extra)
	if err != nil {
		return client.Spec{}, err
	}
	return client.Spec{
		Type:        providerType,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		ExtraConfig: extraMap,
	}, nil
}

func mergeFallbackModels(providerType string, api []client.ModelInfo) []client.ModelInfo {
	fb := fallbackModels(providerType)
	if len(fb) == 0 {
		return api
	}
	seen := make(map[string]struct{}, len(api)+len(fb))
	out := make([]client.ModelInfo, 0, len(api)+len(fb))
	for _, m := range api {
		if m.ID == "" {
			continue
		}
		if _, ok := seen[m.ID]; ok {
			continue
		}
		seen[m.ID] = struct{}{}
		out = append(out, m)
	}
	for _, id := range fb {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, client.ModelInfo{ID: id})
	}
	return out
}

func fallbackModels(providerType string) []string {
	for _, meta := range supportedTypesMeta() {
		if meta.Type == providerType {
			return append([]string(nil), meta.Models...)
		}
	}
	return nil
}

// supportedTypesMeta is the canonical type metadata (including offline fallbacks).
func supportedTypesMeta() []ProviderTypeMeta {
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
			DefaultModel:   "mimo-v2-pro",
			Models: []string{
				"mimo-v2-pro",
				"mimo-v2-flash",
				"mimo-v2.5",
				"mimo-v2-omni",
			},
			APIKeyRequired: true,
			Notes:          "小米大模型网关（OpenAI 兼容）。含图片消息时平台会自动路由到 MiMo-V2.5（或网关中的 2.5/omni 多模态型号），纯文本可继续使用 Pro/Flash。",
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
