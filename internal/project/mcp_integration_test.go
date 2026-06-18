package project

import (
	"net/http/httptest"
	"testing"

	"autotest/internal/config"

	"github.com/google/uuid"
)

func TestBuildMcpIntegrationGuide(t *testing.T) {
	t.Parallel()
	pid := uuid.New()
	sid := uuid.New()
	eid := uuid.New()
	req := httptest.NewRequest("GET", "http://example.com/api/v1/projects", nil)
	req.Host = "app.example.com:443"
	req.Header.Set("X-Forwarded-Proto", "https")

	guide := BuildMcpIntegrationGuide(
		config.MCPHTTP{Enabled: true, Path: "/mcp"},
		":8080",
		req,
		Service{ID: sid, ProjectID: pid, Name: "demo", McpEnabled: true},
		[]Environment{{ID: eid, Name: "test"}},
		"at-test-key-plain",
	)

	if guide.McpHttpURL != "https://app.example.com:443/mcp" {
		t.Fatalf("mcp url = %q", guide.McpHttpURL)
	}
	if guide.DefaultEnvironmentID != eid.String() {
		t.Fatalf("default env = %q", guide.DefaultEnvironmentID)
	}
	if guide.CursorInstallHTTP == "" || guide.CursorInstallStdio == "" {
		t.Fatal("expected cursor install links")
	}
}

func TestPatchMcpIntegrationEnvironment(t *testing.T) {
	t.Parallel()
	guide := McpIntegrationGuide{
		CursorServerName: "autotest-deadbeef",
		ApiKeyToken:      "at-rotate-me",
		CursorStdio:      []byte(`{"mcpServers":{"autotest-deadbeef":{"type":"stdio","env":{"AUTOTEST_ENVIRONMENT_ID":"old"}}}}}`),
	}
	newID := uuid.New()
	patched := PatchMcpIntegrationEnvironment(guide, newID)
	if patched.DefaultEnvironmentID != newID.String() {
		t.Fatalf("got %s", patched.DefaultEnvironmentID)
	}
}
