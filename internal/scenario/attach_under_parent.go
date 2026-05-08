package scenario

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AttachUnderParentInput is optional metadata on upsert: after saving this step,
// append its step_seq to the given control-flow parent's config (same transaction semantics as two Upserts in order).
type AttachUnderParentInput struct {
	ParentStepID uuid.UUID `json:"parentStepId"`
	// Exactly one of the following must be set (enforced in service):
	ForLoopBody            bool `json:"forLoopBody,omitempty"`
	ConditionElse          bool `json:"conditionElse,omitempty"`
	ConditionBranchIndex   *int `json:"conditionBranchIndex,omitempty"`
}

func mergeChildIntoParentConfig(parentType StepType, raw []byte, childSeq int, attach AttachUnderParentInput) ([]byte, error) {
	if childSeq <= 0 {
		return nil, errors.New("invalid child step_seq")
	}
	switch parentType {
	case StepTypeFor:
		if !attach.ForLoopBody {
			return nil, errors.New("for parent requires attachUnderParent.forLoopBody")
		}
		return mergeIntoForConfig(raw, childSeq)
	case StepTypeCondition:
		if attach.ConditionElse {
			return mergeIntoConditionElse(raw, childSeq)
		}
		if attach.ConditionBranchIndex != nil {
			return mergeIntoConditionBranch(raw, childSeq, *attach.ConditionBranchIndex)
		}
		return nil, errors.New("condition parent requires attachUnderParent.conditionElse or conditionBranchIndex")
	default:
		return nil, errors.New("parent step must be for or condition")
	}
}

func normalizeJSONObj(raw []byte) []byte {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return []byte("{}")
	}
	return raw
}

func mergeIntoForConfig(raw []byte, childSeq int) ([]byte, error) {
	var m map[string]json.RawMessage
	raw = normalizeJSONObj(raw)
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	seqs := decodeIntSliceJSON(m["bodyStepSeqs"])
	seqs = appendUniquePositive(seqs, childSeq)
	b, err := json.Marshal(seqs)
	if err != nil {
		return nil, err
	}
	m["bodyStepSeqs"] = b
	return json.Marshal(m)
}

func mergeIntoConditionElse(raw []byte, childSeq int) ([]byte, error) {
	norm, err := parseConditionConfig(raw)
	if err != nil {
		return nil, err
	}
	norm.ElseStepSeqs = appendUniquePositive(norm.ElseStepSeqs, childSeq)
	return marshalConditionNorm(norm)
}

func mergeIntoConditionBranch(raw []byte, childSeq int, branchIndex int) ([]byte, error) {
	if branchIndex < 0 {
		return nil, errors.New("conditionBranchIndex must be >= 0")
	}
	norm, err := parseConditionConfig(raw)
	if err != nil {
		return nil, err
	}
	if branchIndex >= len(norm.Branches) {
		return nil, fmt.Errorf("condition branch index %d out of range", branchIndex)
	}
	norm.Branches[branchIndex].StepSeqs = appendUniquePositive(norm.Branches[branchIndex].StepSeqs, childSeq)
	return marshalConditionNorm(norm)
}

type normalizedCondition struct {
	Branches     []conditionBranchNorm
	ElseStepSeqs []int
}

type conditionBranchNorm struct {
	Left, Operator, Right string
	StepSeqs                []int `json:"stepSeqs"`
}

