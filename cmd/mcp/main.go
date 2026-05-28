// Command mcp runs the autotest Model Context Protocol server over stdio.
// Configure Cursor or other MCP clients to launch this binary with AUTOTEST_* env vars.
package main

import (
	"context"
	"os"

	"autotest/internal/logx"
	"autotest/internal/mcp"
)

func main() {
	logx.InitWith("error", "text")

	cfg, err := mcp.LoadConfigFromEnv()
	if err != nil {
		logx.Error("mcp config", "err", err)
		os.Exit(1)
	}

	if err := mcp.RunStdio(context.Background(), cfg); err != nil {
		logx.Error("mcp server", "err", err)
		os.Exit(1)
	}
}
