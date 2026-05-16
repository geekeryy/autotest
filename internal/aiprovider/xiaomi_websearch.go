package aiprovider

import (
	"encoding/json"
	"strings"

	"autotest/internal/aiprovider/client"
)

const xiaomiWebSearchToolName = "web_search"

// xiaomiWebSearchToolDefinition is the built-in search tool required by the
// Xiaomi MiMo API. The platform executes searches and injects results; callers
// must not run this tool locally. See:
// https://platform.xiaomimimo.com/docs/zh-CN/usage-guide/tool-calling/web-search
func xiaomiWebSearchToolDefinition() client.ToolDefinition {
	return client.ToolDefinition{
		Name:        xiaomiWebSearchToolName,
		Description: "Search the web for current information",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query"
				}
			},
			"required": ["query"]
		}`),
	}
}

func isProviderNativeTool(name string) bool {
	return strings.TrimSpace(name) == xiaomiWebSearchToolName
}

// filterProviderNativeToolCalls removes Xiaomi provider-native tools that must
// not be executed or persisted by the assistant loop.
func filterProviderNativeToolCalls(calls []client.ToolCall) []client.ToolCall {
	if len(calls) == 0 {
		return calls
	}
	out := make([]client.ToolCall, 0, len(calls))
	for _, c := range calls {
		if isProviderNativeTool(c.Name) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func assistantWebSearchEnabled(provider *providerRow, enabled *bool) bool {
	if enabled == nil || !*enabled {
		return false
	}
	if provider == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(provider.ProviderType), ProviderTypeXiaomi)
}

func mergeAssistantToolDefs(base []client.ToolDefinition, provider *providerRow, webSearchEnabled *bool) []client.ToolDefinition {
	if !assistantWebSearchEnabled(provider, webSearchEnabled) {
		return base
	}
	out := make([]client.ToolDefinition, 0, len(base)+1)
	out = append(out, base...)
	out = append(out, xiaomiWebSearchToolDefinition())
	return out
}
