package aiprovider

import (
	"encoding/json"
	"testing"
)

func TestParsePlannerOutput_Clean(t *testing.T) {
	text := `{"intent":"生成登录下单场景","domains":["scenarios","cases"],"workflow":["列接口","建模板","建场景"],"needsWrite":true,"ambiguities":[]}`
	out, err := parsePlannerOutput(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Intent != "生成登录下单场景" || !out.NeedsWrite {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(out.Domains) != 2 || out.Domains[0] != "scenarios" {
		t.Fatalf("unexpected domains: %+v", out.Domains)
	}
}

func TestParsePlannerOutput_CodeFenceAndProse(t *testing.T) {
	text := "好的，这是规划：\n```json\n{\"intent\":\"查看场景\",\"domains\":[\"scenarios\"],\"needsWrite\":false}\n```\n"
	out, err := parsePlannerOutput(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Domains) != 1 || out.Domains[0] != "scenarios" || out.NeedsWrite {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestParsePlannerOutput_UnknownDomainsDropped(t *testing.T) {
	text := `{"domains":["scenarios","banana","",  "MOCK"],"ambiguities":["  "]}`
	out, err := parsePlannerOutput(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Domains) != 2 {
		t.Fatalf("expected scenarios+mock only, got %+v", out.Domains)
	}
	// normalized to lowercase + deduped + known only
	if out.Domains[0] != "scenarios" || out.Domains[1] != "mock" {
		t.Fatalf("unexpected normalized domains: %+v", out.Domains)
	}
	if len(out.Ambiguities) != 0 {
		t.Fatalf("blank ambiguity should be dropped, got %+v", out.Ambiguities)
	}
}

func TestParsePlannerOutput_GarbageDegradesSafely(t *testing.T) {
	for _, text := range []string{"", "这不是 JSON", "<<<", "null"} {
		out, err := parsePlannerOutput(text)
		if err == nil {
			t.Fatalf("expected error for %q", text)
		}
		if out.Domains == nil || out.NeedsWrite {
			t.Fatalf("expected safe degraded output for %q, got %+v", text, out)
		}
	}
}

func TestMarshalPlannerOutput_NonNilSlices(t *testing.T) {
	raw := marshalPlannerOutput(safePlannerOutput())
	var back map[string]json.RawMessage
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("marshal/unmarshal: %v", err)
	}
	if _, ok := back["domains"]; !ok {
		t.Fatal("expected domains key present")
	}
}
