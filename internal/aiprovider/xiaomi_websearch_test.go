package aiprovider

import (
	"encoding/json"
	"testing"

	"autotest/internal/aiprovider/client"
)

func TestFilterProviderNativeToolCalls(t *testing.T) {
	calls := []client.ToolCall{
		{ID: "1", Name: "web_search", Arguments: json.RawMessage(`{"query":"news"}`)},
		{ID: "2", Name: "get_case", Arguments: json.RawMessage(`{}`)},
	}
	got := filterProviderNativeToolCalls(calls)
	if len(got) != 1 || got[0].Name != "get_case" {
		t.Fatalf("unexpected filtered calls: %+v", got)
	}
}

func TestAssistantWebSearchEnabled(t *testing.T) {
	enabled := true
	disabled := false
	provider := &providerRow{ProviderType: ProviderTypeXiaomi}
	if !assistantWebSearchEnabled(provider, &enabled) {
		t.Fatal("expected web search enabled for xiaomi")
	}
	if assistantWebSearchEnabled(provider, &disabled) {
		t.Fatal("expected web search disabled when flag false")
	}
	if assistantWebSearchEnabled(&providerRow{ProviderType: ProviderTypeDeepSeek}, &enabled) {
		t.Fatal("deepseek must not enable xiaomi web search")
	}
}

func TestMergeAssistantToolDefs_AppendsWebSearch(t *testing.T) {
	base := []client.ToolDefinition{{Name: "get_case"}}
	enabled := true
	got := mergeAssistantToolDefs(base, &providerRow{ProviderType: ProviderTypeXiaomi}, &enabled)
	if len(got) != 2 {
		t.Fatalf("expected builtin + web_search, got %d", len(got))
	}
	if got[1].Name != xiaomiWebSearchToolName {
		t.Fatalf("unexpected second tool: %+v", got[1])
	}
}
