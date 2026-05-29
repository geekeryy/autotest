package aitools

import (
	"context"
	"fmt"
)

// QueryEmbedder embeds a short natural-language query for catalog retrieval.
type QueryEmbedder interface {
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
	Model() string
}

type queryEmbedderKey struct{}

// WithQueryEmbedder stamps an embedder onto ctx for find_tools semantic search.
func WithQueryEmbedder(ctx context.Context, e QueryEmbedder) context.Context {
	if e == nil {
		return ctx
	}
	return context.WithValue(ctx, queryEmbedderKey{}, e)
}

// QueryEmbedderFromContext returns the embedder stamped on ctx, if any.
func QueryEmbedderFromContext(ctx context.Context) (QueryEmbedder, bool) {
	e, ok := ctx.Value(queryEmbedderKey{}).(QueryEmbedder)
	return e, ok && e != nil
}

// ClientQueryEmbedder adapts a low-level batch embed function to QueryEmbedder.
type ClientQueryEmbedder struct {
	ModelName string
	EmbedFn   func(ctx context.Context, model string, inputs []string) ([][]float32, error)
}

func (e *ClientQueryEmbedder) Model() string { return e.ModelName }

func (e *ClientQueryEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	if e == nil || e.EmbedFn == nil {
		return nil, fmt.Errorf("query embedder not configured")
	}
	vecs, err := e.EmbedFn(ctx, e.ModelName, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}
	return vecs[0], nil
}
