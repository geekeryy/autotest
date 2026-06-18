package project

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestCursorInstallLink(t *testing.T) {
	t.Parallel()
	link, err := CursorInstallLink("autotest-demo", map[string]any{
		"url": "http://localhost:8080/mcp",
		"headers": map[string]string{
			"Authorization": "Bearer at-test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, cursorMCPInstallScheme) {
		t.Fatalf("link = %q", link)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("name") != "autotest-demo" {
		t.Fatalf("name = %q", u.Query().Get("name"))
	}
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("config"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["url"] != "http://localhost:8080/mcp" {
		t.Fatalf("cfg = %+v", cfg)
	}
}
