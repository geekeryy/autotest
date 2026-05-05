package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunScenarioJavaScript_variablesAndStdout(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{"a": "1"}
	stdout, stderr, exitCode, err := runScenarioJavaScript(ctx, `
pm.variables.set("b", "2");
console.log("ok", pm.variables.get("b"));
`, vars)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode=%d", exitCode)
	}
	if stderr != "" {
		t.Fatalf("stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "ok") || vars["b"] != "2" {
		t.Fatalf("stdout=%q vars=%v", stdout, vars)
	}
}

func TestRunScenarioJavaScript_pmTestFail(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{}
	_, _, exitCode, err := runScenarioJavaScript(ctx, `
pm.test("x", function() { throw new Error("nope"); });
`, vars)
	if err == nil {
		t.Fatal("expected error")
	}
	if exitCode != 1 {
		t.Fatalf("exitCode=%d", exitCode)
	}
}

func TestRunScenarioJavaScript_contextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(15 * time.Millisecond)
		cancel()
	}()
	_, _, _, err := runScenarioJavaScript(ctx, `while (true) {}`, map[string]string{})
	if err == nil {
		t.Fatal("expected cancel error")
	}
}
