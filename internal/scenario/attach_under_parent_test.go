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
