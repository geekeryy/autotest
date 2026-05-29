package aitools

import (
	"context"
	"encoding/json"
	"testing"
)

func sampleCatalog() *Catalog {
	tools := []Tool{
		{Name: "list_services", Domain: "meta", Summary: "列出当前项目下的全部服务及其ID"},
		{Name: "get_scenario", Domain: "scenarios", Summary: "查询场景详情及其所有步骤"},
		{Name: "create_scenario_with_steps", Domain: "scenarios", Summary: "一次性创建场景及其全部步骤",
			Parameters: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"additionalProperties":false}`)},
		{Name: "create_mock_route", Domain: "mock", Summary: "为Mock Server新增路由规则"},
		{Name: "list_test_data_tables", Domain: "testdata", Summary: "列出项目下全部测试数据表"},
	}
	return NewCatalog(tools)
}

func runTool(t *testing.T, tool Tool, ctx context.Context, args string) map[string]any {
	t.Helper()
	v, err := tool.Run(ctx, json.RawMessage(args))
	if err != nil {
		t.Fatalf("tool %s run error: %v", tool.Name, err)
	}
	out, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("tool %s returned non-map: %T", tool.Name, v)
	}
	return out
}

func TestFindTools_KeywordMatch(t *testing.T) {
	cat := sampleCatalog()
	tool := FindToolsTool(cat)
	if tool.Mutating {
		t.Fatal("find_tools must be read-only")
	}
	out := runTool(t, tool, context.Background(), `{"query":"创建场景"}`)
	results, _ := out["results"].([]FindToolResult)
	if len(results) == 0 {
		t.Fatal("expected at least one result for 创建场景")
	}
	if results[0].Name != "create_scenario_with_steps" {
		t.Fatalf("expected create_scenario_with_steps first, got %s", results[0].Name)
	}
	for _, r := range results {
		if r.Name == FindToolsName || r.Name == DescribeToolsName {
			t.Fatalf("find_tools must not surface meta tools, got %s", r.Name)
		}
	}
}

func TestFindTools_DomainFilter(t *testing.T) {
	cat := sampleCatalog()
	tool := FindToolsTool(cat)
	out := runTool(t, tool, context.Background(), `{"query":"","domain":"scenarios"}`)
	results, _ := out["results"].([]FindToolResult)
	if len(results) != 2 {
		t.Fatalf("expected 2 scenario tools, got %d", len(results))
	}
	for _, r := range results {
		if r.Domain != "scenarios" {
			t.Fatalf("domain filter leaked %s", r.Domain)
		}
	}
}

func TestFindTools_LimitDefaultAndCap(t *testing.T) {
	cat := sampleCatalog()
	tool := FindToolsTool(cat)
	out := runTool(t, tool, context.Background(), `{"query":"list","limit":1}`)
	results, _ := out["results"].([]FindToolResult)
	if len(results) != 1 {
		t.Fatalf("expected limit=1 to cap results, got %d", len(results))
	}
}

func TestDescribeTools_ReturnsSchemaAndRecordsCollector(t *testing.T) {
	cat := sampleCatalog()
	tool := DescribeToolsTool(cat)
	collector := NewDescribeCollector()
	ctx := WithDescribeCollector(context.Background(), collector)

	out := runTool(t, tool, ctx, `{"names":["create_scenario_with_steps","does_not_exist"]}`)
	count, _ := out["count"].(int)
	if count != 1 {
		t.Fatalf("expected 1 resolved tool (unknown skipped), got %d", count)
	}
	names := collector.Names()
	if len(names) != 1 || names[0] != "create_scenario_with_steps" {
		t.Fatalf("collector should record only resolved names, got %+v", names)
	}
}

func TestDescribeTools_NoCollectorIsSafe(t *testing.T) {
	cat := sampleCatalog()
	tool := DescribeToolsTool(cat)
	out := runTool(t, tool, context.Background(), `{"names":["list_services"]}`)
	if count, _ := out["count"].(int); count != 1 {
		t.Fatalf("expected 1 resolved tool, got %d", count)
	}
}

func TestDescribeCollector_Dedup(t *testing.T) {
	c := NewDescribeCollector()
	c.Add("a", "b", "a", "", "  ", "c")
	c.Add("b", "d")
	names := c.Names()
	want := []string{"a", "b", "c", "d"}
	if len(names) != len(want) {
		t.Fatalf("expected %v, got %v", want, names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("order mismatch: expected %v, got %v", want, names)
		}
	}
}
