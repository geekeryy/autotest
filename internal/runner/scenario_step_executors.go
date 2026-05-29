package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"autotest/internal/paramsource"
	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/scenario"
	"autotest/internal/templating"

	"github.com/google/uuid"
)

const (
	defaultScriptTimeoutMillis = 10_000
	maxScriptOutputBytes       = 1 << 20
)

type databaseStepConfig struct {
	DataSourceID  uuid.UUID       `json:"dataSourceId"`
	SQL           string          `json:"sql"`
	InputParams   json.RawMessage `json:"inputParams"`
	TimeoutMillis int             `json:"timeoutMillis"`
}

type scriptStepConfig struct {
	// Script is multi-line JavaScript (Postman-style pm.*). Legacy configs may use command only.
	Script        string `json:"script"`
	Command       string `json:"command"`
	TimeoutMillis int    `json:"timeoutMillis"`
}

func (s *Service) executeDatabaseScenarioStep(
	ctx context.Context,
	runID uuid.UUID,
	step scenario.Step,
	env project.Environment,
	baseVars map[string]string,
	steps []scenario.Step,
	stepOutputs map[int]any,
	_ *templating.MockExpanderConfig,
) (*report.Result, map[string]any, error) {
	start := time.Now()

	var cfg databaseStepConfig
	if err := decodeStepConfig(step.Config, &cfg); err != nil {
		return s.saveScenarioStepError(ctx, runID, step, start, nil, nil, err)
	}
	if s.params == nil {
		return s.saveScenarioStepError(ctx, runID, step, start, nil, nil, errors.New("sql parameter source service is unavailable"))
	}

	// Build per-step vars with step refs injected from SQL and inputParams.
	stepVars := copyVars(baseVars)
	injectStepRefs(cfg.SQL, stepOutputs, steps, stepVars)
	injectStepRefs(string(cfg.InputParams), stepOutputs, steps, stepVars)

	sqlResult, err := s.params.ExecuteSQLStep(ctx, env.ProjectID, env.ServiceID, env.ID, paramsource.SQLStepInput{
		DataSourceID:  cfg.DataSourceID,
		SQL:           cfg.SQL,
		InputParams:   cfg.InputParams,
		TimeoutMillis: cfg.TimeoutMillis,
	}, stepVars)

	reqSnapshot, _ := json.Marshal(map[string]any{
		"type":         scenario.StepTypeDatabase,
		"stepId":       step.ID,
		"dataSourceId": cfg.DataSourceID,
		"sql":          cfg.SQL,
		"inputParams":  jsonOrDefault(cfg.InputParams, []byte(`[]`)),
	})
	respSnapshot, _ := json.Marshal(sqlResult)
	status := report.ResultPassed
	errText := ""
	if err != nil {
		status = report.ResultError
		errText = err.Error()
	}

	result, saveErr := s.reports.SaveResult(ctx, report.Result{
		RunID:            runID,
		StepID:           &step.ID,
		Status:           status,
		DurationMillis:   time.Since(start).Milliseconds(),
		RequestSnapshot:  reqSnapshot,
		ResponseSnapshot: respSnapshot,
		Error:            errText,
	})
	if saveErr != nil {
		return nil, nil, saveErr
	}
	if err != nil {
		return result, nil, err
	}

	output := databaseStepOutput(sqlResult)
	return result, output, nil
}

