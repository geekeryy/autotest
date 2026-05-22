// Package aianalysis implements the "AI 智能分析" feature: it exposes two HTTP
// endpoints that build a structured context from the platform's run results
// and OpenAPI/Swagger specs, then forward that context to the per-project AI
// provider (via `internal/aiprovider`) for natural-language analysis.
//
// Two actions are supported:
//
//   - analyze_failure       run-result failure analysis (single run, single
//                           or multi-step scenario)
//   - analyze_spec_changes  diff a freshly imported spec against the previous
//                           version on the same service and ask the model to
//                           assess the impact on existing templates and
//                           scenario steps
//
// The package owns its own SQL helpers because both handlers need narrow
// joins (spec by id + previous version, scenario steps for a service) that
// would otherwise pollute single-purpose repositories with one-off queries.
package aianalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"autotest/internal/aiprovider"
	"autotest/internal/aitools"
	"autotest/internal/auth"
	"autotest/internal/httpx"
	"autotest/internal/project"
	"autotest/internal/projectprompt"
	"autotest/internal/report"
	"autotest/internal/specdiff"
	"autotest/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AIChatService is the slice of `aiprovider.Service` we depend on. Defined as
// an interface so handler tests can inject a fake without spinning up a real
// model client.
//
// Both Chat and ChatWithTools are required: ChatWithTools is invoked when
// the handler has a non-empty tool registry attached, otherwise we fall back
// to the legacy single-shot Chat for parity with earlier behaviour.
type AIChatService interface {
	Chat(ctx context.Context, projectID uuid.UUID, req aiprovider.ChatRequest) (*aiprovider.ChatResponse, error)
	ChatWithTools(ctx context.Context, projectID uuid.UUID, req aiprovider.ChatRequest, tools []aitools.Tool) (*aiprovider.ChatResponse, error)
	List(ctx context.Context) ([]aiprovider.Provider, error)
}

// PromptService matches `internal/projectprompt.Service` for the GetByAction
// lookup used to resolve platform-level overrides.
type PromptService interface {
	GetByAction(ctx context.Context, action string) (*projectprompt.ProjectPrompt, error)
}

// ReportService is the slice of `internal/report.Repository` needed by the
// failure-analysis handler.
type ReportService interface {
	GetRun(ctx context.Context, runID uuid.UUID) (*report.Run, error)
	ListResults(ctx context.Context, runID uuid.UUID) ([]report.Result, error)
	LoadScenarioStepIndex(ctx context.Context, scenarioID uuid.UUID) (map[uuid.UUID]report.StepMeta, error)
}

// Handler implements the two AI analysis endpoints.
type Handler struct {
	repo           store.Repository
	ai             AIChatService
	prompts        PromptService
	reports        ReportService
	projectHandler *project.Handler

	// tools is the read-only tool set surfaced to the model on every
	// analyze call. Mutating tools are filtered out by aiprovider.Service
	// for safety, but we keep this slice read-only by convention.
	tools []aitools.Tool
}

// NewHandler constructs the Handler. All collaborators are required; tools
// are optional and may be attached via WithTools.
func NewHandler(repo store.Repository, ai AIChatService, prompts PromptService, reports ReportService, projectHandler *project.Handler) *Handler {
	return &Handler{
		repo:           repo,
		ai:             ai,
		prompts:        prompts,
		reports:        reports,
		projectHandler: projectHandler,
	}
}

// WithTools attaches the read-only AI tool set that smart-analysis actions
// expose to the model. Callers should pass the platform's built-in read-only
// tools; the handler relies on aiprovider.Service to filter out any
// accidentally-included mutating tool.
func (h *Handler) WithTools(tools []aitools.Tool) *Handler {
	h.tools = tools
	return h
}

// dispatch sends the request through ChatWithTools when tools are attached,
// falling back to the single-shot Chat path otherwise. Keeps the two
// analyze handlers identical regardless of whether tool calling is enabled.
//
// Before the tool-enabled path we stamp a CallerContext onto ctx so that
// builtin tools enforcing project isolation (`aitools.ResolveProjectID`)
// know which project this analysis belongs to and reject any cross-
// project tool argument from the model.
func (h *Handler) dispatch(ctx context.Context, projectID uuid.UUID, req aiprovider.ChatRequest) (*aiprovider.ChatResponse, error) {
	if len(h.tools) > 0 {
		principal := auth.PrincipalFromContext(ctx)
		cc := aitools.CallerContext{ProjectID: projectID}
		if principal != nil {
			cc.UserID = principal.UserID
			cc.Username = principal.Username
		}
		ctx = aitools.WithCaller(ctx, cc)
		return h.ai.ChatWithTools(ctx, projectID, req, h.tools)
	}
	return h.ai.Chat(ctx, projectID, req)
}

