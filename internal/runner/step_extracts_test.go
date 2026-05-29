package runner

import (
	"encoding/json"
	"testing"
)

func TestApplyStepExtractsWritesScenarioVars(t *testing.T) {
	t.Parallel()
	vars := map[string]string{}
	output := map[string]any{
		"response": map[string]any{
			"body": map[string]any{
				"data": map[string]any{"token": "tok-99"},
			},
		},
	}
	cfg, _ := json.Marshal(map[string]any{
		"extracts": []map[string]string{{
			"name": "authToken",
			"from": "response.body.data.token",
		}},
	})
	applyStepExtracts(cfg, output, vars)
	if vars["authToken"] != "tok-99" {
		t.Fatalf("authToken = %q, want tok-99", vars["authToken"])
	}
}
