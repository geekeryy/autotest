package aitools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func noopRun(context.Context, json.RawMessage) (any, error) { return nil, nil }

// sampleTools returns a small, deterministic tool set spanning two domains
// and a tool without parameters/metadata to exercise edge cases.
func sampleTools() []Tool {
	return []Tool{
		Tool{
			Name:        "list_services",
			Description: "列出当前项目下的全部服务",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			Run:         noopRun,
		}.WithMeta("meta", "列出服务"),
		Tool{
			Name:        "create_service",
			Description: "在当前项目下新建服务",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`),
			Mutating:    true,
			Run:         noopRun,
		}.WithMeta("meta", "新建服务"),
		Tool{
			Name:            "delete_scenario",
			Description:     "删除整个测试场景",
			Parameters:      json.RawMessage(`{"type":"object","properties":{"scenarioId":{"type":"string"}}}`),
			Mutating:        true,
			RequiresConfirm: true,
			Run:             noopRun,
		}.WithMeta("scenarios", "删除场景"),
		// No Parameters / no metadata: ToDefinition should still default the
		// schema, and Index should tolerate empty domain/summary.
		{
			Name:        "bare_tool",
			Description: "无参数无元数据的工具",
			Run:         noopRun,
		},
	}
}

func TestNewCatalog_BuildAndOrder(t *testing.T) {
	t.Parallel()
	c := NewCatalog(sampleTools())
	if got := c.Len(); got != 4 {
		t.Fatalf("expected 4 tools, got %d", got)
	}
	names := c.Names()
	want := []string{"bare_tool", "create_service", "delete_scenario", "list_services"}
	if len(names) != len(want) {
		t.Fatalf("expected %v, got %v", want, names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names not sorted: expected %v, got %v", want, names)
		}
	}
}

func TestNewCatalog_SkipsEmptyAndDuplicateNames(t *testing.T) {
	t.Parallel()
	tools := []Tool{
		{Name: "  ", Description: "blank", Run: noopRun},
		{Name: "dup", Description: "first", Run: noopRun, Domain: "meta", Summary: "first"},
		{Name: "dup", Description: "second", Run: noopRun, Domain: "cases", Summary: "second"},
	}
	c := NewCatalog(tools)
	if c.Len() != 1 {
		t.Fatalf("expected 1 tool after skipping blank/dup, got %d", c.Len())
	}
	got, ok := c.Get("dup")
	if !ok {
		t.Fatalf("expected dup to exist")
	}
	if got.Summary != "first" {
		t.Errorf("first occurrence should win, got summary %q", got.Summary)
	}
}

func TestCatalog_IndexHasNoSchema(t *testing.T) {
	t.Parallel()
	c := NewCatalog(sampleTools())
	idx := c.Index()
	if len(idx) != 4 {
		t.Fatalf("expected 4 index entries, got %d", len(idx))
	}

	// Marshalling the index must not leak any JSON Schema fragments such as
	// "properties" or "parameters".
	blob, err := json.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	for _, banned := range []string{"properties", "parameters", "scenarioId", "type\":\"object"} {
		if strings.Contains(string(blob), banned) {
			t.Errorf("index unexpectedly contains schema fragment %q: %s", banned, blob)
		}
	}

	byName := map[string]IndexEntry{}
	for _, e := range idx {
		byName[e.Name] = e
	}
	if e := byName["create_service"]; e.Domain != "meta" || e.Summary != "新建服务" || !e.Mutating || e.RequiresConfirm {
		t.Errorf("create_service entry wrong: %+v", e)
	}
	if e := byName["delete_scenario"]; !e.Mutating || !e.RequiresConfirm {
		t.Errorf("delete_scenario should be mutating + requires confirm: %+v", e)
	}
	if e := byName["bare_tool"]; e.Domain != "" || e.Summary != "" || e.Mutating {
		t.Errorf("bare_tool should have empty metadata: %+v", e)
	}
}

func TestCatalog_LookupHasSchema(t *testing.T) {
	t.Parallel()
	c := NewCatalog(sampleTools())
	defs := c.Lookup([]string{"create_service", "list_services"})
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
	// Order is preserved from the request, not catalog order.
	if defs[0].Name != "create_service" || defs[1].Name != "list_services" {
		t.Fatalf("lookup order not preserved: %s, %s", defs[0].Name, defs[1].Name)
	}
	if defs[0].Description != "在当前项目下新建服务" {
		t.Errorf("expected full description, got %q", defs[0].Description)
	}
	if !strings.Contains(string(defs[0].Parameters), "name") {
		t.Errorf("expected parameters schema to be present: %s", defs[0].Parameters)
	}

	// A tool with no Parameters should still get a defaulted object schema.
	bare := c.Lookup([]string{"bare_tool"})
	if len(bare) != 1 {
		t.Fatalf("expected 1 definition for bare_tool, got %d", len(bare))
	}
	if len(strings.TrimSpace(string(bare[0].Parameters))) == 0 {
		t.Errorf("expected defaulted schema for bare_tool, got empty")
	}
}

func TestCatalog_LookupSkipsUnknownNames(t *testing.T) {
	t.Parallel()
	c := NewCatalog(sampleTools())
	defs := c.Lookup([]string{"create_service", "does_not_exist", "  ", "list_services"})
	if len(defs) != 2 {
		t.Fatalf("expected unknown/blank names skipped, got %d defs", len(defs))
	}
	if defs[0].Name != "create_service" || defs[1].Name != "list_services" {
		t.Errorf("unexpected definitions: %+v", defs)
	}

	// Entirely unknown lookup yields an empty (non-nil) slice.
	empty := c.Lookup([]string{"nope"})
	if empty == nil {
		t.Errorf("expected non-nil empty slice")
	}
	if len(empty) != 0 {
		t.Errorf("expected empty result, got %d", len(empty))
	}
}

func TestCatalog_ByDomain(t *testing.T) {
	t.Parallel()
	c := NewCatalog(sampleTools())

	meta := c.ByDomain("meta")
	want := []string{"create_service", "list_services"}
	if len(meta) != len(want) {
		t.Fatalf("expected %v, got %v", want, meta)
	}
	for i := range want {
		if meta[i] != want[i] {
			t.Fatalf("ByDomain not sorted/filtered correctly: expected %v, got %v", want, meta)
		}
	}

	if scenarios := c.ByDomain("scenarios"); len(scenarios) != 1 || scenarios[0] != "delete_scenario" {
		t.Errorf("expected [delete_scenario], got %v", scenarios)
	}

	// Unknown domain → empty, non-nil.
	unknown := c.ByDomain("nonexistent")
	if unknown == nil {
		t.Errorf("expected non-nil slice for unknown domain")
	}
	if len(unknown) != 0 {
		t.Errorf("expected empty slice for unknown domain, got %v", unknown)
	}
}
