// Package aitools provides the registry and built-in implementations of
// tools that AI actions (smart analysis, future assistant chat) can call to
// fetch additional platform context. Tools are plain Go functions wrapped
// with metadata and exposed to the LLM via the function-calling protocol in
// `internal/aiprovider/client`.
//
// Tool implementations are expected to:
//   - be safe to call concurrently
//   - honour the supplied context for cancellation/timeouts
//   - return either a JSON-serialisable value (for the model) or a Go error
//   - never panic on bad model input — return an error instead
//
// The registry intentionally lives outside `aiprovider` so the latter can
// stay focused on provider/protocol concerns; tool implementations may
// freely depend on any internal/* service.
package aitools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"autotest/internal/aiprovider/client"
)

// Tool is a discoverable, callable function exposed to the LLM.
//
// Parameters must be a JSON Schema (draft-07 subset is safest across all
// supported providers) describing the function input. An empty value is
// treated as `{"type":"object","properties":{}}` (no arguments).
//
// Mutating signals that the tool changes platform state.
//
// RequiresConfirm marks destructive or high-risk writes (typically delete_*)
// that must pause the SSE stream until the user approves via
// /tool-calls/{id}/confirm. Non-destructive mutating tools (create/update)
// execute immediately in the tool loop.
//
// Domain / Summary / Prerequisites / RelatedTools / AntiPatterns are
// platform-internal routing metadata used by the on-demand tool surface
// (Router selection + find_tools retrieval). They are intentionally
// optional and MUST NOT be shipped to the LLM: ToDefinition / Describe
// only expose name/description/parameters. Treat these fields as private
// to the platform.
type Tool struct {
	Name            string
	Description     string
	Parameters      json.RawMessage
	Mutating        bool
	RequiresConfirm bool
	Run             func(ctx context.Context, args json.RawMessage) (any, error)

	// Domain is the coarse grouping used by the Router to pick a tool
	// package. One of: meta, cases, scenarios, mock, mockset, testdata,
	// paramsource, scripts, runs, spec.
	Domain string
	// Summary is a short (≤40 字) Chinese one-liner used for Router /
	// find_tools retrieval, kept independent of the verbose Description.
	Summary string
	// Prerequisites lists tool names that typically must run first.
	Prerequisites []string
	// RelatedTools lists tool names commonly used alongside this one.
	RelatedTools []string
	// AntiPatterns lists usage mistakes the Router/assistant should avoid.
	AntiPatterns []string
}

// WithMeta returns a copy of the tool with the platform-internal routing
// metadata (Domain + Summary) populated. These fields never reach the LLM
// wire definition; they exist only for the Router / find_tools catalog.
func (t Tool) WithMeta(domain, summary string) Tool {
	t.Domain = domain
	t.Summary = summary
	return t
}

// WithRelations returns a copy of the tool with the optional relational
// routing hints populated. Like WithMeta these are platform-internal and
// never shipped to the LLM.
func (t Tool) WithRelations(prerequisites, relatedTools, antiPatterns []string) Tool {
	t.Prerequisites = prerequisites
	t.RelatedTools = relatedTools
	t.AntiPatterns = antiPatterns
	return t
}

// ToDefinition converts the tool into a client-facing definition that can be
// shipped to either OpenAI or Anthropic clients without further adaptation.
// We default empty Parameters to an empty object schema because both wire
// formats require a present schema.
func (t Tool) ToDefinition() client.ToolDefinition {
	schema := t.Parameters
	if len(strings.TrimSpace(string(schema))) == 0 {
		schema = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return client.ToolDefinition{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  schema,
	}
}

// Registry holds a named collection of tools and supports concurrent reads.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry { return &Registry{tools: map[string]Tool{}} }

// MustRegister panics on the first registration error. Use this from main
// where a registration failure indicates a programming bug, not a runtime
// condition.
func (r *Registry) MustRegister(tools ...Tool) {
	for _, t := range tools {
		if err := r.Register(t); err != nil {
			panic(err)
		}
	}
}

// Register adds a tool to the registry. The tool name must be unique within
// the registry and Run must not be nil.
func (r *Registry) Register(tool Tool) error {
	name := strings.TrimSpace(tool.Name)
	if name == "" {
		return errors.New("aitools: tool name is empty")
	}
	if tool.Run == nil {
		return fmt.Errorf("aitools: tool %s has nil Run", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("aitools: tool %s already registered", name)
	}
	tool.Name = name
	r.tools[name] = tool
	return nil
}

// Get fetches a registered tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// ReadOnly returns the subset of registered tools with Mutating=false,
// sorted by name for deterministic prompt construction.
func (r *Registry) ReadOnly() []Tool {
	return r.Filter(func(t Tool) bool { return !t.Mutating })
}

// All returns every registered tool, sorted by name.
func (r *Registry) All() []Tool {
	return r.Filter(func(Tool) bool { return true })
}

// Filter returns the tools matching predicate, sorted by name.
func (r *Registry) Filter(predicate func(Tool) bool) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		if predicate(t) {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Describe maps a slice of tools to client-facing definitions in order.
func Describe(tools []Tool) []client.ToolDefinition {
	out := make([]client.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		out = append(out, t.ToDefinition())
	}
	return out
}
