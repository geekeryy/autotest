package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	testcase "autotest/internal/case"
	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/runner"
	"autotest/internal/scenario"
	"autotest/internal/spec"

	"github.com/google/uuid"
)

const maxSpecBytes = 20 << 20

// APIClient calls autotest REST API using an API Key (Bearer at-...).
type APIClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewAPIClient(cfg Config) *APIClient {
	return &APIClient{
		baseURL: cfg.APIBaseURL,
		apiKey:  cfg.APIKey,
		http: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

func (c *APIClient) doJSON(ctx context.Context, method, path string, query url.Values, reqBody any, out any) (int, error) {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var body io.Reader
	if reqBody != nil {
		raw, err := json.Marshal(reqBody)
		if err != nil {
			return 0, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, parseAPIError(resp.StatusCode, respBody)
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

func (c *APIClient) doRaw(ctx context.Context, method, path string, query url.Values, contentType string, rawBody []byte, out any) (int, error) {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(rawBody))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, parseAPIError(resp.StatusCode, respBody)
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// ImportSpec posts OpenAPI/Swagger document bytes to the specs import endpoint.
func (c *APIClient) ImportSpec(ctx context.Context, projectID, serviceID uuid.UUID, content []byte, contentType string, syncMode string) (*spec.ImportSummary, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("spec content is empty")
	}
	if len(content) > maxSpecBytes {
		return nil, fmt.Errorf("spec content exceeds %d bytes limit", maxSpecBytes)
	}
	if contentType == "" {
		contentType = detectContentType(content)
	}
	path := fmt.Sprintf("/projects/%s/services/%s/specs/import", projectID, serviceID)
	q := url.Values{}
	if mode := strings.TrimSpace(syncMode); mode == string(spec.SyncModeOverwrite) {
		q.Set("sync", "overwrite")
	}
	var summary spec.ImportSummary
	if _, err := c.doRaw(ctx, http.MethodPost, path, q, contentType, content, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func (c *APIClient) ListEndpoints(ctx context.Context, projectID, serviceID uuid.UUID) ([]spec.Endpoint, error) {
	path := fmt.Sprintf("/projects/%s/services/%s/endpoints", projectID, serviceID)
	var out []spec.Endpoint
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIClient) ListSpecs(ctx context.Context, projectID, serviceID uuid.UUID) ([]spec.APISpec, error) {
	path := fmt.Sprintf("/projects/%s/services/%s/specs", projectID, serviceID)
	var out []spec.APISpec
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIClient) ListCases(ctx context.Context, projectID, serviceID uuid.UUID) ([]testcase.TestCase, error) {
	q := url.Values{}
	q.Set("projectId", projectID.String())
	if serviceID != uuid.Nil {
		q.Set("serviceId", serviceID.String())
	}
	var out []testcase.TestCase
	if _, err := c.doJSON(ctx, http.MethodGet, "/cases", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIClient) GetCase(ctx context.Context, caseID uuid.UUID) (*testcase.TestCase, error) {
	var out testcase.TestCase
	if _, err := c.doJSON(ctx, http.MethodGet, "/cases/"+caseID.String(), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) CreateCase(ctx context.Context, input testcase.CreateManualInput) (*testcase.TestCase, error) {
	var out testcase.TestCase
	if _, err := c.doJSON(ctx, http.MethodPost, "/cases", nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) PatchCase(ctx context.Context, caseID uuid.UUID, input testcase.PatchInput) (*testcase.TestCase, error) {
	var out testcase.TestCase
	if _, err := c.doJSON(ctx, http.MethodPatch, "/cases/"+caseID.String(), nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) ListScenarios(ctx context.Context, projectID, serviceID uuid.UUID) ([]scenario.Scenario, error) {
	q := url.Values{}
	q.Set("projectId", projectID.String())
	if serviceID != uuid.Nil {
		q.Set("serviceId", serviceID.String())
	}
	var out []scenario.Scenario
	if _, err := c.doJSON(ctx, http.MethodGet, "/scenarios", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIClient) GetScenario(ctx context.Context, scenarioID uuid.UUID) (*scenario.Scenario, error) {
	var out scenario.Scenario
	if _, err := c.doJSON(ctx, http.MethodGet, "/scenarios/"+scenarioID.String(), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) CreateScenario(ctx context.Context, input scenario.CreateScenarioInput) (*scenario.Scenario, error) {
	var out scenario.Scenario
	if _, err := c.doJSON(ctx, http.MethodPost, "/scenarios", nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) UpdateScenario(ctx context.Context, scenarioID uuid.UUID, input scenario.UpdateScenarioInput) (*scenario.Scenario, error) {
	var out scenario.Scenario
	if _, err := c.doJSON(ctx, http.MethodPut, "/scenarios/"+scenarioID.String(), nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) DeleteScenario(ctx context.Context, scenarioID uuid.UUID) error {
	_, err := c.doJSON(ctx, http.MethodDelete, "/scenarios/"+scenarioID.String(), nil, nil, nil)
	return err
}

func (c *APIClient) ListScenarioSteps(ctx context.Context, scenarioID uuid.UUID) ([]scenario.Step, error) {
	var out []scenario.Step
	if _, err := c.doJSON(ctx, http.MethodGet, "/scenarios/"+scenarioID.String()+"/steps", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIClient) UpsertScenarioStep(ctx context.Context, scenarioID uuid.UUID, stepOrder int, input scenario.UpsertStepInput) (*scenario.Step, error) {
	var out scenario.Step
	path := fmt.Sprintf("/scenarios/%s/steps/%d", scenarioID, stepOrder)
	if _, err := c.doJSON(ctx, http.MethodPut, path, nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) DeleteScenarioStep(ctx context.Context, scenarioID, stepID uuid.UUID) error {
	path := fmt.Sprintf("/scenarios/%s/steps/%s", scenarioID, stepID)
	_, err := c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
	return err
}

func (c *APIClient) ReorderScenarioSteps(ctx context.Context, scenarioID uuid.UUID, input scenario.ReorderStepsInput) error {
	path := fmt.Sprintf("/scenarios/%s/steps/reorder", scenarioID)
	_, err := c.doJSON(ctx, http.MethodPut, path, nil, input, nil)
	return err
}

func (c *APIClient) ListServices(ctx context.Context, projectID uuid.UUID) ([]project.Service, error) {
	var out []project.Service
	path := fmt.Sprintf("/projects/%s/services", projectID)
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIClient) ListEnvironments(ctx context.Context, projectID uuid.UUID) ([]project.Environment, error) {
	var out []project.Environment
	path := fmt.Sprintf("/projects/%s/environments", projectID)
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *APIClient) RunCase(ctx context.Context, caseID uuid.UUID, input runner.RunCaseInput) (*runner.RunCaseOutput, error) {
	var out runner.RunCaseOutput
	path := "/cases/" + caseID.String() + "/run"
	if _, err := c.doJSON(ctx, http.MethodPost, path, nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) RunScenario(ctx context.Context, scenarioID uuid.UUID, input runner.RunScenarioInput) (*runner.RunScenarioOutput, error) {
	var out runner.RunScenarioOutput
	path := "/scenarios/" + scenarioID.String() + "/run"
	if _, err := c.doJSON(ctx, http.MethodPost, path, nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *APIClient) ListProjectRuns(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]report.RunListEntry, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		q.Set("offset", fmt.Sprintf("%d", offset))
	}
	path := fmt.Sprintf("/projects/%s/runs", projectID)
	var page struct {
		Items []report.RunListEntry `json:"items"`
	}
	if _, err := c.doJSON(ctx, http.MethodGet, path, q, nil, &page); err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (c *APIClient) GetRunResult(ctx context.Context, runID uuid.UUID) (*runner.RunResultOutput, error) {
	var out runner.RunResultOutput
	if _, err := c.doJSON(ctx, http.MethodGet, "/runs/"+runID.String()+"/result", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func parseAPIError(status int, body []byte) error {
	var er struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &er) == nil && er.Error != "" {
		return fmt.Errorf("API failed (%d): %s", status, er.Error)
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(status)
	}
	return fmt.Errorf("API failed (%d): %s", status, msg)
}

func detectContentType(data []byte) string {
	trim := bytes.TrimSpace(data)
	if len(trim) > 0 && trim[0] == '{' {
		return "application/json"
	}
	return "application/yaml"
}

// RunsPage is the paginated list_runs API response.
type RunsPage struct {
	Items    []report.RunListEntry
	Total    int
	Page     int
	PageSize int
}

// ListRuns lists scenario runs for a project with optional query filters.
func (c *APIClient) ListRuns(ctx context.Context, projectID uuid.UUID, query url.Values) (*RunsPage, error) {
	path := fmt.Sprintf("/projects/%s/runs", projectID)
	var page struct {
		Items    []report.RunListEntry `json:"items"`
		Total    int                   `json:"total"`
		Page     int                   `json:"page"`
		PageSize int                   `json:"pageSize"`
	}
	if _, err := c.doJSON(ctx, http.MethodGet, path, query, nil, &page); err != nil {
		return nil, err
	}
	return &RunsPage{
		Items:    page.Items,
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
	}, nil
}

func (c *APIClient) ListSteps(ctx context.Context, scenarioID uuid.UUID) ([]scenario.Step, error) {
	return c.ListScenarioSteps(ctx, scenarioID)
}

func (c *APIClient) UpsertStep(ctx context.Context, scenarioID uuid.UUID, stepOrder int, input scenario.UpsertStepInput) (*scenario.Step, error) {
	return c.UpsertScenarioStep(ctx, scenarioID, stepOrder, input)
}

func (c *APIClient) DeleteStep(ctx context.Context, scenarioID, stepID uuid.UUID) error {
	return c.DeleteScenarioStep(ctx, scenarioID, stepID)
}

func (c *APIClient) ReorderSteps(ctx context.Context, scenarioID uuid.UUID, input scenario.ReorderStepsInput) error {
	return c.ReorderScenarioSteps(ctx, scenarioID, input)
}

func buildRunsQuery(scenarioID, serviceID uuid.UUID, status string, limit, offset int) url.Values {
	q := url.Values{}
	if scenarioID != uuid.Nil {
		q.Set("scenarioId", scenarioID.String())
	}
	if serviceID != uuid.Nil {
		q.Set("serviceId", serviceID.String())
	}
	if s := strings.TrimSpace(status); s != "" {
		q.Set("status", s)
	}
	if limit > 0 {
		q.Set("pageSize", fmt.Sprintf("%d", limit))
	}
	if limit > 0 && offset > 0 {
		q.Set("page", fmt.Sprintf("%d", offset/limit+1))
	}
	return q
}