func (s *Service) executeScriptScenarioStep(
	ctx context.Context,
	runID uuid.UUID,
	step scenario.Step,
	baseVars map[string]string,
	steps []scenario.Step,
	stepOutputs map[int]any,
	mockCfg *templating.MockExpanderConfig,
) (*report.Result, map[string]any, error) {
	start := time.Now()

	var cfg scriptStepConfig
	if err := decodeStepConfig(step.Config, &cfg); err != nil {
		return s.saveScenarioStepError(ctx, runID, step, start, nil, nil, err)
	}

	scriptSrc := strings.TrimSpace(cfg.Script)
	if scriptSrc == "" {
		scriptSrc = strings.TrimSpace(cfg.Command)
	}
	if scriptSrc == "" {
		return s.saveScenarioStepError(ctx, runID, step, start, nil, nil, errors.New("script is required"))
	}

	// Template pass: step refs and {{var}} in the script source.
	stepVars := copyVars(baseVars)
	injectStepRefs(scriptSrc, stepOutputs, steps, stepVars)
	rendered := renderVariables(scriptSrc, stepVars, mockCfg)

	// JS runtime variables: scenario + run input only (not temporary $steps[] keys).
	jsVars := copyVars(baseVars)
	timeout := time.Duration(cfg.TimeoutMillis) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultScriptTimeoutMillis * time.Millisecond
	}
	scriptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout, stderr, exitCode, runErr := runScenarioJavaScript(scriptCtx, rendered, jsVars)
	for k, v := range jsVars {
		baseVars[k] = v
	}

	status := report.ResultPassed
	errText := ""
	if runErr != nil {
		status = report.ResultFailed
		errText = runErr.Error()
		if scriptCtx.Err() == context.DeadlineExceeded {
			status = report.ResultError
			errText = "script step timed out"
		} else if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runErr, context.Canceled) {
			status = report.ResultError
		}
	}

	output := scriptStepOutput(stdout, stderr, exitCode, errText)
	reqSnapshot, _ := json.Marshal(map[string]any{
		"type":          scenario.StepTypeScript,
		"stepId":        step.ID,
		"script":        scriptSrc,
		"timeoutMillis": int(timeout / time.Millisecond),
	})
	respSnapshot, _ := json.Marshal(output)
	result, saveErr := s.reports.SaveResult(ctx, report.Result{
		RunID:            runID,
		StepID:           &step.ID,
		Status:           status,
		DurationMillis:   time.Since(start).Milliseconds(),
		RequestSnapshot:  reqSnapshot,
		ResponseSnapshot: respSnapshot,
		Error:            errText,
	})
	if saveErr != nil {
		return nil, nil, saveErr
	}
	if runErr != nil {
		return result, output, runErr
	}
	return result, output, nil
}

func (s *Service) saveScenarioStepError(ctx context.Context, runID uuid.UUID, step scenario.Step, start time.Time, reqSnapshot, respSnapshot json.RawMessage, err error) (*report.Result, map[string]any, error) {
	result, saveErr := s.reports.SaveResult(ctx, report.Result{
		RunID:            runID,
		StepID:           &step.ID,
		Status:           report.ResultError,
		DurationMillis:   time.Since(start).Milliseconds(),
		RequestSnapshot:  reqSnapshot,
		ResponseSnapshot: respSnapshot,
		Error:            err.Error(),
	})
	if saveErr != nil {
		return nil, nil, saveErr
	}
	return result, nil, err
}

func decodeStepConfig(raw json.RawMessage, dest any) error {
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode step config: %w", err)
	}
	return nil
}

func jsonOrDefault(raw json.RawMessage, fallback json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return fallback
	}
	return raw
}

// databaseStepOutput builds the output map for a database step, suitable for
// referencing via {{$steps[N].rows[0].field}}, {{$steps[N].firstRow.field}}, etc.
func databaseStepOutput(result paramsource.SQLStepResult) map[string]any {
	rows := make([]any, 0, len(result.Rows))
	for _, row := range result.Rows {
		rowAny := make(map[string]any, len(row))
		for key, value := range row {
			rowAny[key] = value
		}
		rows = append(rows, rowAny)
	}
	firstRow := map[string]any{}
	for key, value := range result.FirstRow {
		firstRow[key] = value
	}
	return map[string]any{
		"rows":         rows,
		"firstRow":     firstRow,
		"rowCount":     len(result.Rows),
		"rowsAffected": result.RowsAffected,
		"commandTag":   result.CommandTag,
	}
}

// scriptStepOutput builds the output map for a script step, suitable for
// referencing via {{$steps[N].stdout}}, {{$steps[N].stdoutJson.field}}, etc.
func scriptStepOutput(stdout, stderr string, exitCode int, errText string) map[string]any {
	out := map[string]any{
		"stdout":   stdout,
		"stderr":   stderr,
		"exitCode": exitCode,
		"success":  errText == "",
	}
	if errText != "" {
		out["error"] = errText
	}
	var stdoutJSON any
	if strings.TrimSpace(stdout) != "" && json.Unmarshal([]byte(stdout), &stdoutJSON) == nil {
		out["stdoutJson"] = stdoutJSON
	}
	return out
}

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return b.Buffer.Write(p)
	}
	remaining := b.limit - b.Buffer.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
