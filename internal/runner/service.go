package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	testcase "autotest/internal/case"
	"autotest/internal/paramsource"
	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/testdata"

	"github.com/google/uuid"
)

type Service struct {
	cases    *testcase.Service
	projects *project.ServiceLayer
	reports  *report.Repository
	runner   *Runner
	params   parameterSourceService
	testData testDataService
}

type parameterSourceService interface {
	ExecuteInlineReferences(ctx context.Context, projectID, serviceID uuid.UUID, refs []paramsource.InlineReference, vars map[string]string) (map[string]string, []paramsource.ExecutionSnapshot, error)
	ExecuteSQLStep(ctx context.Context, projectID, serviceID, environmentID uuid.UUID, input paramsource.SQLStepInput, vars map[string]string) (paramsource.SQLStepResult, error)
}

// testDataService is the minimal slice of `internal/testdata.Service` the
// runner needs to resolve `{{$ds.*}}` references at run time. Defined locally
// so tests can substitute a fake without dragging in the full service.
type testDataService interface {
	Resolve(ctx context.Context, projectID uuid.UUID, refs []testdata.InlineReference) (map[string]string, []testdata.Snapshot, error)
}

type RunCaseInput struct {
	EnvironmentID uuid.UUID       `json:"environmentId"`
	Name          string          `json:"name"`
	Request       json.RawMessage `json:"request"`
	Variables     map[string]any  `json:"variables"`
}

type RunCaseOutput struct {
	Run     *report.Run     `json:"run"`
	Result  *report.Result  `json:"result"`
	Results []report.Result `json:"results,omitempty"`
}

type RunResultOutput struct {
	Run     *report.Run     `json:"run"`
	Result  *report.Result  `json:"result,omitempty"`
	Results []report.Result `json:"results"`
}

func NewService(cases *testcase.Service, projects *project.ServiceLayer, reports *report.Repository, runner *Runner, params parameterSourceService, testData testDataService) *Service {
	return &Service{
		cases:    cases,
		projects: projects,
		reports:  reports,
		runner:   runner,
		params:   params,
		testData: testData,
	}
}

