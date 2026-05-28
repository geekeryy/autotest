package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestResolveProjectService(t *testing.T) {
	t.Parallel()
	pid := uuid.New()
	sid := uuid.New()
	cfg := Config{ProjectID: pid.String(), ServiceID: sid.String()}

	gotPID, gotSID, err := resolveProjectService(cfg, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if gotPID != pid || gotSID != sid {
		t.Fatalf("got %s %s", gotPID, gotSID)
	}

	override := uuid.New()
	gotPID, gotSID, err = resolveProjectService(cfg, override.String(), sid.String())
	if err != nil || gotPID != override {
		t.Fatalf("override project: %v %s", err, gotPID)
	}
}

func TestLoadSpecBytes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(path, []byte(`{"openapi":"3.0.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := loadSpecBytes(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty data")
	}
	data, err = loadSpecBytes("", `openapi: "3.0.0"`)
	if err != nil || len(data) == 0 {
		t.Fatalf("content: %v", err)
	}
	if _, err := loadSpecBytes("", ""); err == nil {
		t.Fatal("expected error")
	}
}
