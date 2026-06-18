package mcp

import (
	"context"
	_ "embed"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed template_reference.md
var templateReferenceMarkdown string

func registerTemplateTools(server *sdkmcp.Server) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "describe_template_syntax",
		Description: "Return autotest template variable reference for scenario/case authoring (no HTTP call).",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, any, error) {
		return toolText(templateReferenceMarkdown)
	})
}
