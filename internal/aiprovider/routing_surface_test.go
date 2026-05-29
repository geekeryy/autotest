package aiprovider

import (
	"context"
	"encoding/json"
	"testing"

	"autotest/internal/aiconfig"
	"autotest/internal/aiprovider/client"
	"autotest/internal/aitools"

	"github.com/google/uuid"
)

// --- fakes -----------------------------------------------------------------

// recordingStore extends the behaviour of fakeSessionStore with a working
// AppendMessage (returns a real row with an incrementing seq) and captures
// the routing log so dynamic-surface tests can assert on per-hop telemetry.
type recordingStore struct {
	seq        int
	routingLog *RoutingLogInput
}

func (s *recordingStore) ListMessages(context.Context, uuid.UUID) ([]StoredMessage, error) {
	return []StoredMessage{{Role: "user", Content: "帮我创建一个登录场景", Status: storedStatusFinal}}, nil
}
func (s *recordingStore) AppendMessage(_ context.Context, sessionID uuid.UUID, in StoredMessageInput) (*StoredMessage, error) {
	s.seq++
	return &StoredMessage{ID: uuid.New(), SessionID: sessionID, Seq: s.seq, Role: in.Role, Content: in.Content, Status: in.Status}, nil
}
func (s *recordingStore) FindPendingToolCall(context.Context, uuid.UUID, string) (*StoredMessage, *StoredToolCall, error) {
	return nil, nil, nil
}
func (s *recordingStore) MarkPendingFinal(context.Context, uuid.UUID, json.RawMessage) error {
	return nil
}
func (s *recordingStore) MarkPendingRejected(context.Context, uuid.UUID) error { return nil }
func (s *recordingStore) GetSession(_ context.Context, _, _, sessionID uuid.UUID) (*StoredSession, error) {
	return &StoredSession{ID: sessionID}, nil
}
func (s *recordingStore) UpdateSessionTitle(_ context.Context, _, _, _ uuid.UUID, title string) (*StoredSession, error) {
	return &StoredSession{Title: title}, nil
}
func (s *recordingStore) AppendRoutingLog(_ context.Context, in RoutingLogInput) error {
	cp := in
	s.routingLog = &cp
	return nil
}

// scriptedStreamClient replays a fixed sequence of hop responses and records
// the tool definitions offered on each hop.
type scriptedStreamClient struct {
	hops        [][]client.ToolCall // tool calls to emit per hop; empty -> final text
	offeredPer  [][]string          // captured tool names offered per hop
	current     int
}

func (c *scriptedStreamClient) Chat(context.Context, []client.Message, client.Options) (*client.Result, error) {
	return &client.Result{}, nil
}

func (c *scriptedStreamClient) ChatStream(_ context.Context, _ []client.Message, opts client.Options, onEvent client.StreamCallback) error {
	offered := make([]string, 0, len(opts.Tools))
	for _, d := range opts.Tools {
		offered = append(offered, d.Name)
	}
	c.offeredPer = append(c.offeredPer, offered)

	var calls []client.ToolCall
	if c.current < len(c.hops) {
		calls = c.hops[c.current]
	}
	c.current++
	if len(calls) == 0 {
		if err := onEvent(client.StreamEvent{Kind: client.StreamEventText, Text: "完成"}); err != nil {
			return err
		}
		return onEvent(client.StreamEvent{Kind: client.StreamEventDone, Model: "test-model", Finish: "stop"})
	}
	for i := range calls {
		c := calls[i]
		if err := onEvent(client.StreamEvent{Kind: client.StreamEventToolCall, ToolCall: &c}); err != nil {
			return err
		}
	}
	return onEvent(client.StreamEvent{Kind: client.StreamEventDone, Model: "test-model", Finish: "tool_calls"})
}

// --- test ------------------------------------------------------------------

func dynamicTestTools() map[string]aitools.Tool {
	mk := func(name, domain string, mutating bool) aitools.Tool {
		return aitools.Tool{Name: name, Domain: domain, Mutating: mutating, Run: noopRun}
	}
	cat := aitools.NewCatalog([]aitools.Tool{
		mk("list_services", "meta", false),
		mk("get_scenario", "scenarios", false),
		mk("create_scenario_with_steps", "scenarios", true),
		mk("create_mock_route", "mock", true),
	})
	tools := map[string]aitools.Tool{
		"list_services":              mk("list_services", "meta", false),
		"get_scenario":               mk("get_scenario", "scenarios", false),
		"create_scenario_with_steps": mk("create_scenario_with_steps", "scenarios", true),
		"create_mock_route":          mk("create_mock_route", "mock", true),
		aitools.FindToolsName:         aitools.FindToolsTool(cat),
		aitools.DescribeToolsName:     aitools.DescribeToolsTool(cat),
	}
	return tools
}

func offeredContains(offered []string, name string) bool {
	for _, n := range offered {
		if n == name {
			return true
		}
	}
	return false
}

