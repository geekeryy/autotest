package mcp

import sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

// RegisterTools attaches all autotest MCP tools to the server.
func RegisterTools(server *sdkmcp.Server, cfg Config, client *APIClient) {
	registerSpecTools(server, cfg, client)
	registerMetaTools(server, cfg, client)
	registerCaseTools(server, cfg, client)
	registerScenarioTools(server, cfg, client)
	registerRunTools(server, cfg, client)
	registerTemplateTools(server)
}