func parseConditionConfig(raw []byte) (*normalizedCondition, error) {
	raw = normalizeJSONObj(raw)
	var cfg struct {
		Branches []struct {
			Left, Operator, Right string
			StepSeqs                []int `json:"stepSeqs"`
		} `json:"branches"`
		ElseStepSeqs []int `json:"elseStepSeqs"`
		Left, Operator, Right string
		ThenStepSeqs []int `json:"thenStepSeqs"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Branches) > 0 {
		branches := make([]conditionBranchNorm, len(cfg.Branches))
		for i, b := range cfg.Branches {
			branches[i] = conditionBranchNorm{
				Left: b.Left, Operator: b.Operator, Right: b.Right,
				StepSeqs: dedupePositive(b.StepSeqs),
			}
		}
		return &normalizedCondition{Branches: branches, ElseStepSeqs: dedupePositive(cfg.ElseStepSeqs)}, nil
	}
	branches := []conditionBranchNorm{{
		Left: cfg.Left, Operator: cfg.Operator, Right: cfg.Right,
		StepSeqs: dedupePositive(cfg.ThenStepSeqs),
	}}
	return &normalizedCondition{Branches: branches, ElseStepSeqs: dedupePositive(cfg.ElseStepSeqs)}, nil
}

func marshalConditionNorm(norm *normalizedCondition) ([]byte, error) {
	type outT struct {
		Branches     []conditionBranchNorm `json:"branches"`
		ElseStepSeqs []int                `json:"elseStepSeqs"`
	}
	return json.Marshal(outT{
		Branches:     norm.Branches,
		ElseStepSeqs: norm.ElseStepSeqs,
	})
}

func decodeIntSliceJSON(raw json.RawMessage) []int {
	if len(raw) == 0 {
		return nil
	}
	var arr []interface{}
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	var out []int
	for _, v := range arr {
		n, ok := jsonNumberToInt(v)
		if ok && n > 0 {
			out = append(out, n)
		}
	}
	return dedupePositive(out)
}

func jsonNumberToInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	default:
		return 0, false
	}
}

func appendUniquePositive(seqs []int, n int) []int {
	for _, x := range seqs {
		if x == n {
			return seqs
		}
	}
	seqs = append(seqs, n)
	sort.Ints(seqs)
	return seqs
}

func dedupePositive(seqs []int) []int {
	if len(seqs) == 0 {
		return nil
	}
	sort.Ints(seqs)
	var out []int
	var prev int
	for i, n := range seqs {
		if n <= 0 {
			continue
		}
		if i == 0 || n != prev {
			out = append(out, n)
			prev = n
		}
	}
	return out
}

func (r *Repository) GetStepByID(ctx context.Context, scenarioID, stepID uuid.UUID) (*Step, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select id, scenario_id, test_case_id, step_seq, step_order, step_type, name, enabled,
		       config, request_override, created_at, updated_at
		from test_scenario_steps
		where scenario_id = $1 and id = $2 and deleted_at is null
	`, scenarioID, stepID)
	step, err := scanStep(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrStepNotFound
		}
		return nil, err
	}
	return step, nil
}

func parentConfigBytes(cfg json.RawMessage) []byte {
	if len(cfg) == 0 {
		return []byte("{}")
	}
	return cfg
}

// attachStepUnderParent updates the parent control step after the child was persisted.
func (s *Service) attachStepUnderParent(ctx context.Context, scenarioID uuid.UUID, child *Step, attach AttachUnderParentInput) error {
	if attach.ParentStepID == uuid.Nil {
		return errors.New("attachUnderParent.parentStepId is required")
	}
	parent, err := s.repo.GetStepByID(ctx, scenarioID, attach.ParentStepID)
	if err != nil {
		return err
	}
	switch parent.StepType {
	case StepTypeFor:
		if !attach.ForLoopBody || attach.ConditionElse || attach.ConditionBranchIndex != nil {
			return errors.New("for parent requires attachUnderParent.forLoopBody only")
		}
	case StepTypeCondition:
		if attach.ForLoopBody {
			return errors.New("condition parent cannot use forLoopBody")
		}
	default:
		return errors.New("attachUnderParent.parentStepId must reference a for or condition step")
	}
	newRaw, err := mergeChildIntoParentConfig(parent.StepType, parentConfigBytes(parent.Config), child.StepSeq, attach)
	if err != nil {
		return err
	}
	if len(newRaw) == 0 {
		newRaw = []byte(`{}`)
	}
	en := parent.Enabled
	pIn := UpsertStepInput{
		StepOrder:       parent.StepOrder,
		StepType:        parent.StepType,
		Name:            parent.Name,
		Enabled:         &en,
		Config:          newRaw,
		RequestOverride: parent.RequestOverride,
	}
	if parent.TestCaseID != uuid.Nil {
		pIn.TestCaseID = parent.TestCaseID
	}
	if len(pIn.RequestOverride) == 0 {
		pIn.RequestOverride = []byte(`{}`)
	}
	_, err = s.repo.UpsertStep(ctx, scenarioID, pIn)
	return err
}

func validateAttachUnderParent(a *AttachUnderParentInput) error {
	if a == nil {
		return nil
	}
	n := 0
	if a.ForLoopBody {
		n++
	}
	if a.ConditionElse {
		n++
	}
	if a.ConditionBranchIndex != nil {
		n++
	}
	if n != 1 {
		return errors.New("attachUnderParent must set exactly one of forLoopBody, conditionElse, conditionBranchIndex")
	}
	return nil
}