// TestDynamicSurface_DescribeExpandsNextHop verifies the heart of the
// on-demand surface: in dynamic mode a tool that the Router did NOT select
// is absent on hop 1, the model expands it via describe_tools, and it
// becomes mounted on hop 2 — all while cfg.Tools stays full so the tool can
// still execute. It also checks per-hop routing telemetry is recorded.
func TestDynamicSurface_DescribeExpandsNextHop(t *testing.T) {
	store := &recordingStore{}
	// Hop 1: model calls describe_tools for a mock-domain tool not in the
	// Router package. Hop 2: model produces a final answer.
	cli := &scriptedStreamClient{hops: [][]client.ToolCall{
		{{ID: "c1", Name: aitools.DescribeToolsName, Arguments: json.RawMessage(`{"names":["create_mock_route"]}`)}},
		{},
	}}
	cfg := &streamConfig{
		SessionID: uuid.New(),
		Provider:  &providerRow{DefaultModel: "test-model", ProviderType: ProviderTypeOpenAI},
		Cli:       cli,
		Store:     store,
		Sink:      func(AssistantStreamEvent) error { return nil },
		Tools:     dynamicTestTools(),
		HopBudget: aiconfig.DefaultMaxToolHops,
		Catalog:   aitools.NewCatalog(nil),
		// Router selected only the scenarios package; mock tools are NOT here.
		ActiveTools:    map[string]bool{"get_scenario": true, "create_scenario_with_steps": true},
		DescribedTools: map[string]bool{},
		Routing:        &routingState{Enabled: true, Mode: RoutingModeDynamic, Confidence: 0.9},
	}

	if err := new(Service).runStreamLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runStreamLoop: %v", err)
	}

	if len(cli.offeredPer) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(cli.offeredPer))
	}
	hop1, hop2 := cli.offeredPer[0], cli.offeredPer[1]

	// Hop 1: scenarios package + core + meta, but NOT the mock tool.
	if !offeredContains(hop1, aitools.DescribeToolsName) || !offeredContains(hop1, aitools.FindToolsName) {
		t.Fatalf("hop1 must always mount meta tools: %v", hop1)
	}
	if !offeredContains(hop1, "create_scenario_with_steps") {
		t.Fatalf("hop1 should mount Router-selected scenario tool: %v", hop1)
	}
	if offeredContains(hop1, "create_mock_route") {
		t.Fatalf("hop1 must NOT mount the un-selected mock tool: %v", hop1)
	}

	// Hop 2: the described mock tool is now mounted.
	if !offeredContains(hop2, "create_mock_route") {
		t.Fatalf("hop2 should mount the described mock tool: %v", hop2)
	}

	// Telemetry: per-hop offered/called recorded on the accumulator;
	// outcome set by the terminal branch. (finalizeRoutingLog, which writes
	// to the store, is exercised by the entry points; here we assert the
	// accumulator the loop fills.)
	if cfg.Routing.Outcome != "success" {
		t.Fatalf("expected success outcome, got %q", cfg.Routing.Outcome)
	}
	hops := cfg.Routing.Hops
	if len(hops) != 2 {
		t.Fatalf("expected 2 per-hop entries, got %d", len(hops))
	}
	if len(hops[0].Called) != 1 || hops[0].Called[0] != aitools.DescribeToolsName {
		t.Fatalf("hop1 called telemetry wrong: %+v", hops[0])
	}
	if !hops[0].Narrowed {
		t.Fatalf("hop1 should be marked narrowed in dynamic mode: %+v", hops[0])
	}

	// And finalizeRoutingLog should persist that accumulator to the store.
	new(Service).finalizeRoutingLog(context.Background(), cfg, store, nil)
	if store.routingLog == nil || store.routingLog.Outcome != "success" {
		t.Fatalf("expected routing log persisted with success, got %+v", store.routingLog)
	}
}

// TestDynamicSurface_ShadowModeShipsFullSet ensures shadow mode never
// narrows the wire definitions even though routing telemetry is recorded.
func TestDynamicSurface_ShadowModeShipsFullSet(t *testing.T) {
	store := &recordingStore{}
	cli := &scriptedStreamClient{hops: [][]client.ToolCall{{}}}
	full := dynamicTestTools()
	defs := make([]client.ToolDefinition, 0, len(full))
	for _, tlt := range full {
		defs = append(defs, tlt.ToDefinition())
	}
	cfg := &streamConfig{
		SessionID:      uuid.New(),
		Provider:       &providerRow{DefaultModel: "test-model", ProviderType: ProviderTypeOpenAI},
		Cli:            cli,
		Store:          store,
		Sink:           func(AssistantStreamEvent) error { return nil },
		Tools:          full,
		ToolDefs:       defs,
		BaseOpts:       client.Options{Tools: defs},
		HopBudget:      aiconfig.DefaultMaxToolHops,
		Catalog:        aitools.NewCatalog(nil),
		ActiveTools:    map[string]bool{"get_scenario": true},
		DescribedTools: map[string]bool{},
		Routing:        &routingState{Enabled: true, Mode: RoutingModeShadow, Confidence: 0.9},
	}
	if err := new(Service).runStreamLoop(context.Background(), cfg); err != nil {
		t.Fatalf("runStreamLoop: %v", err)
	}
	if len(cli.offeredPer) != 1 {
		t.Fatalf("expected 1 hop, got %d", len(cli.offeredPer))
	}
	if len(cli.offeredPer[0]) != len(defs) {
		t.Fatalf("shadow mode must ship full set (%d), got %d", len(defs), len(cli.offeredPer[0]))
	}
	hops := cfg.Routing.Hops
	if len(hops) != 1 || hops[0].Narrowed {
		t.Fatalf("shadow hop must be recorded and not narrowed: %+v", hops)
	}
}
