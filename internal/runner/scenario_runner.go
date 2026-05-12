package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"autotest/internal/paramsource"
	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/scenario"
	"autotest/internal/templating"
	"autotest/internal/testdata"

	"github.com/google/uuid"
)

// RunScenarioInput is the request body for POST /scenarios/{id}/run.
type RunScenarioInput struct {
	EnvironmentID uuid.UUID      `json:"environmentId"`
	Name          string         `json:"name"`
	Variables     map[string]any `json:"variables"`
}

// StepRunResult is the per-step outcome returned in the scenario run output.
type StepRunResult struct {
	Step       scenario.Step  `json:"step"`
	Result     *report.Result `json:"result,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	StepErrors []string       `json:"stepErrors,omitempty"`
}

// RunScenarioOutput is the full run output for a scenario execution.
type RunScenarioOutput struct {
	Run         *report.Run     `json:"run"`
	StepResults []StepRunResult `json:"stepResults"`
}

// scenarioRepository provides the scenario/step data needed by the runner.
type scenarioRepository interface {
	Get(ctx context.Context, id uuid.UUID) (*scenario.Scenario, error)
	ListSteps(ctx context.Context, scenarioID uuid.UUID) ([]scenario.Step, error)
}

// RunScenario executes all enabled steps of a scenario in order.
// Each step's full output is stored and made available to subsequent steps via
// the {{$steps[N].*}} variable syntax, where N is the step's permanent step_seq.
func (s *Service) RunScenario(ctx context.Context, scenarioRepo scenarioRepository, scenarioID uuid.UUID, input RunScenarioInput) (*RunScenarioOutput, error) {
	if input.EnvironmentID == uuid.Nil {
		return nil, errors.New("environmentId is required")
	}

	sc, err := scenarioRepo.Get(ctx, scenarioID)
	if err != nil {
		return nil, err
	}
	steps, err := scenarioRepo.ListSteps(ctx, scenarioID)
	if err != nil {
		return nil, err
	}

	env, err := s.projects.GetServiceEnvironment(ctx, sc.ProjectID, sc.ServiceID, input.EnvironmentID)
	if err != nil {
		return nil, err
	}

	// Base variables: environment variables merged with caller-supplied overrides.
	baseVars, err := mergeVariables(env.Variables, input.Variables)
	if err != nil {
		return nil, err
	}

	runName := strings.TrimSpace(input.Name)
	if runName == "" {
		runName = sc.Name
	}

	rawVars, _ := json.Marshal(baseVars)
	snapshot, _ := json.Marshal(map[string]any{
		"type":            "scenario",
		"scenarioId":      sc.ID,
		"scenarioName":    sc.Name,
		"environmentId":   env.ID,
		"environmentName": env.Name,
		"runName":         runName,
	})

	run, err := s.reports.CreateRun(ctx, report.CreateRunInput{
		Name:          runName,
		ProjectID:     sc.ProjectID,
		ServiceID:     sc.ServiceID,
		ScenarioID:    &sc.ID,
		EnvironmentID: env.ID,
		Variables:     rawVars,
		Snapshot:      snapshot,
	})
	if err != nil {
		return nil, fmt.Errorf("create scenario run: %w", err)
	}

	mockCfg := s.buildRunMockConfig(sc.ProjectID)
	executor := newScenarioExecution(s, run.ID, *env, baseVars, steps, mockCfg)
	overallStatus := executor.run(ctx)
	stepResults := executor.stepResults

	finished, err := s.reports.FinishRun(ctx, run.ID, overallStatus)
	if err != nil {
		return nil, err
	}

	return &RunScenarioOutput{Run: finished, StepResults: stepResults}, nil
}

func (s *Service) executeScenarioStep(
	ctx context.Context,
	runID uuid.UUID,
	step scenario.Step,
	env project.Environment,
	baseVars map[string]string,
	stepOutputs map[int]any,
	mockCfg *templating.MockExpanderConfig,
) (*report.Result, map[string]any, error) {
	switch step.StepType {
	case "", scenario.StepTypeAPI:
		return s.executeAPIScenarioStep(ctx, runID, step, env, baseVars, stepOutputs, mockCfg)
	case scenario.StepTypeDatabase:
		return s.executeDatabaseScenarioStep(ctx, runID, step, env, baseVars, stepOutputs, mockCfg)
	case scenario.StepTypeScript:
		return s.executeScriptScenarioStep(ctx, runID, step, baseVars, stepOutputs, mockCfg)
	default:
		return nil, nil, fmt.Errorf("step %d: unsupported step type %q", step.StepOrder, step.StepType)
	}
}

// apiStepConfig holds the optional step-level configuration for API steps.
// When non-empty, Assertions are appended to the test case's base assertions.
type apiStepConfig struct {
	Assertions json.RawMessage `json:"assertions"`
}

func (s *Service) executeAPIScenarioStep(
	ctx context.Context,
	runID uuid.UUID,
	step scenario.Step,
	env project.Environment,
	baseVars map[string]string,
	stepOutputs map[int]any,
	mockCfg *templating.MockExpanderConfig,
) (*report.Result, map[string]any, error) {
	tc, err := s.cases.Get(ctx, step.TestCaseID)
	if err != nil {
		return nil, nil, fmt.Errorf("step %d: load test case: %w", step.StepOrder, err)
	}

	// Compute effective request: merge step.RequestOverride on top of tc.Request.
	effectiveRequest, err := mergeRequestOverride(tc.Request, step.RequestOverride)
	if err != nil {
		return nil, nil, fmt.Errorf("step %d: merge request override: %w", step.StepOrder, err)
	}
	effectiveCase := *tc
	effectiveCase.Request = effectiveRequest

	// Apply step-level assertion overrides: append step assertions to base assertions.
	var stepCfg apiStepConfig
	if len(step.Config) > 0 {
		_ = json.Unmarshal(step.Config, &stepCfg)
	}
	if len(stepCfg.Assertions) > 0 {
		effectiveCase.Assertions = appendAssertions(tc.Assertions, stepCfg.Assertions)
	}

	// Build a per-step vars map and inject any $steps[N].* references that
	// appear in the effective request JSON so renderVariables can resolve them.
	stepVars := copyVars(baseVars)
	injectStepRefs(string(effectiveRequest), stepOutputs, stepVars)

	// Collect and resolve SQL + test data inline references.
	var inlineRefs []paramsource.InlineReference
	var testDataRefs []testdata.InlineReference
	var inlineScanErr error
	if reqDef, decodeErr := decodeRequest(effectiveRequest); decodeErr == nil {
		if reqDef.Method == "" {
			reqDef.Method = tc.Method
		}
		if reqDef.Path == "" {
			reqDef.Path = tc.Path
		}
		inlineRefs, inlineScanErr = collectInlineSQLReferences(reqDef)
		if inlineScanErr == nil {
			testDataRefs, inlineScanErr = collectInlineTestDataReferences(reqDef)
		}
	}

	variants := []caseRunVariant{{Variables: stepVars}}
	var runErr error
	if inlineScanErr != nil {
		runErr = inlineScanErr
	} else {
		if len(testDataRefs) > 0 {
			variants, runErr = s.applyInlineTestDataReferences(ctx, tc.ProjectID, testDataRefs, variants)
		}
		if runErr == nil && len(inlineRefs) > 0 {
			variants, runErr = s.applyInlineSQLReferences(ctx, tc, inlineRefs, variants)
		}
	}

	if runErr != nil {
		result, _ := s.reports.SaveResult(ctx, report.Result{
			RunID:      runID,
			TestCaseID: tc.ID,
			StepID:     &step.ID,
			Status:     report.ResultError,
			Error:      runErr.Error(),
		})
		return result, nil, runErr
	}

	result, err := s.runner.ExecuteCaseWithStepID(
		ctx, runID, step.ID, effectiveCase, env,
		variants[0].Variables,
		paramsource.MarshalSnapshots(variants[0].ParameterSourceSnapshots),
		mockCfg,
	)
	if err != nil {
		return result, nil, fmt.Errorf("step %d execute: %w", step.StepOrder, err)
	}

	reqParams := buildAPIRequestParams(effectiveRequest, tc.Path, variants[0].Variables, mockCfg)
	stepOutput := buildAPIStepOutput(result, reqParams)
	return result, stepOutput, nil
}

// buildAPIStepOutput extracts the status, headers, and parsed body from a
// result's ResponseSnapshot and returns them as the step's output map.
// reqParams, when non-nil, is included under the "request" key so subsequent
// steps can reference {{$steps[N].request.query.*}}, .pathvar.*, .body.*.
func buildAPIStepOutput(result *report.Result, reqParams map[string]any) map[string]any {
	if result == nil {
		return nil
	}
	snap, err := parseResponseSnapshot(result.ResponseSnapshot)
	if err != nil {
		return nil
	}

	flatHeaders := make(map[string]any, len(snap.Headers))
	for k, vals := range snap.Headers {
		if len(vals) > 0 {
			flatHeaders[k] = vals[0]
		}
	}

	var bodyValue any
	if len(snap.Body) > 0 {
		if jsonErr := json.Unmarshal(snap.Body, &bodyValue); jsonErr != nil {
			bodyValue = string(snap.Body)
		}
	}

	out := map[string]any{
		"status":  snap.StatusCode,
		"headers": flatHeaders,
		"body":    bodyValue,
	}
	if reqParams != nil {
		out["request"] = reqParams
	}
	return out
}

// buildAPIRequestParams builds the rendered request parameters (query, pathvar,
// body) from the effective request JSON and the final variable map so that
// subsequent steps can reference them via {{$steps[N].request.*}}.
//
// fallbackPath is tc.Path (OpenAPI-style {varName}) and is used only when the
// request definition does not carry its own path field.
func buildAPIRequestParams(effectiveRequest json.RawMessage, fallbackPath string, vars map[string]string, mockCfg *templating.MockExpanderConfig) map[string]any {
	reqDef, err := decodeRequest(effectiveRequest)
	if err != nil {
		return nil
	}
	effectiveVars := mergeRequestVariables(reqDef.Variables, vars)

	path := reqDef.Path
	if path == "" {
		path = fallbackPath
	}

	// query
	query := make(map[string]any, len(reqDef.Query))
	for k, v := range renderMap(reqDef.Query, effectiveVars, mockCfg) {
		query[k] = v
	}

	// pathvar – variables substituted into path placeholders
	pathvar := make(map[string]any)
	for _, name := range extractPathVarNames(path) {
		if val, ok := effectiveVars[name]; ok {
			pathvar[name] = renderVariables(val, effectiveVars, mockCfg)
		}
	}

	// body
	var bodyValue any
	if reqDef.Body != nil {
		bodyValue = renderAny(reqDef.Body, effectiveVars, mockCfg)
	}

	return map[string]any{
		"query":   query,
		"pathvar": pathvar,
		"body":    bodyValue,
	}
}

// mergeRequestOverride deep-merges the override JSON map onto the base request JSON map.
// Top-level keys in override replace keys in base; non-map values at top level are replaced.
func mergeRequestOverride(base, override json.RawMessage) (json.RawMessage, error) {
	if len(override) == 0 || string(override) == "{}" {
		return base, nil
	}
	if len(base) == 0 || string(base) == "{}" {
		return override, nil
	}
	var baseMap map[string]json.RawMessage
	if err := json.Unmarshal(base, &baseMap); err != nil {
		return base, nil
	}
	var overrideMap map[string]json.RawMessage
	if err := json.Unmarshal(override, &overrideMap); err != nil {
		return base, nil
	}
	for k, v := range overrideMap {
		baseMap[k] = v
	}
	merged, err := json.Marshal(baseMap)
	if err != nil {
		return base, fmt.Errorf("re-marshal merged request: %w", err)
	}
	return merged, nil
}

// parseResponseSnapshot converts the stored responseSnapshot JSON back into a
// minimal struct suitable for building step output.
func parseResponseSnapshot(raw json.RawMessage) (struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}, error) {
	var snap struct {
		StatusCode int                 `json:"statusCode"`
		Headers    map[string][]string `json:"headers"`
		Body       string              `json:"body"`
	}
	var out struct {
		StatusCode int
		Headers    http.Header
		Body       []byte
	}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		return out, fmt.Errorf("unmarshal response snapshot: %w", err)
	}
	h := make(http.Header, len(snap.Headers))
	for k, vals := range snap.Headers {
		h[k] = vals
	}
	out.StatusCode = snap.StatusCode
	out.Headers = h
	out.Body = []byte(snap.Body)
	return out, nil
}

// appendAssertions concatenates two JSON assertion arrays.
// If base is empty it returns extra; if extra is empty it returns base.
func appendAssertions(base, extra json.RawMessage) json.RawMessage {
	if len(extra) == 0 || string(extra) == "[]" || string(extra) == "null" {
		return base
	}
	if len(base) == 0 || string(base) == "[]" || string(base) == "null" {
		return extra
	}
	var baseArr, extraArr []json.RawMessage
	if err := json.Unmarshal(base, &baseArr); err != nil {
		return extra
	}
	if err := json.Unmarshal(extra, &extraArr); err != nil {
		return base
	}
	merged, err := json.Marshal(append(baseArr, extraArr...))
	if err != nil {
		return base
	}
	return merged
}

// copyVars returns a shallow copy of vars.
func copyVars(vars map[string]string) map[string]string {
	out := make(map[string]string, len(vars))
	for k, v := range vars {
		out[k] = v
	}
	return out
}
