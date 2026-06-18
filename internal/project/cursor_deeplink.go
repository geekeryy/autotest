package project

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
)

const cursorMCPInstallScheme = "cursor://anysphere.cursor-deeplink/mcp/install"

// CursorInstallLink builds a one-click MCP install deeplink for Cursor.
// config is the transport block (same shape as a single entry in mcp.json, without the server name wrapper).
// See https://cursor.com/docs/mcp/install-links
func CursorInstallLink(serverName string, config any) (string, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshal mcp config: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	u, err := url.Parse(cursorMCPInstallScheme)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("name", serverName)
	q.Set("config", encoded)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func cursorServerName(serviceID string) string {
	sid := serviceID
	if len(sid) > 8 {
		sid = sid[:8]
	}
	return "autotest-" + sid
}
