// Command gen-tool-embeddings precomputes catalog tool embeddings for
// find_tools hybrid retrieval and writes internal/aitools/data/tool_embeddings.json.
//
// Usage (reads .env when present):
//
//	AI_TOOL_EMBED_BASE_URL=https://api.openai.com/v1 \
//	AI_TOOL_EMBED_API_KEY=sk-... \
//	AI_TOOL_EMBED_MODEL=text-embedding-3-small \
//	go run ./cmd/gen-tool-embeddings
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"autotest/internal/aiprovider"
	"autotest/internal/aiprovider/client"
	"autotest/internal/aitools"
	"autotest/internal/aitools/builtin"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	out := flag.String("out", aitools.EmbeddingsDataPath, "output JSON path")
	baseURL := flag.String("base-url", strings.TrimSpace(os.Getenv("AI_TOOL_EMBED_BASE_URL")), "OpenAI-compatible embeddings base URL (…/v1)")
	apiKey := flag.String("api-key", strings.TrimSpace(os.Getenv("AI_TOOL_EMBED_API_KEY")), "embeddings API key")
	model := flag.String("model", aiprovider.ToolEmbedModel(), "embedding model id")
	flag.Parse()

	if strings.TrimSpace(*baseURL) == "" {
		fmt.Fprintln(os.Stderr, "缺少 embeddings 端点：设置 AI_TOOL_EMBED_BASE_URL 或 -base-url")
		os.Exit(2)
	}
	// ollama 不需要 API Key
	if strings.TrimSpace(*baseURL) != "http://localhost:11434/v1" {
		if strings.TrimSpace(*apiKey) == "" {
			fmt.Fprintln(os.Stderr, "缺少 API Key：设置 AI_TOOL_EMBED_API_KEY 或 -api-key")
			os.Exit(2)
		}
	}

	tools := builtin.All(builtin.Deps{})
	ec := &client.OpenAICompatibleClient{BaseURL: strings.TrimSpace(*baseURL), APIKey: strings.TrimSpace(*apiKey)}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	idx, err := aiprovider.EmbedCatalogTools(ctx, ec, strings.TrimSpace(*model), tools)
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成 embedding 失败: %v\n", err)
		os.Exit(1)
	}

	raw, err := aitools.WriteToolEmbeddingsJSON(idx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化失败: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "写入 %s 失败: %v\n", *out, err)
		os.Exit(1)
	}

	fmt.Printf("已写入 %s（model=%s tools=%d dim=%d）\n", *out, idx.Model, len(idx.Vectors), idx.Dimensions)
}
