package aiprovider

import (
	"context"
	"strings"

	"autotest/internal/aiconfig"
	"autotest/internal/aiprovider/client"
	"autotest/internal/aitools"
)

const defaultToolEmbedModel = aiconfig.DefaultToolEmbedModel

// ToolEmbedModel returns the embedding model used for find_tools query vectors.
func ToolEmbedModel() string {
	_, _, model := aiconfig.ToolEmbedSettings()
	if strings.TrimSpace(model) != "" {
		return strings.TrimSpace(model)
	}
	return defaultToolEmbedModel
}

func dedicatedEmbedClient() client.EmbeddingClient {
	base, key, _ := aiconfig.ToolEmbedSettings()
	base = strings.TrimSpace(base)
	if base == "" {
		return nil
	}
	return &client.OpenAICompatibleClient{
		BaseURL: base,
		APIKey:  strings.TrimSpace(key),
	}
}

func queryEmbedderFromConfig(cfg *streamConfig) aitools.QueryEmbedder {
	model := ToolEmbedModel()
	if ec := dedicatedEmbedClient(); ec != nil {
		return &aitools.ClientQueryEmbedder{
			ModelName: model,
			EmbedFn:     ec.Embed,
		}
	}
	if cfg == nil || cfg.Cli == nil {
		return nil
	}
	ec, ok := cfg.Cli.(client.EmbeddingClient)
	if !ok {
		return nil
	}
	// Anthropic chat clients do not implement EmbeddingClient; OpenAI-compatible
	// chat providers reuse the same gateway for /embeddings when supported.
	if cfg.Provider != nil {
		switch strings.ToLower(strings.TrimSpace(cfg.Provider.ProviderType)) {
		case ProviderTypeAnthropic:
			return nil
		}
	}
	return &aitools.ClientQueryEmbedder{
		ModelName: model,
		EmbedFn:     ec.Embed,
	}
}

func toolExecutionContext(ctx context.Context, cfg *streamConfig, sink StreamCallback) context.Context {
	ctx = toolContextWithProgress(ctx, sink)
	if emb := queryEmbedderFromConfig(cfg); emb != nil {
		ctx = aitools.WithQueryEmbedder(ctx, emb)
	}
	return ctx
}

// EmbedCatalogTools is a helper for the generator CLI: batch-embed catalog
// tool texts via an OpenAI-compatible /embeddings endpoint.
func EmbedCatalogTools(ctx context.Context, ec client.EmbeddingClient, model string, tools []aitools.Tool) (*aitools.ToolEmbeddingIndex, error) {
	if ec == nil {
		return nil, client.ErrUnsupportedProvider
	}
	names := make([]string, 0, len(tools))
	texts := make([]string, 0, len(tools))
	for _, t := range tools {
		name := strings.TrimSpace(t.Name)
		if name == "" || name == aitools.FindToolsName || name == aitools.DescribeToolsName {
			continue
		}
		text := aitools.ToolEmbedText(t)
		if text == "" {
			continue
		}
		names = append(names, name)
		texts = append(texts, text)
	}
	vecs, err := ec.Embed(ctx, model, texts)
	if err != nil {
		return nil, err
	}
	return aitools.BuildToolEmbeddingIndex(model, names, vecs)
}