// Register mounts the two endpoints. The run-failure endpoint sits in the
// global protected group (already runs Authenticate + RejectAPIKey) because
// it is keyed by runID, not project. The spec-changes endpoint is project
// scoped via RequireProjectRole(viewer) so we get the membership check + role
// context that other spec routes already enjoy.
func (h *Handler) Register(r chi.Router) {
	r.Post("/runs/{runID}/ai/analyze-failure", h.analyzeFailure)

	r.Route("/projects/{projectID}/services/{serviceID}/specs/{specID}/ai", func(r chi.Router) {
		r.Use(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer))
		r.Post("/analyze-changes", h.analyzeSpecChanges)
	})
}

// ── analyze_failure ────────────────────────────────────────────────────────

func (h *Handler) analyzeFailure(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 runId"))
		return
	}

	run, err := h.reports.GetRun(r.Context(), runID)
	if err != nil {
		if errors.Is(err, report.ErrRunNotFound) {
			httpx.Error(w, http.StatusNotFound, errors.New("找不到指定运行记录"))
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	results, err := h.reports.ListResults(r.Context(), runID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	stepNamesByID := map[uuid.UUID]report.StepMeta{}
	if run.ScenarioID != nil && *run.ScenarioID != uuid.Nil {
		stepNamesByID, err = h.loadScenarioStepIndex(r.Context(), *run.ScenarioID)
		if err != nil {
			// Step-name lookup is best-effort — log via header-less default
			// and continue with bare step IDs so the analysis can still run.
			stepNamesByID = map[uuid.UUID]report.StepMeta{}
		}
	}

	ctx := buildFailureContext(run, results, stepNamesByID)
	rawCtx, err := json.Marshal(ctx)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, fmt.Errorf("encode failure context: %w", err))
		return
	}

	providerID, model, systemPrompt, cfgErr := resolveAIConfig(r.Context(), h.prompts, h.ai, run.ProjectID, aiprovider.ActionAnalyzeFailure)
	if cfgErr != nil {
		httpx.Error(w, http.StatusBadRequest, cfgErr)
		return
	}

	resp, err := h.dispatch(r.Context(), run.ProjectID, aiprovider.ChatRequest{
		ProviderID:           providerID,
		Action:               aiprovider.ActionAnalyzeFailure,
		Context:              rawCtx,
		Model:                model,
		SystemPromptOverride: systemPrompt,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) loadScenarioStepIndex(ctx context.Context, scenarioID uuid.UUID) (map[uuid.UUID]report.StepMeta, error) {
	return h.reports.LoadScenarioStepIndex(ctx, scenarioID)
}

// buildFailureContext assembles the structured payload sent to the model. We
// trim large response bodies to keep prompt size manageable while still
// retaining enough evidence for root-cause analysis.
func buildFailureContext(run *report.Run, results []report.Result, stepIndex map[uuid.UUID]report.StepMeta) map[string]any {
	ctx := map[string]any{
		"run": map[string]any{
			"id":            run.ID,
			"name":          run.Name,
			"status":        string(run.Status),
			"projectId":     run.ProjectID,
			"serviceId":     run.ServiceID,
			"scenarioId":    run.ScenarioID,
			"environmentId": run.EnvironmentID,
		},
	}

	if len(run.Snapshot) > 0 {
		var snap any
		if err := json.Unmarshal(run.Snapshot, &snap); err == nil {
			ctx["runSnapshot"] = snap
		}
	}

	stepResults := make([]map[string]any, 0, len(results))
	assertionFailures := []map[string]any{}
	for _, result := range results {
		entry := report.BuildResultSummaryEntry(result, stepIndex, report.DefaultSummaryMaxBodyChars)
		stepResults = append(stepResults, entry)
		assertionFailures = append(assertionFailures, report.ExtractAssertionFailures(result)...)
	}

	if run.ScenarioID != nil && *run.ScenarioID != uuid.Nil {
		ctx["scenario"] = map[string]any{
			"id": *run.ScenarioID,
		}
		ctx["stepResults"] = stepResults
	} else if len(stepResults) > 0 {
		// Single-case run: surface the first result under `result` for the
		// model to focus on, while still keeping the full list available.
		ctx["result"] = stepResults[0]
		if len(stepResults) > 1 {
			ctx["additionalResults"] = stepResults[1:]
		}
	}
	if len(assertionFailures) > 0 {
		ctx["assertionFailures"] = assertionFailures
	}
	return ctx
}

// ── analyze_spec_changes ───────────────────────────────────────────────────

// analyzeSpecChangesNoPrev is the response payload returned when the supplied
// spec is the only version on the service: the diff is meaningless and we
// would rather give the user a clear Chinese explanation than ask the model
// to "compare with nothing".
type analyzeSpecChangesNoPrev struct {
	Message string `json:"message"`
}

func (h *Handler) analyzeSpecChanges(w http.ResponseWriter, r *http.Request) {
	projectID, serviceID, specID, ok := parseSpecRouteIDs(w, r)
	if !ok {
		return
	}

	currSpec, err := h.fetchSpec(r.Context(), projectID, serviceID, specID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, http.StatusNotFound, errors.New("找不到指定 spec 记录"))
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	prevSpec, err := h.fetchPreviousSpec(r.Context(), projectID, serviceID, currSpec.Version)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if prevSpec == nil {
		httpx.JSON(w, http.StatusOK, analyzeSpecChangesNoPrev{Message: "暂无前一版本可对比，无需变更分析"})
		return
	}

	diff, err := specdiff.DiffSnapshots(prevSpec.NormalizedSnapshot, currSpec.NormalizedSnapshot)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	templates, err := h.fetchTemplatesForService(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	steps, err := h.fetchScenarioStepsForService(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	affectedTemplates := filterAffectedTemplates(diff, templates)
	affectedSteps := filterAffectedScenarioSteps(diff, steps)

	ctx := map[string]any{
		// projectId / serviceId 顶层暴露便于 AI 在 tool 调用时无需从嵌套结构里推断。
		"projectId": projectID,
		"serviceId": serviceID,
		"service":   map[string]any{"id": serviceID},
		"prevSpec": map[string]any{
			"id":          prevSpec.ID,
			"version":     prevSpec.Version,
			"contentHash": prevSpec.ContentHash,
			"createdAt":   prevSpec.CreatedAt,
		},
		"currSpec": map[string]any{
			"id":          currSpec.ID,
			"version":     currSpec.Version,
			"contentHash": currSpec.ContentHash,
			"createdAt":   currSpec.CreatedAt,
		},
		"diff":                  diff,
		"affectedTemplates":     affectedTemplates,
		"affectedScenarioSteps": affectedSteps,
	}
	rawCtx, err := json.Marshal(ctx)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	providerID, model, systemPrompt, cfgErr := resolveAIConfig(r.Context(), h.prompts, h.ai, projectID, aiprovider.ActionAnalyzeSpecChanges)
	if cfgErr != nil {
		httpx.Error(w, http.StatusBadRequest, cfgErr)
		return
	}

	resp, err := h.dispatch(r.Context(), projectID, aiprovider.ChatRequest{
		ProviderID:           providerID,
		Action:               aiprovider.ActionAnalyzeSpecChanges,
		Context:              rawCtx,
		Model:                model,
		SystemPromptOverride: systemPrompt,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// specRow is a local mirror of the `api_specs` row, just enough for diffing
// and prompt context. Avoids introducing a new method on `spec.Repository`
// for what is essentially a one-off projection.
type specRow struct {
	ID                 uuid.UUID
	ServiceID          uuid.UUID
	Version            int
	ContentHash        string
	NormalizedSnapshot json.RawMessage
	Status             string
	CreatedAt          time.Time
}

func (h *Handler) fetchSpec(ctx context.Context, projectID, serviceID, specID uuid.UUID) (*specRow, error) {
	if h.repo.DB == nil {
		return nil, errors.New("database unavailable")
	}
	row := h.repo.DB.QueryRow(ctx, `
		select sp.id, sp.service_id, sp.version, sp.content_hash,
		       sp.normalized_snapshot, sp.status, sp.created_at
		from api_specs sp
		join services s on s.id = sp.service_id and s.deleted_at is null
		join projects p on p.id = s.project_id and p.deleted_at is null
		where p.id = $1 and sp.service_id = $2 and sp.id = $3 and sp.deleted_at is null
	`, projectID, serviceID, specID)
	var out specRow
	if err := row.Scan(&out.ID, &out.ServiceID, &out.Version, &out.ContentHash, &out.NormalizedSnapshot, &out.Status, &out.CreatedAt); err != nil {
		return nil, err
	}
	return &out, nil
}

func (h *Handler) fetchPreviousSpec(ctx context.Context, projectID, serviceID uuid.UUID, currentVersion int) (*specRow, error) {
	if h.repo.DB == nil {
		return nil, errors.New("database unavailable")
	}
	row := h.repo.DB.QueryRow(ctx, `
		select sp.id, sp.service_id, sp.version, sp.content_hash,
		       sp.normalized_snapshot, sp.status, sp.created_at
		from api_specs sp
		join services s on s.id = sp.service_id and s.deleted_at is null
		join projects p on p.id = s.project_id and p.deleted_at is null
		where p.id = $1 and sp.service_id = $2 and sp.version < $3 and sp.deleted_at is null
		order by sp.version desc
		limit 1
	`, projectID, serviceID, currentVersion)
	var out specRow
	if err := row.Scan(&out.ID, &out.ServiceID, &out.Version, &out.ContentHash, &out.NormalizedSnapshot, &out.Status, &out.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

// templateRow is the projection of `test_cases` rows used when assessing
// impact: just enough for the AI to refer to the template by name and path.
type templateRow struct {
	CaseID uuid.UUID `json:"caseId"`
	Name   string    `json:"name"`
	Method string    `json:"method"`
	Path   string    `json:"path"`
	Source string    `json:"source"`
}

func (h *Handler) fetchTemplatesForService(ctx context.Context, projectID, serviceID uuid.UUID) ([]templateRow, error) {
	if h.repo.DB == nil {
		return nil, errors.New("database unavailable")
	}
	rows, err := h.repo.DB.Query(ctx, `
		select tc.id, tc.name, tc.method, tc.path, tc.source
		from test_cases tc
		where tc.project_id = $1 and tc.service_id = $2
		  and tc.deleted_at is null and tc.source != 'derived'
	`, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("查询接口模板失败: %w", err)
	}
	defer rows.Close()
	out := []templateRow{}
	for rows.Next() {
		var t templateRow
		if err := rows.Scan(&t.CaseID, &t.Name, &t.Method, &t.Path, &t.Source); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// scenarioStepRow is the projection of `test_scenario_steps` joined to the
// referenced template, used to flag scenario steps whose endpoint changed.
type scenarioStepRow struct {
	ScenarioID   uuid.UUID  `json:"scenarioId"`
	ScenarioName string     `json:"scenarioName"`
	StepID       uuid.UUID  `json:"stepId"`
	StepSeq      int        `json:"stepSeq"`
	StepName     string     `json:"stepName,omitempty"`
	TestCaseID   *uuid.UUID `json:"testCaseId,omitempty"`
	Method       string     `json:"method"`
	Path         string     `json:"path"`
}

func (h *Handler) fetchScenarioStepsForService(ctx context.Context, projectID, serviceID uuid.UUID) ([]scenarioStepRow, error) {
	if h.repo.DB == nil {
		return nil, errors.New("database unavailable")
	}
	rows, err := h.repo.DB.Query(ctx, `
		select sc.id, sc.name, st.id, st.step_seq, st.name, st.test_case_id,
		       coalesce(tc.method, ''), coalesce(tc.path, '')
		from test_scenario_steps st
		join test_scenarios sc on sc.id = st.scenario_id and sc.deleted_at is null
		left join test_cases tc on tc.id = st.test_case_id and tc.deleted_at is null
		where sc.project_id = $1 and sc.service_id = $2
		  and st.deleted_at is null and st.step_type = 'api'
	`, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("查询场景步骤失败: %w", err)
	}
	defer rows.Close()
	out := []scenarioStepRow{}
	for rows.Next() {
		var s scenarioStepRow
		if err := rows.Scan(&s.ScenarioID, &s.ScenarioName, &s.StepID, &s.StepSeq, &s.StepName, &s.TestCaseID, &s.Method, &s.Path); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// endpointKey is the `METHOD path` key used by specdiff. Helpers below build
// the same key for templates and scenario steps so we can intersect.
func endpointKey(method, path string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	if method == "" || path == "" {
		return ""
	}
	return method + " " + path
}

func diffEndpointKeys(diff specdiff.Diff) map[string]struct{} {
	out := map[string]struct{}{}
	for _, ep := range diff.AddedEndpoints {
		if k := endpointKey(ep.Method, ep.Path); k != "" {
			out[k] = struct{}{}
		}
	}
	for _, ep := range diff.RemovedEndpoints {
		if k := endpointKey(ep.Method, ep.Path); k != "" {
			out[k] = struct{}{}
		}
	}
	for _, ep := range diff.ModifiedEndpoints {
		if k := endpointKey(ep.Method, ep.Path); k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

func filterAffectedTemplates(diff specdiff.Diff, templates []templateRow) []templateRow {
	keys := diffEndpointKeys(diff)
	if len(keys) == 0 {
		return []templateRow{}
	}
	out := []templateRow{}
	for _, t := range templates {
		if _, ok := keys[endpointKey(t.Method, t.Path)]; ok {
			out = append(out, t)
		}
	}
	return out
}

func filterAffectedScenarioSteps(diff specdiff.Diff, steps []scenarioStepRow) []scenarioStepRow {
	keys := diffEndpointKeys(diff)
	if len(keys) == 0 {
		return []scenarioStepRow{}
	}
	out := []scenarioStepRow{}
	for _, s := range steps {
		if _, ok := keys[endpointKey(s.Method, s.Path)]; ok {
			out = append(out, s)
		}
	}
	return out
}

// ── shared helpers ─────────────────────────────────────────────────────────

func parseSpecRouteIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 projectId"))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	serviceID, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 serviceId"))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	specID, err := uuid.Parse(chi.URLParam(r, "specID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 specId"))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return projectID, serviceID, specID, true
}

// resolveAIConfig combines project-level prompt overrides with the project
// default AI provider in the same fallback order used by `internal/testdata`:
//
//  1. If the prompt override is enabled and pins a `providerId`, use it.
//  2. Otherwise pick the project's default provider (`isDefault && enabled`)
//     or the first enabled provider as a soft fallback.
//  3. Return a clear Chinese error when nothing is configured so the
//     frontend can surface a helpful message instead of a generic 500.
//
// `defaultModel` and `systemPrompt` come from the prompt override when
// present so admins can pin a model + override the prompt without touching
// per-call payloads.
func resolveAIConfig(ctx context.Context, prompts PromptService, ai AIChatService, projectID uuid.UUID, action string) (uuid.UUID, string, string, error) {
	var cfg *projectprompt.ProjectPrompt
	if prompts != nil {
		got, err := prompts.GetByAction(ctx, action)
		if err == nil && got != nil {
			cfg = got
		}
	}

	var providerID uuid.UUID
	model := ""
	systemPrompt := ""
	if cfg != nil {
		if cfg.ProviderID != nil && *cfg.ProviderID != uuid.Nil {
			providerID = *cfg.ProviderID
		}
		model = strings.TrimSpace(cfg.DefaultModel)
		if cfg.Enabled {
			systemPrompt = cfg.SystemPrompt
		}
	}

	if providerID == uuid.Nil {
		fallback, err := lookupDefaultProvider(ctx, ai)
		if err != nil {
			return uuid.Nil, "", "", err
		}
		providerID = fallback
	}
	if providerID == uuid.Nil {
		return uuid.Nil, "", "", fmt.Errorf("请先在「平台资源 / Prompt 管理」或 AI 提供商中配置 %s 的提供商", action)
	}
	return providerID, model, systemPrompt, nil
}

func lookupDefaultProvider(ctx context.Context, ai AIChatService) (uuid.UUID, error) {
	if ai == nil {
		return uuid.Nil, errors.New("AI provider 服务不可用")
	}
	providers, err := ai.List(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("查询平台默认 AI 提供商失败: %w", err)
	}
	var firstEnabled uuid.UUID
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		if p.IsDefault {
			return p.ID, nil
		}
		if firstEnabled == uuid.Nil {
			firstEnabled = p.ID
		}
	}
	return firstEnabled, nil
}
