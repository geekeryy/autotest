package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	testcase "autotest/internal/case"
	"autotest/internal/project"
	"autotest/internal/report"

	"github.com/google/uuid"
)

type Service struct {
	cases    *testcase.Service
	projects *project.ServiceLayer
	reports  *report.Repository
	runner   *Runner
}

type RunCaseInput struct {
	EnvironmentID uuid.UUID       `json:"environmentId"`
	Name          string          `json:"name"`
	Request       json.RawMessage `json:"request"`
	Variables     map[string]any  `json:"variables"`
	SaveSnapshot  bool            `json:"saveSnapshot"`
}

type RunCaseOutput struct {
	Run    *report.Run    `json:"run"`
	Result *report.Result `json:"result"`
}

type RunResultOutput struct {
	Run     *report.Run     `json:"run"`
	Result  *report.Result  `json:"result,omitempty"`
	Results []report.Result `json:"results"`
}

func NewService(cases *testcase.Service, projects *project.ServiceLayer, reports *report.Repository, runner *Runner) *Service {
	return &Service{
		cases:    cases,
		projects: projects,
		reports:  reports,
		runner:   runner,
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

	vars, err := mergeVariables(env.Variables, input.Variables)
	if err != nil {
		return nil, err
	}
	rawVars, _ := json.Marshal(vars)
	runName := strings.TrimSpace(input.Name)
	if runName == "" {
		runName = tc.Name
	}
	snapshot, _ := json.Marshal(map[string]any{
		"type":            "case",
		"testCaseId":      tc.ID,
		"testCaseName":    tc.Name,
		"environmentId":   env.ID,
		"environmentName": env.Name,
		"runName":         runName,
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

	result, runErr := s.runner.ExecuteCase(ctx, run.ID, effectiveCase, *env, vars)
	status := report.RunPassed
	if runErr != nil || result == nil || result.Status != report.ResultPassed {
		status = report.RunFailed
	}

	finished, finishErr := s.reports.FinishRun(ctx, run.ID, status)
	if finishErr != nil {
		return nil, finishErr
	}
	if runErr != nil {
		return nil, runErr
	}
	if input.SaveSnapshot && result != nil {
		if err := s.cases.SaveRunSnapshot(ctx, tc.ID, effectiveRequest, result.ResponseSnapshot); err != nil {
			return nil, err
		}
	}

	return &RunCaseOutput{Run: finished, Result: result}, nil
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
