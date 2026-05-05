package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/scenario"

	"github.com/google/uuid"
)

const (
	defaultMaxLoopIterations  = 100
	hardMaxLoopIterations     = 1000
	maxControlNestingDepth    = 8
	maxScenarioStepExecutions = 1000
)

type scenarioExecution struct {
	service       *Service
	runID         uuid.UUID
	env           project.Environment
	baseVars      map[string]string
	stepOutputs   map[int]any
	steps         []scenario.Step
	stepsBySeq    map[int]scenario.Step
	controlledSeq map[int]bool
	stepResults   []StepRunResult
	executions    int
}

type forStepConfig struct {
	Mode            string          `json:"mode"`
	Count           int             `json:"count"`
	CountExpression string          `json:"countExpression"`
	Items           json.RawMessage `json:"items"`
	ItemsExpression string          `json:"itemsExpression"`
	ItemVar         string          `json:"itemVar"`
	IndexVar        string          `json:"indexVar"`
	BodyStepSeqs    []int           `json:"bodyStepSeqs"`
	MaxIterations   int             `json:"maxIterations"`
}

// conditionBranchConfig is one predicate branch or the legacy "then" arm.
type conditionBranchConfig struct {
	Left     string `json:"left"`
	Operator string `json:"operator"`
	Right    string `json:"right"`
	StepSeqs []int  `json:"stepSeqs"`
}

type conditionStepConfig struct {
	// Legacy single predicate + then/else (used when Branches is empty).
	Left         string `json:"left"`
	Operator     string `json:"operator"`
	Right        string `json:"right"`
	ThenStepSeqs []int  `json:"thenStepSeqs"`
	ElseStepSeqs []int  `json:"elseStepSeqs"`
	// Branches is ordered; the first matching predicate runs its StepSeqs. Empty means legacy mode.
	Branches []conditionBranchConfig `json:"branches"`
}

func (cfg conditionStepConfig) resolvedBranches() []conditionBranchConfig {
	if len(cfg.Branches) > 0 {
		out := make([]conditionBranchConfig, len(cfg.Branches))
		for i, b := range cfg.Branches {
			b.StepSeqs = uniquePositiveSeqs(b.StepSeqs)
			out[i] = b
		}
		return out
	}
	return []conditionBranchConfig{{
		Left:     cfg.Left,
		Operator: cfg.Operator,
		Right:    cfg.Right,
		StepSeqs: uniquePositiveSeqs(cfg.ThenStepSeqs),
	}}
}

func (cfg conditionStepConfig) resolvedElseSeqs() []int {
	return uniquePositiveSeqs(cfg.ElseStepSeqs)
}

