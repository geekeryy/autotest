package aiassert

import (
	"encoding/json"
	"testing"
)

func TestGenerateSchemaAssertionsFromWrappedResponse(t *testing.T) {
	raw := json.RawMessage(`{
		"status": "201",
		"body": {
			"type": "object",
			"required": ["code", "data"],
			"properties": {
				"code": {"type": "integer", "enum": [0, 1]},
				"data": {
					"type": "object",
					"required": ["id"],
					"properties": {
						"id": {"type": "string", "format": "uuid"}
					}
				}
			}
		}
	}`)

	body := mustExtractBody(t, raw)
	assertions, err := GenerateSchemaAssertions(body)
	if err != nil {
		t.Fatalf("GenerateSchemaAssertions: %v", err)
	}
	if len(assertions) == 0 {
		t.Fatal("expected schema assertions")
	}

	foundExists := false
	foundEnumMatches := false
	for _, a := range assertions {
		switch {
		case a["path"] == "$.code" && a["op"] == "matches":
			foundEnumMatches = true
		case a["path"] == "$.code" && a["op"] == "exists":
			foundExists = true
		}
		if a["op"] == "oneOf" {
			t.Fatalf("unexpected unsupported op oneOf: %#v", a)
		}
	}
	if !foundExists {
		t.Fatal("expected exists assertion on $.code")
	}
	if !foundEnumMatches {
		t.Fatal("expected matches assertion for multi-value enum")
	}
}

func TestInferResponseStatus(t *testing.T) {
	raw := json.RawMessage(`{"status":"201","body":{"type":"object"}}`)
	if got := inferResponseStatus(raw); got != 201 {
		t.Fatalf("inferResponseStatus = %d, want 201", got)
	}
}

func TestAnalyzeHistoryResponsesSkipsDynamicFields(t *testing.T) {
	responses := []json.RawMessage{
		json.RawMessage(`{"id":"a1","code":0}`),
		json.RawMessage(`{"id":"a2","code":0}`),
	}
	assertions, err := AnalyzeHistoryResponses(responses)
	if err != nil {
		t.Fatalf("AnalyzeHistoryResponses: %v", err)
	}
	for _, a := range assertions {
		if a["path"] == "$.id" && a["op"] == "eq" {
			t.Fatalf("should not infer eq on dynamic id field: %#v", a)
		}
	}
}

func mustExtractBody(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	var wrapper struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal wrapper: %v", err)
	}
	return wrapper.Body
}
