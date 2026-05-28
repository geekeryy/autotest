package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverName = "autotest"
const serverVersion = "1.0.0"

// NewServer builds an MCP server with autotest Swagger/OpenAPI import tools.
func NewServer(cfg Config) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)
	client := NewAPIClient(cfg)
	RegisterTools(server, cfg, client)
	return server
}

// RunStdio serves MCP over stdin/stdout (Cursor, Claude Desktop, etc.).
func RunStdio(ctx context.Context, cfg Config) error {
	server := NewServer(cfg)
	return server.Run(ctx, &sdkmcp.StdioTransport{})
}
