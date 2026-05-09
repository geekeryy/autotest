package scenario

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestMergeIntoForConfig(t *testing.T) {
	raw := []byte(`{"mode":"count","countExpression":"1"}`)
	out, err := mergeIntoForConfig(raw, 2)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	var seqs []int
	if err := json.Unmarshal(m["bodyStepSeqs"], &seqs); err != nil {
		t.Fatal(err)
	}
	if len(seqs) != 1 || seqs[0] != 2 {
		t.Fatalf("bodyStepSeqs: %#v", seqs)
	}
}

func TestValidateAttachUnderParent(t *testing.T) {
	a := AttachUnderParentInput{
		ParentStepID:  uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ForLoopBody:   true,
		ConditionElse: true,
	}
	if validateAttachUnderParent(&a) == nil {
		t.Fatal("expected error when multiple flags set")
	}
}

// TestMergeIntoConditionBranchPreservesLowercaseKeys 校验 condition 分支序列化后字段
// 名仍然是小写驼峰（left/operator/right/stepSeqs），与前端及 conditionStepConfig
// 保持一致；曾因 conditionBranchNorm 缺少 json tag 导致输出 PascalCase。
func TestMergeIntoConditionBranchPreservesLowercaseKeys(t *testing.T) {
	raw := []byte(`{"branches":[{"left":"$x","operator":"==","right":"1","stepSeqs":[]}],"elseStepSeqs":[]}`)
	out, err := mergeIntoConditionBranch(raw, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Branches []struct {
			Left     string `json:"left"`
			Operator string `json:"operator"`
			Right    string `json:"right"`
			StepSeqs []int  `json:"stepSeqs"`
		} `json:"branches"`
		ElseStepSeqs []int `json:"elseStepSeqs"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal merged condition config: %v\nraw=%s", err, string(out))
	}
	if len(decoded.Branches) != 1 {
		t.Fatalf("expected 1 branch, got %d (raw=%s)", len(decoded.Branches), string(out))
	}
	br := decoded.Branches[0]
	if br.Left != "$x" || br.Operator != "==" || br.Right != "1" {
		t.Fatalf("branch fields lost during round-trip: %+v (raw=%s)", br, string(out))
	}
	if len(br.StepSeqs) != 1 || br.StepSeqs[0] != 5 {
		t.Fatalf("stepSeqs not appended: %+v (raw=%s)", br.StepSeqs, string(out))
	}
}

// TestParseConditionConfigLegacyShape ensures the legacy left/operator/right/thenStepSeqs
// shape (without a `branches` array) is normalized into a single branch.
func TestParseConditionConfigLegacyShape(t *testing.T) {
	raw := []byte(`{"left":"$x","operator":"==","right":"1","thenStepSeqs":[7],"elseStepSeqs":[8]}`)
	norm, err := parseConditionConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(norm.Branches) != 1 {
		t.Fatalf("expected 1 branch, got %d", len(norm.Branches))
	}
	br := norm.Branches[0]
	if br.Left != "$x" || br.Operator != "==" || br.Right != "1" {
		t.Fatalf("legacy fields lost: %+v", br)
	}
	if len(br.StepSeqs) != 1 || br.StepSeqs[0] != 7 {
		t.Fatalf("legacy thenStepSeqs lost: %+v", br.StepSeqs)
	}
	if len(norm.ElseStepSeqs) != 1 || norm.ElseStepSeqs[0] != 8 {
		t.Fatalf("legacy elseStepSeqs lost: %+v", norm.ElseStepSeqs)
	}
}