func (s *Service) RunCase(ctx context.Context, testCaseID uuid.UUID, input RunCaseInput) (*RunCaseOutput, error) {
	if input.EnvironmentID == uuid.Nil {
		return nil, errors.New("environmentId is required")
	}

	tc, err := s.cases.Get(ctx, testCaseID)
	if err != nil {
		return nil, err
	}
	env, err := s.projects.GetServiceEnvironment(ctx, tc.ProjectID, tc.ServiceID, input.EnvironmentID)
	if err != nil {
		return nil, err
	}

	effectiveCase := *tc
	effectiveRequest := tc.Request
	if len(input.Request) > 0 {
		effectiveRequest = input.Request
		effectiveCase.Request = effectiveRequest
	}
	var inlineRefs []paramsource.InlineReference
	var testDataRefs []testdata.InlineReference
	var inlineScanErr error
	if reqDef, err := decodeRequest(effectiveRequest); err == nil {
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
	if inlineScanErr == nil {
		var variableRefs []paramsource.InlineReference
		variableRefs, inlineScanErr = collectInlineSQLReferencesFromVariables(input.Variables)
		inlineRefs = append(inlineRefs, variableRefs...)
	}
	if inlineScanErr == nil {
		var dsVariableRefs []testdata.InlineReference
		dsVariableRefs, inlineScanErr = collectInlineTestDataReferencesFromVariables(input.Variables)
		testDataRefs = append(testDataRefs, dsVariableRefs...)
	}

	baseVars, err := mergeVariables(env.Variables, input.Variables)
	if err != nil {
		return nil, err
	}
	runName := strings.TrimSpace(input.Name)
	if runName == "" {
		runName = tc.Name
	}

	variants := []caseRunVariant{{Variables: baseVars}}
	var runErr error
	if inlineScanErr != nil {
		runErr = inlineScanErr
	} else {
		variants, runErr = s.applyInlineTestDataReferences(ctx, tc.ProjectID, testDataRefs, variants)
		if runErr == nil {
			variants, runErr = s.applyInlineSQLReferences(ctx, tc, inlineRefs, variants)
		}
	}
	rawVars, _ := json.Marshal(variants[0].Variables)
	rawParamSnapshots := paramsource.MarshalSnapshots(variants[0].ParameterSourceSnapshots)
	snapshot, _ := json.Marshal(map[string]any{
		"type":                     "case",
		"testCaseId":               tc.ID,
		"templateName":             tc.Name,
		"environmentId":            env.ID,
		"environmentName":          env.Name,
		"runName":                  runName,
		"parameterSourceSnapshots": variants[0].ParameterSourceSnapshots,
		"testDataSnapshots":        variants[0].TestDataSnapshots,
	})

	run, err := s.reports.CreateRun(ctx, report.CreateRunInput{
		Name:          runName,
		ProjectID:     tc.ProjectID,
		ServiceID:     tc.ServiceID,
		EnvironmentID: env.ID,
		Variables:     rawVars,
		Snapshot:      snapshot,
	})
	if err != nil {
		return nil, err
	}
	if runErr != nil {
		result, saveErr := s.reports.SaveResult(ctx, report.Result{
			RunID:                    run.ID,
			TestCaseID:               tc.ID,
			Status:                   report.ResultError,
			ParameterSourceSnapshots: rawParamSnapshots,
			Error:                    runErr.Error(),
		})
		if saveErr != nil {
			return nil, saveErr
		}
		finished, finishErr := s.reports.FinishRun(ctx, run.ID, report.RunFailed)
		if finishErr != nil {
			return nil, finishErr
		}
		return &RunCaseOutput{Run: finished, Result: result, Results: []report.Result{*result}}, runErr
	}

	firstResult, results, status, firstRunErr := executeCaseRunVariants(ctx, variants, func(ctx context.Context, variant caseRunVariant) (*report.Result, error) {
		return s.runner.ExecuteCase(
			ctx,
			run.ID,
			effectiveCase,
			*env,
			variant.Variables,
			paramsource.MarshalSnapshots(variant.ParameterSourceSnapshots),
		)
	})

	finished, finishErr := s.reports.FinishRun(ctx, run.ID, status)
	if finishErr != nil {
		return nil, finishErr
	}
	if firstRunErr != nil {
		return &RunCaseOutput{Run: finished, Result: firstResult, Results: results}, firstRunErr
	}

	return &RunCaseOutput{Run: finished, Result: firstResult, Results: results}, nil
}

func (s *Service) applyInlineSQLReferences(ctx context.Context, tc *testcase.TestCase, refs []paramsource.InlineReference, variants []caseRunVariant) ([]caseRunVariant, error) {
	if len(refs) == 0 {
		return variants, nil
	}
	if s.params == nil {
		return variants, errors.New("sql parameter source service is unavailable")
	}
	for idx := range variants {
		inlineVars, snapshots, err := s.params.ExecuteInlineReferences(ctx, tc.ProjectID, tc.ServiceID, refs, variants[idx].Variables)
		variants[idx].ParameterSourceSnapshots = append(variants[idx].ParameterSourceSnapshots, snapshots...)
		if err != nil {
			return variants, err
		}
		for key, value := range inlineVars {
			variants[idx].Variables[key] = value
		}
	}
	return variants, nil
}

// applyInlineTestDataReferences resolves `{{$ds.*}}` expressions for every
// variant. Resolved values are written into each variant's variables so that
// renderVariables can substitute them just like SQL inline references.
func (s *Service) applyInlineTestDataReferences(ctx context.Context, projectID uuid.UUID, refs []testdata.InlineReference, variants []caseRunVariant) ([]caseRunVariant, error) {
	if len(refs) == 0 {
		return variants, nil
	}
	if s.testData == nil {
		return variants, errors.New("test data service is unavailable")
	}
	for idx := range variants {
		inlineVars, snapshots, err := s.testData.Resolve(ctx, projectID, refs)
		variants[idx].TestDataSnapshots = append(variants[idx].TestDataSnapshots, snapshots...)
		if err != nil {
			return variants, err
		}
		if variants[idx].Variables == nil {
			variants[idx].Variables = map[string]string{}
		}
		for key, value := range inlineVars {
			variants[idx].Variables[key] = value
		}
	}
	return variants, nil
}

func (s *Service) GetRunResult(ctx context.Context, runID uuid.UUID) (*RunResultOutput, error) {
	if runID == uuid.Nil {
		return nil, errors.New("runId is required")
	}
	run, err := s.reports.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	results, err := s.reports.ListResults(ctx, runID)
	if err != nil {
		return nil, err
	}
	var first *report.Result
	if len(results) > 0 {
		first = &results[0]
	}
	return &RunResultOutput{Run: run, Result: first, Results: results}, nil
}

func mergeVariables(raw json.RawMessage, overrides map[string]any) (map[string]string, error) {
	vars := map[string]string{}
	if len(raw) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		envVars := map[string]any{}
		if err := decoder.Decode(&envVars); err != nil {
			return nil, fmt.Errorf("decode environment variables: %w", err)
		}
		for key, value := range envVars {
			vars[key] = variableString(value)
		}
	}
	for key, value := range overrides {
		vars[key] = variableString(value)
	}
	return vars, nil
}

type caseRunVariant struct {
	Variables                map[string]string
	ParameterSourceSnapshots []paramsource.ExecutionSnapshot
	TestDataSnapshots        []testdata.Snapshot
}

type caseRunVariantExecutor func(context.Context, caseRunVariant) (*report.Result, error)

func executeCaseRunVariants(ctx context.Context, variants []caseRunVariant, execute caseRunVariantExecutor) (*report.Result, []report.Result, report.RunStatus, error) {
	status := report.RunPassed
	var firstResult *report.Result
	var results []report.Result
	var firstErr error
	for _, variant := range variants {
		result, err := execute(ctx, variant)
		if result != nil {
			if firstResult == nil {
				firstResult = result
			}
			results = append(results, *result)
		}
		if err != nil {
			status = report.RunFailed
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if result == nil || result.Status != report.ResultPassed {
			status = report.RunFailed
		}
	}
	return firstResult, results, status, firstErr
}

func variableString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}