func allConditionChildSeqs(cfg conditionStepConfig) []int {
	branches := cfg.resolvedBranches()
	elseSeqs := cfg.resolvedElseSeqs()
	seen := map[int]bool{}
	var out []int
	for _, br := range branches {
		for _, s := range br.StepSeqs {
			if s > 0 && !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	for _, s := range elseSeqs {
		if s > 0 && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func skippedConditionSeqs(branches []conditionBranchConfig, elseSeqs []int, pickedIdx int, usedElse bool) []int {
	var skipped []int
	if usedElse {
		for _, br := range branches {
			skipped = append(skipped, br.StepSeqs...)
		}
		return uniquePositiveSeqs(skipped)
	}
	for i, br := range branches {
		if i != pickedIdx {
			skipped = append(skipped, br.StepSeqs...)
		}
	}
	skipped = append(skipped, elseSeqs...)
	return uniquePositiveSeqs(skipped)
}

func newScenarioExecution(s *Service, runID uuid.UUID, env project.Environment, baseVars map[string]string, steps []scenario.Step) *scenarioExecution {
	stepsBySeq := make(map[int]scenario.Step, len(steps))
	for _, step := range steps {
		stepsBySeq[step.StepSeq] = step
	}
	return &scenarioExecution{
		service:       s,
		runID:         runID,
		env:           env,
		baseVars:      baseVars,
		stepOutputs:   make(map[int]any),
		steps:         steps,
		stepsBySeq:    stepsBySeq,
		controlledSeq: collectControlledStepSeqs(steps),
	}
}

func (e *scenarioExecution) run(ctx context.Context) report.RunStatus {
	overall := report.RunPassed
	for _, step := range e.steps {
		if !step.Enabled || e.controlledSeq[step.StepSeq] {
			continue
		}
		if e.executeStep(ctx, step, 0, map[int]bool{}) != report.RunPassed {
			overall = report.RunFailed
		}
	}
	return overall
}

func (e *scenarioExecution) executeStep(ctx context.Context, step scenario.Step, depth int, stack map[int]bool) report.RunStatus {
	if !step.Enabled {
		return report.RunPassed
	}
	if e.executions >= maxScenarioStepExecutions {
		err := fmt.Errorf("scenario execution exceeded %d step executions", maxScenarioStepExecutions)
		e.appendControlError(ctx, step, err, map[string]any{"limit": maxScenarioStepExecutions})
		return report.RunFailed
	}
	if depth > maxControlNestingDepth {
		err := fmt.Errorf("control flow nesting exceeded %d levels", maxControlNestingDepth)
		e.appendControlError(ctx, step, err, map[string]any{"limit": maxControlNestingDepth})
		return report.RunFailed
	}
	if stack[step.StepSeq] {
		err := fmt.Errorf("control flow cycle detected at step #%d", step.StepSeq)
		e.appendControlError(ctx, step, err, nil)
		return report.RunFailed
	}
	stack[step.StepSeq] = true
	defer delete(stack, step.StepSeq)

	e.executions++
	switch step.StepType {
	case scenario.StepTypeFor:
		return e.executeForStep(ctx, step, depth, stack)
	case scenario.StepTypeCondition:
		return e.executeConditionStep(ctx, step, depth, stack)
	default:
		return e.executeExecutableStep(ctx, step)
	}
}

func (e *scenarioExecution) executeExecutableStep(ctx context.Context, step scenario.Step) report.RunStatus {
	result, stepOutput, err := e.service.executeScenarioStep(ctx, e.runID, step, e.env, e.baseVars, e.stepOutputs)
	stepResult := StepRunResult{Step: step, Result: result, Output: stepOutput}
	if err != nil {
		stepResult.StepErrors = []string{err.Error()}
	}
	if stepOutput != nil {
		e.stepOutputs[step.StepSeq] = stepOutput
	}
	e.stepResults = append(e.stepResults, stepResult)
	if result == nil || result.Status != report.ResultPassed {
		return report.RunFailed
	}
	return report.RunPassed
}

func (e *scenarioExecution) executeForStep(ctx context.Context, step scenario.Step, depth int, stack map[int]bool) report.RunStatus {
	var cfg forStepConfig
	if err := decodeStepConfig(step.Config, &cfg); err != nil {
		e.appendControlError(ctx, step, err, nil)
		return report.RunFailed
	}
	cfg.BodyStepSeqs = uniquePositiveSeqs(cfg.BodyStepSeqs)
	items, mode, err := buildLoopItems(cfg, e.baseVars, e.stepOutputs)
	if err != nil {
		e.appendControlError(ctx, step, err, nil)
		return report.RunFailed
	}
	if err := e.ensureChildSteps(step, cfg.BodyStepSeqs); err != nil {
		e.appendControlError(ctx, step, err, map[string]any{"bodyStepSeqs": cfg.BodyStepSeqs})
		return report.RunFailed
	}

	output := map[string]any{
		"type":          scenario.StepTypeFor,
		"mode":          mode,
		"iterations":    len(items),
		"itemVar":       loopVarName(cfg.ItemVar, "item"),
		"indexVar":      loopVarName(cfg.IndexVar, "index"),
		"bodyStepSeqs":  cfg.BodyStepSeqs,
		"maxIterations": loopLimit(cfg.MaxIterations),
	}
	e.appendControlResult(ctx, step, report.ResultPassed, output, "")
	e.stepOutputs[step.StepSeq] = output

	overall := report.RunPassed
	for idx, item := range items {
		restore := e.applyLoopVariables(cfg, idx, item)
		childStatus := e.executeChildSteps(ctx, cfg.BodyStepSeqs, depth+1, stack)
		restore()
		if childStatus != report.RunPassed {
			overall = report.RunFailed
		}
	}
	return overall
}

func (e *scenarioExecution) executeConditionStep(ctx context.Context, step scenario.Step, depth int, stack map[int]bool) report.RunStatus {
	var cfg conditionStepConfig
	if err := decodeStepConfig(step.Config, &cfg); err != nil {
		e.appendControlError(ctx, step, err, nil)
		return report.RunFailed
	}

	branches := cfg.resolvedBranches()
	elseSeqs := cfg.resolvedElseSeqs()

	var union []int
	for _, br := range branches {
		union = append(union, br.StepSeqs...)
	}
	union = append(union, elseSeqs...)
	union = uniquePositiveSeqs(union)
	if err := e.ensureChildSteps(step, union); err != nil {
		e.appendControlError(ctx, step, err, map[string]any{"childStepSeqs": union})
		return report.RunFailed
	}

	var selected []int
	var branchLabel string
	var branchIdx = -1
	evaluations := make([]map[string]any, 0, len(branches))

	for i, br := range branches {
		sub := conditionStepConfig{Left: br.Left, Operator: br.Operator, Right: br.Right}
		ok, detail, err := evaluateCondition(sub, e.baseVars, e.stepOutputs)
		evaluations = append(evaluations, map[string]any{
			"index":   i,
			"matched": ok,
			"detail":  detail,
		})
		if err != nil {
			e.appendControlError(ctx, step, err, map[string]any{"branchIndex": i})
			return report.RunFailed
		}
		if ok {
			branchIdx = i
			selected = br.StepSeqs
			branchLabel = fmt.Sprintf("branch_%d", i)
			break
		}
	}

	usedElse := branchIdx < 0
	if usedElse {
		selected = elseSeqs
		branchLabel = "else"
	}

	skipped := skippedConditionSeqs(branches, elseSeqs, branchIdx, usedElse)
	e.appendSkippedSteps(skipped, fmt.Sprintf("condition path %q not entered", branchLabel))

	output := map[string]any{
		"type":                   scenario.StepTypeCondition,
		"branch":                 branchLabel,
		"selectedBranchIndex":    branchIdx,
		"usedElseBranch":         usedElse,
		"evaluations":            evaluations,
		"selectedStepSeqs":       selected,
		"elseStepSeqs":           elseSeqs,
		"resolvedBranchesConfig": branches,
	}
	e.appendControlResult(ctx, step, report.ResultPassed, output, "")
	e.stepOutputs[step.StepSeq] = output

	return e.executeChildSteps(ctx, selected, depth+1, stack)
}

func (e *scenarioExecution) executeChildSteps(ctx context.Context, seqs []int, depth int, stack map[int]bool) report.RunStatus {
	overall := report.RunPassed
	for _, seq := range seqs {
		child, ok := e.stepsBySeq[seq]
		if !ok || !child.Enabled {
			continue
		}
		if e.executeStep(ctx, child, depth, stack) != report.RunPassed {
			overall = report.RunFailed
		}
	}
	return overall
}

func (e *scenarioExecution) ensureChildSteps(parent scenario.Step, seqs []int) error {
	for _, seq := range seqs {
		if seq == parent.StepSeq {
			return fmt.Errorf("control step #%d cannot reference itself", parent.StepSeq)
		}
		if _, ok := e.stepsBySeq[seq]; !ok {
			return fmt.Errorf("control step #%d references missing child step #%d", parent.StepSeq, seq)
		}
	}
	return nil
}

func (e *scenarioExecution) appendControlError(ctx context.Context, step scenario.Step, err error, extra map[string]any) {
	output := map[string]any{
		"type":  step.StepType,
		"error": err.Error(),
	}
	for k, v := range extra {
		output[k] = v
	}
	e.appendControlResult(ctx, step, report.ResultError, output, err.Error())
	e.stepOutputs[step.StepSeq] = output
}

func (e *scenarioExecution) appendControlResult(ctx context.Context, step scenario.Step, status report.ResultStatus, output map[string]any, errText string) {
	start := time.Now()
	reqSnapshot, _ := json.Marshal(map[string]any{
		"type":   step.StepType,
		"stepId": step.ID,
		"config": jsonOrDefault(step.Config, []byte(`{}`)),
	})
	respSnapshot, _ := json.Marshal(output)
	result, saveErr := e.service.reports.SaveResult(ctx, report.Result{
		RunID:            e.runID,
		StepID:           &step.ID,
		Status:           status,
		DurationMillis:   time.Since(start).Milliseconds(),
		RequestSnapshot:  reqSnapshot,
		ResponseSnapshot: respSnapshot,
		Error:            errText,
	})
	stepResult := StepRunResult{Step: step, Result: result, Output: output}
	if saveErr != nil {
		stepResult.StepErrors = []string{saveErr.Error()}
	}
	e.stepResults = append(e.stepResults, stepResult)
}

func (e *scenarioExecution) appendSkippedSteps(seqs []int, reason string) {
	for _, seq := range seqs {
		step, ok := e.stepsBySeq[seq]
		if !ok || !step.Enabled {
			continue
		}
		e.stepResults = append(e.stepResults, StepRunResult{
			Step: step,
			Output: map[string]any{
				"skipped": true,
				"reason":  reason,
			},
		})
	}
}

func (e *scenarioExecution) applyLoopVariables(cfg forStepConfig, idx int, item any) func() {
	itemVar := loopVarName(cfg.ItemVar, "item")
	indexVar := loopVarName(cfg.IndexVar, "index")
	touched := map[string]*string{}
	set := func(key, value string) {
		if _, exists := touched[key]; !exists {
			if old, ok := e.baseVars[key]; ok {
				oldCopy := old
				touched[key] = &oldCopy
			} else {
				touched[key] = nil
			}
		}
		e.baseVars[key] = value
	}

	set(indexVar, strconv.Itoa(idx))
	set(itemVar, stepOutputStr(item))
	switch value := item.(type) {
	case map[string]any:
		for key, fieldValue := range value {
			set(itemVar+"."+key, stepOutputStr(fieldValue))
		}
	case []any:
		for i, fieldValue := range value {
			set(fmt.Sprintf("%s.%d", itemVar, i), stepOutputStr(fieldValue))
		}
	}

	return func() {
		for key, old := range touched {
			if old == nil {
				delete(e.baseVars, key)
				continue
			}
			e.baseVars[key] = *old
		}
	}
}

func collectControlledStepSeqs(steps []scenario.Step) map[int]bool {
	out := map[int]bool{}
	for _, step := range steps {
		switch step.StepType {
		case scenario.StepTypeFor:
			var cfg forStepConfig
			if decodeStepConfig(step.Config, &cfg) == nil {
				for _, seq := range cfg.BodyStepSeqs {
					if seq > 0 {
						out[seq] = true
					}
				}
			}
		case scenario.StepTypeCondition:
			var cfg conditionStepConfig
			if decodeStepConfig(step.Config, &cfg) == nil {
				for _, seq := range allConditionChildSeqs(cfg) {
					if seq > 0 {
						out[seq] = true
					}
				}
			}
		}
	}
	return out
}

func buildLoopItems(cfg forStepConfig, vars map[string]string, stepOutputs map[int]any) ([]any, string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		if strings.TrimSpace(cfg.ItemsExpression) != "" || len(cfg.Items) > 0 {
			mode = "array"
		} else {
			mode = "count"
		}
	}

	switch mode {
	case "count", "times":
		count, err := resolveLoopCount(cfg, vars, stepOutputs)
		if err != nil {
			return nil, "", err
		}
		if count > loopLimit(cfg.MaxIterations) {
			return nil, "", fmt.Errorf("loop count %d exceeds maxIterations %d", count, loopLimit(cfg.MaxIterations))
		}
		items := make([]any, count)
		for i := 0; i < count; i++ {
			items[i] = i
		}
		return items, "count", nil
	case "array", "items", "for_each", "foreach":
		items, err := resolveLoopArray(cfg, vars, stepOutputs)
		if err != nil {
			return nil, "", err
		}
		if len(items) > loopLimit(cfg.MaxIterations) {
			return nil, "", fmt.Errorf("loop item count %d exceeds maxIterations %d", len(items), loopLimit(cfg.MaxIterations))
		}
		return items, "array", nil
	default:
		return nil, "", fmt.Errorf("unsupported loop mode %q", cfg.Mode)
	}
}

func resolveLoopCount(cfg forStepConfig, vars map[string]string, stepOutputs map[int]any) (int, error) {
	if strings.TrimSpace(cfg.CountExpression) == "" {
		if cfg.Count < 0 {
			return 0, errors.New("loop count must be >= 0")
		}
		return cfg.Count, nil
	}
	raw := renderControlExpression(cfg.CountExpression, vars, stepOutputs)
	count, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("loop countExpression must render to an integer: %w", err)
	}
	if count < 0 {
		return 0, errors.New("loop countExpression must be >= 0")
	}
	return count, nil
}

func resolveLoopArray(cfg forStepConfig, vars map[string]string, stepOutputs map[int]any) ([]any, error) {
	raw := strings.TrimSpace(cfg.ItemsExpression)
	if raw != "" {
		raw = renderControlExpression(raw, vars, stepOutputs)
	} else if len(cfg.Items) > 0 {
		raw = string(cfg.Items)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("itemsExpression or items is required for array loop")
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("loop items must be valid JSON: %w", err)
	}
	items, err := coerceLoopItems(value)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func coerceLoopItems(value any) ([]any, error) {
	switch v := value.(type) {
	case []any:
		return v, nil
	case map[string]any:
		for _, key := range []string{"items", "data", "list", "rows", "results", "records"} {
			if arr, ok := v[key].([]any); ok {
				return arr, nil
			}
		}
		var onlyArray []any
		arrayFields := 0
		for _, field := range v {
			if arr, ok := field.([]any); ok {
				onlyArray = arr
				arrayFields++
			}
		}
		if arrayFields == 1 {
			return onlyArray, nil
		}
		return nil, errors.New("loop items must be a JSON array, or an object with an array field named items/data/list/rows/results/records")
	default:
		return nil, fmt.Errorf("loop items must be a JSON array, got %T", value)
	}
}

func evaluateCondition(cfg conditionStepConfig, vars map[string]string, stepOutputs map[int]any) (bool, map[string]any, error) {
	leftRaw := strings.TrimSpace(cfg.Left)
	if leftRaw == "" {
		return false, nil, errors.New("condition left expression is required")
	}
	op := normalizeConditionOperator(cfg.Operator)
	left := renderControlExpression(leftRaw, vars, stepOutputs)
	right := renderControlExpression(cfg.Right, vars, stepOutputs)
	detail := map[string]any{
		"left":      leftRaw,
		"operator":  op,
		"right":     cfg.Right,
		"leftValue": left,
	}
	if cfg.Right != "" {
		detail["rightValue"] = right
	}

	switch op {
	case "exists":
		return !isMissingControlValue(left), detail, nil
	case "not_exists":
		return isMissingControlValue(left), detail, nil
	case "truthy":
		return isTruthyControlValue(left), detail, nil
	case "falsy":
		return !isTruthyControlValue(left), detail, nil
	case "equals":
		return left == right, detail, nil
	case "not_equals":
		return left != right, detail, nil
	case "contains":
		return strings.Contains(left, right), detail, nil
	case "not_contains":
		return !strings.Contains(left, right), detail, nil
	case "greater_than", "greater_or_equal", "less_than", "less_or_equal":
		lf, rf, err := parseConditionNumbers(left, right)
		if err != nil {
			return false, detail, err
		}
		switch op {
		case "greater_than":
			return lf > rf, detail, nil
		case "greater_or_equal":
			return lf >= rf, detail, nil
		case "less_than":
			return lf < rf, detail, nil
		default:
			return lf <= rf, detail, nil
		}
	default:
		return false, detail, fmt.Errorf("unsupported condition operator %q", cfg.Operator)
	}
}

func renderControlExpression(input string, vars map[string]string, stepOutputs map[int]any) string {
	renderVars := copyVars(vars)
	injectStepRefs(input, stepOutputs, renderVars)
	return renderVariables(input, renderVars)
}

func normalizeConditionOperator(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "", "=", "==", "eq", "equals":
		return "equals"
	case "!=", "<>", "ne", "not_equals", "not-equals":
		return "not_equals"
	case "contains":
		return "contains"
	case "not_contains", "not-contains":
		return "not_contains"
	case ">", "gt", "greater_than", "greater-than":
		return "greater_than"
	case ">=", "gte", "greater_or_equal", "greater-or-equal":
		return "greater_or_equal"
	case "<", "lt", "less_than", "less-than":
		return "less_than"
	case "<=", "lte", "less_or_equal", "less-or-equal":
		return "less_or_equal"
	case "exists":
		return "exists"
	case "not_exists", "not-exists":
		return "not_exists"
	case "truthy":
		return "truthy"
	case "falsy":
		return "falsy"
	default:
		return strings.ToLower(strings.TrimSpace(op))
	}
}

func parseConditionNumbers(left, right string) (float64, float64, error) {
	lf, err := strconv.ParseFloat(strings.TrimSpace(left), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("left value %q is not numeric", left)
	}
	rf, err := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("right value %q is not numeric", right)
	}
	return lf, rf, nil
}

func isMissingControlValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || trimmed == "null" || trimmed == "undefined" || strings.Contains(trimmed, "{{")
}

func isTruthyControlValue(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "", "0", "false", "null", "undefined", "[]", "{}":
		return false
	default:
		return true
	}
}

func loopLimit(maxIterations int) int {
	if maxIterations <= 0 {
		return defaultMaxLoopIterations
	}
	if maxIterations > hardMaxLoopIterations {
		return hardMaxLoopIterations
	}
	return maxIterations
}

func loopVarName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	return name
}

func uniquePositiveSeqs(seqs []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(seqs))
	for _, seq := range seqs {
		if seq <= 0 || seen[seq] {
			continue
		}
		seen[seq] = true
		out = append(out, seq)
	}
	return out
}
