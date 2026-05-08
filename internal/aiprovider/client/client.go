// Package client implements unified Chat clients for the supported AI providers.
package client

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Message is a single chat message exchanged with the LLM.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Options describes per-call parameters that may override the provider defaults.
type Options struct {
	Model       string
	Temperature *float64
	MaxTokens   int
	JSONOnly    bool
}

// Result is the model response in a provider-agnostic shape.
type Result struct {
	Model        string
	Text         string
	FinishReason string
}

// Client is implemented by every provider-specific chat client.
type Client interface {
	Chat(ctx context.Context, messages []Message, opts Options) (*Result, error)
}

// Errors surfaced to the service layer.
var (
	ErrUnsupportedProvider = errors.New("unsupported ai provider type")
	ErrEmptyResponse       = errors.New("ai provider returned empty response")
)

// defaultHTTPClient is shared across provider clients.
var defaultHTTPClient = &http.Client{Timeout: 60 * time.Second}
