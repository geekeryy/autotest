// Package client implements unified Chat clients for the supported AI providers.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Message is a single chat message exchanged with the LLM. We keep a single
// flat struct rather than provider-specific content-block arrays: each
// provider client is responsible for serialising the right wire format from
// the same neutral struct.
//
// Role semantics:
//   - "system": top-level instructions (Anthropic moves these to a separate field).
//   - "user": end-user / runtime input.
//   - "assistant": model output. May carry ToolCalls.
//   - "tool": local result of a tool execution. ToolCallID must reference the
//     ToolCall.ID emitted by a prior assistant message.
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	Images           []ImageAttachment `json:"-"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	Name             string     `json:"name,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall is a request from the model to invoke one of the declared tools.
// Arguments is the raw JSON object emitted by the model and is passed through
// to the tool runner unmodified.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolDefinition declares a tool that the model may call. Parameters must be
// a valid JSON Schema (draft-07 subset is safest across providers).
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Options describes per-call parameters that may override the provider defaults.
type Options struct {
	Model       string
	Temperature *float64
	MaxTokens   int
	JSONOnly    bool
	// Thinking optionally overrides provider-level reasoning controls for a
	// single request. Nil means use the provider's stored default.
	Thinking *OpenAIThinkingConfig
	// Tools, when non-empty, switches the call to function-calling mode.
	// JSONOnly is ignored when Tools is set.
	Tools []ToolDefinition
	// ToolChoice controls model tool selection. Valid values: "", "auto",
	// "none", "required". Empty string means "auto" when Tools is non-empty.
	ToolChoice string
}

// Result is the model response in a provider-agnostic shape.
//
// When the model decides to call tools, ToolCalls is non-empty and Text is
// usually empty (or contains a short intermediate thought). The caller is
// expected to execute every ToolCall and append a corresponding "tool" role
// Message to the conversation, then call Chat again.
type Result struct {
	Model            string
	Text             string
	ReasoningContent string
	FinishReason     string
	ToolCalls        []ToolCall
}

// Client is implemented by every provider-specific chat client.
type Client interface {
	Chat(ctx context.Context, messages []Message, opts Options) (*Result, error)
}

// StreamingClient is an optional capability. Providers that support
// server-side streaming implement this in addition to Client; callers should
// type-assert when they need streamed responses and fall back to Chat
// otherwise.
type StreamingClient interface {
	Client
	ChatStream(ctx context.Context, messages []Message, opts Options, onEvent StreamCallback) error
}

// StreamEventKind enumerates the discrete events surfaced to the SSE layer.
type StreamEventKind int

const (
	// StreamEventText carries a text delta (model is still talking).
	StreamEventText StreamEventKind = iota
	// StreamEventReasoning carries a reasoning-content delta. Callers may
	// persist it for providers that require it on tool-call continuation, but
	// should not show it as normal assistant text by default.
	StreamEventReasoning
	// StreamEventToolCall carries one fully-parsed tool call request. It
	// fires once per tool call after the provider finishes streaming its
	// arguments JSON; callers do not need to merge incremental fragments.
	StreamEventToolCall
	// StreamEventDone signals end-of-stream. It always fires last (even on
	// graceful aborts driven by the caller).
	StreamEventDone
)

// StreamEvent is the unified shape consumed by the service layer.
type StreamEvent struct {
	Kind             StreamEventKind
	Text             string
	ReasoningContent string
	ToolCall         *ToolCall
	Model            string
	Finish           string
}

// StreamCallback receives StreamEvents in order. Returning a non-nil error
// aborts the stream and surfaces the error from ChatStream.
type StreamCallback func(event StreamEvent) error

// Errors surfaced to the service layer.
var (
	ErrUnsupportedProvider  = errors.New("unsupported ai provider type")
	ErrEmptyResponse        = errors.New("ai provider returned empty response")
	ErrStreamingUnsupported = errors.New("ai provider does not support streaming")
)

// defaultHTTPClient is shared across provider clients.
var defaultHTTPClient = &http.Client{Timeout: 60 * time.Second}
