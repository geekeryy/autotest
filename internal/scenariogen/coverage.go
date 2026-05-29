package scenariogen

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	testcase "autotest/internal/case"
	"autotest/internal/generator"
	"autotest/internal/scenario"
	"autotest/internal/spec"
	"autotest/internal/sampler"

	"github.com/google/uuid"
)

// StepPlan is one API step in a generated scenario.
type StepPlan struct {
	Name            string
	Endpoint        spec.Endpoint
	CaseID          uuid.UUID
	RequestOverride json.RawMessage
	ExpectStatus    int
}

// ScenarioPlan groups steps into one runnable scenario.
type ScenarioPlan struct {
	Name        string
	Description string
	Steps       []StepPlan
}

// CoverageResult summarizes generation output.
type CoverageResult struct {
	Scenarios      []CreatedScenario `json:"scenarios"`
	CasesCreated   int               `json:"casesCreated"`
	EndpointsTotal int               `json:"endpointsTotal"`
}

// CreatedScenario is one scenario persisted by the generator.
type CreatedScenario struct {
	ScenarioID uuid.UUID      `json:"scenarioId"`
	Name       string         `json:"name"`
	StepCount  int            `json:"stepCount"`
	Scenario   *scenario.Scenario `json:"scenario,omitempty"`
	Steps      []scenario.Step    `json:"steps,omitempty"`
}

// Generator builds multi-scenario coverage plans and persists them.
type Generator struct {
	deps Deps
}

func NewGenerator(deps Deps) *Generator {
	gen := deps.Generator
	if gen == nil {
		gen = generator.NewDefault()
	}
	deps.Generator = gen
	return &Generator{deps: deps}
}

// GenerateCoverage ensures request templates exist, plans scenarios, and creates them.
func (g *Generator) GenerateCoverage(ctx context.Context, projectID, serviceID uuid.UUID) (*CoverageResult, error) {
	if g.deps.Specs == nil || g.deps.Cases == nil || g.deps.Scenarios == nil {
		return nil, fmt.Errorf("scenariogen: 依赖未配置")
	}
	endpoints, err := g.deps.Specs.ListEndpoints(ctx, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("当前服务下没有可导入的接口，请先导入 OpenAPI/Swagger")
	}

	caseMap, created, err := g.ensureCases(ctx, projectID, serviceID, endpoints)
	if err != nil {
		return nil, err
	}

	plans := planCoverage(endpoints, caseMap)
	result := &CoverageResult{
		CasesCreated:   created,
		EndpointsTotal: len(endpoints),
	}
	for _, plan := range plans {
		createdSc, err := g.createScenario(ctx, projectID, serviceID, plan)
		if err != nil {
			return nil, fmt.Errorf("create scenario %q: %w", plan.Name, err)
		}
		result.Scenarios = append(result.Scenarios, *createdSc)
	}
	return result, nil
}

func (g *Generator) ensureCases(ctx context.Context, projectID, serviceID uuid.UUID, endpoints []spec.Endpoint) (map[uuid.UUID]uuid.UUID, int, error) {
	existing, err := g.deps.Cases.List(ctx, testcase.ListFilter{ProjectID: projectID, ServiceID: serviceID})
	if err != nil {
		return nil, 0, err
	}
	byEndpoint := map[uuid.UUID]uuid.UUID{}
	byKey := map[string]uuid.UUID{}
	for _, tc := range existing {
		if tc.EndpointID != nil {
			byEndpoint[*tc.EndpointID] = tc.ID
		}
		key := strings.ToUpper(tc.Method) + " " + tc.Path
		if _, ok := byKey[key]; !ok {
			byKey[key] = tc.ID
		}
	}

	created := 0
	for _, ep := range endpoints {
		if _, ok := byEndpoint[ep.ID]; ok {
			continue
		}
		key := strings.ToUpper(ep.Method) + " " + ep.Path
		if id, ok := byKey[key]; ok {
			byEndpoint[ep.ID] = id
			continue
		}
		tc, err := g.createCaseFromEndpoint(ctx, projectID, serviceID, ep)
		if err != nil {
			return nil, 0, fmt.Errorf("%s %s: %w", ep.Method, ep.Path, err)
		}
		byEndpoint[ep.ID] = tc.ID
		byKey[key] = tc.ID
		created++
	}
	return byEndpoint, created, nil
}

func (g *Generator) createCaseFromEndpoint(ctx context.Context, projectID, serviceID uuid.UUID, ep spec.Endpoint) (*testcase.TestCase, error) {
	gep := generator.Endpoint{
		ID:             ep.ID,
		ProjectID:      projectID,
		ServiceID:      serviceID,
		Method:         ep.Method,
		Path:           ep.Path,
		OperationID:    ep.OperationID,
		Summary:        ep.Summary,
		RequestSchema:  ep.RequestSchema,
		ResponseSchema: ep.ResponseSchema,
	}
	drafts, err := g.deps.Generator.Generate(gep)
	if err != nil {
		return nil, err
	}
	if len(drafts) == 0 {
		sample := sampler.FromSchema(ep.RequestSchema)
		req := map[string]any{
			"method":    strings.ToUpper(ep.Method),
			"path":      ep.Path,
			"headers":   sample.Headers,
			"query":     sample.Query,
			"variables": sample.Path,
			"body":      sample.Body,
		}
		if sample.Security != nil {
			req["security"] = sample.Security
		}
		rawReq, _ := json.Marshal(req)
		tc, err := g.deps.Cases.CreateManual(ctx, testcase.CreateManualInput{
			ProjectID:  projectID,
			ServiceID:  serviceID,
			EndpointID: &ep.ID,
			Name:       caseName(ep),
			Method:     ep.Method,
			Path:       ep.Path,
			Request:    rawReq,
			Assertions: defaultAssertions(),
		})
		return tc, err
	}
	tc, err := g.deps.Cases.UpsertGenerated(ctx, drafts[0])
	return tc, err
}

func boolPtr(v bool) *bool {
	return &v
}

func caseName(ep spec.Endpoint) string {
	if ep.Summary != "" {
		return ep.Summary
	}
	if ep.OperationID != "" {
		return ep.OperationID
	}
	return strings.ToUpper(ep.Method) + " " + ep.Path
}

func planCoverage(endpoints []spec.Endpoint, caseMap map[uuid.UUID]uuid.UUID) []ScenarioPlan {
	if looksLikeE2EAPI(endpoints) {
		return planE2ECoverage(endpoints, caseMap)
	}
	return planGenericCoverage(endpoints, caseMap)
}

func looksLikeE2EAPI(endpoints []spec.Endpoint) bool {
	hasUserLogin, hasAdminLogin, hasHealth := false, false, false
	for _, ep := range endpoints {
		p := ep.Path
		switch {
		case ep.Method == "POST" && p == "/api/v1/auth/login":
			hasUserLogin = true
		case ep.Method == "POST" && p == "/api/v1/admin/auth/login":
			hasAdminLogin = true
		case ep.Method == "GET" && p == "/healthz":
			hasHealth = true
		}
	}
	return hasUserLogin && hasAdminLogin && hasHealth
}

func planE2ECoverage(endpoints []spec.Endpoint, caseMap map[uuid.UUID]uuid.UUID) []ScenarioPlan {
	byPath := indexEndpoints(endpoints)
	var plans []ScenarioPlan

	// 1) Health
	if ep, ok := byPath["GET /healthz"]; ok {
		plans = append(plans, ScenarioPlan{
			Name:        "健康检查",
			Description: "无鉴权公共接口",
			Steps: []StepPlan{{
				Name:         "健康检查",
				Endpoint:     ep,
				CaseID:       caseMap[ep.ID],
				ExpectStatus: 200,
			}},
		})
	}

	// 2) User API flow
	userSteps := []StepPlan{}
	userLoginSeq := 1
	if ep, ok := byPath["POST /api/v1/auth/login"]; ok {
		userSteps = append(userSteps, StepPlan{
			Name:            "用户登录",
			Endpoint:        ep,
			CaseID:          caseMap[ep.ID],
			RequestOverride: loginRequestOverride("admin", "admin123"),
			ExpectStatus:    200,
		})
	}
	if ep, ok := byPath["GET /api/v1/me"]; ok && len(userSteps) > 0 {
		userSteps = append(userSteps, StepPlan{
			Name:            "获取当前用户",
			Endpoint:        ep,
			CaseID:          caseMap[ep.ID],
			RequestOverride: bearerAuthOverride(userLoginSeq),
			ExpectStatus:    200,
		})
	}
	listUsersSeq := len(userSteps) + 1
	if ep, ok := byPath["GET /api/v1/users"]; ok && len(userSteps) > 0 {
		userSteps = append(userSteps, StepPlan{
			Name:            "用户列表",
			Endpoint:        ep,
			CaseID:          caseMap[ep.ID],
			RequestOverride: bearerAuthOverride(userLoginSeq),
			ExpectStatus:    200,
		})
	}
	if ep, ok := byPath["GET /api/v1/users/{id}"]; ok && len(userSteps) >= 2 {
		userSteps = append(userSteps, StepPlan{
			Name:            "用户详情",
			Endpoint:        ep,
			CaseID:          caseMap[ep.ID],
			RequestOverride: mergeOverrides(
				bearerAuthOverride(userLoginSeq),
				pathIDOverride(listUsersSeq, "body[0].id"),
			),
			ExpectStatus: 200,
		})
	}
	if len(userSteps) > 0 {
		plans = append(plans, ScenarioPlan{
			Name:        "用户 API 全流程",
			Description: "登录后访问用户只读接口",
			Steps:       userSteps,
		})
	}

	// 3) Admin API flow
	adminSteps := []StepPlan{}
	adminLoginSeq := 1
	if ep, ok := byPath["POST /api/v1/admin/auth/login"]; ok {
		adminSteps = append(adminSteps, StepPlan{
			Name:            "管理员登录",
			Endpoint:        ep,
			CaseID:          caseMap[ep.ID],
			RequestOverride: loginRequestOverride("admin-root", "admin123"),
			ExpectStatus:    200,
		})
	}
	for _, item := range []struct {
		method, path, name string
	}{
		{"GET", "/api/v1/admin/stats", "平台统计"},
		{"GET", "/api/v1/admin/audit-logs", "审计日志"},
		{"GET", "/api/v1/admin/users", "管理员用户列表"},
	} {
		if ep, ok := byPath[item.method+" "+item.path]; ok && len(adminSteps) > 0 {
			adminSteps = append(adminSteps, StepPlan{
				Name:            item.name,
				Endpoint:        ep,
				CaseID:          caseMap[ep.ID],
				RequestOverride: bearerAuthOverride(adminLoginSeq),
				ExpectStatus:    200,
			})
		}
	}
	createSeq := len(adminSteps) + 1
	if ep, ok := byPath["POST /api/v1/admin/users"]; ok && len(adminSteps) > 0 {
		adminSteps = append(adminSteps, StepPlan{
			Name:     "创建用户",
			Endpoint: ep,
			CaseID:   caseMap[ep.ID],
			RequestOverride: mergeOverrides(
				bearerAuthOverride(adminLoginSeq),
				jsonRequestOverride(map[string]any{
					"body": map[string]any{
						"name":  "E2E Generated User",
						"email": "{{$mock.email}}",
						"role":  "tester",
					},
				}),
			),
			ExpectStatus: 201,
		})
	}
	for _, item := range []struct {
		method, path, name string
		status             int
	}{
		{"GET", "/api/v1/admin/users/{id}", "查询新建用户", 200},
		{"PUT", "/api/v1/admin/users/{id}", "更新新建用户", 200},
		{"DELETE", "/api/v1/admin/users/{id}", "删除新建用户", 204},
	} {
		if ep, ok := byPath[item.method+" "+item.path]; ok && len(adminSteps) >= 2 {
			adminSteps = append(adminSteps, StepPlan{
				Name:     item.name,
				Endpoint: ep,
				CaseID:   caseMap[ep.ID],
				RequestOverride: mergeOverrides(
					bearerAuthOverride(adminLoginSeq),
					pathIDOverride(createSeq, "body.id"),
				),
				ExpectStatus: item.status,
			})
		}
	}
	if len(adminSteps) > 0 {
		plans = append(plans, ScenarioPlan{
			Name:        "管理员 API 全流程",
			Description: "管理员登录后完成统计、审计与用户 CRUD",
			Steps:       adminSteps,
		})
	}

	return plans
}

func planGenericCoverage(endpoints []spec.Endpoint, caseMap map[uuid.UUID]uuid.UUID) []ScenarioPlan {
	groups := map[string][]spec.Endpoint{}
	for _, ep := range endpoints {
		tag := "default"
		if len(ep.Tags) > 0 && strings.TrimSpace(ep.Tags[0]) != "" {
			tag = ep.Tags[0]
		}
		groups[tag] = append(groups[tag], ep)
	}
	tags := make([]string, 0, len(groups))
	for t := range groups {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	var plans []ScenarioPlan
	for _, tag := range tags {
		eps := groups[tag]
		sort.Slice(eps, func(i, j int) bool {
			a, b := eps[i], eps[j]
			if priority(a) != priority(b) {
				return priority(a) < priority(b)
			}
			if a.Path == b.Path {
				return a.Method < b.Method
			}
			return a.Path < b.Path
		})

		steps := make([]StepPlan, 0, len(eps))
		loginSeq := 0
		for i, ep := range eps {
			step := StepPlan{
				Name:         caseName(ep),
				Endpoint:     ep,
				CaseID:       caseMap[ep.ID],
				ExpectStatus: expectedStatus(ep),
			}
			if isLoginEndpoint(ep) {
				loginSeq = i + 1
				step.RequestOverride = loginBodyForEndpoint(ep)
			} else if loginSeq > 0 && needsBearer(ep) {
				step.RequestOverride = bearerAuthOverride(loginSeq)
			}
			steps = append(steps, step)
		}
		if len(steps) == 0 {
			continue
		}
		plans = append(plans, ScenarioPlan{
			Name:        fmt.Sprintf("%s 接口覆盖", tag),
			Description: fmt.Sprintf("按标签 %s 自动生成，共 %d 个接口步骤", tag, len(steps)),
			Steps:       steps,
		})
	}
	return plans
}

func priority(ep spec.Endpoint) int {
	if isLoginEndpoint(ep) {
		return 0
	}
	if !needsBearer(ep) {
		return 1
	}
	if ep.Method == "GET" {
		return 2
	}
	if ep.Method == "POST" {
		return 3
	}
	return 4
}

func isLoginEndpoint(ep spec.Endpoint) bool {
	if ep.Method != "POST" {
		return false
	}
	p := strings.ToLower(ep.Path)
	return strings.HasSuffix(p, "/login") || strings.HasSuffix(p, "/auth/login")
}

func needsBearer(ep spec.Endpoint) bool {
	var req map[string]any
	if err := json.Unmarshal(ep.RequestSchema, &req); err != nil {
		return false
	}
	sec, _ := req["security"].([]any)
	return len(sec) > 0
}

func loginBodyForEndpoint(ep spec.Endpoint) json.RawMessage {
	if strings.Contains(ep.Path, "/admin/") {
		return loginRequestOverride("admin-root", "admin123")
	}
	return loginRequestOverride("admin", "admin123")
}

func expectedStatus(ep spec.Endpoint) int {
	if ep.Method == "POST" {
		return 201
	}
	if ep.Method == "DELETE" {
		return 204
	}
	return 200
}

func indexEndpoints(endpoints []spec.Endpoint) map[string]spec.Endpoint {
	out := make(map[string]spec.Endpoint, len(endpoints))
	for _, ep := range endpoints {
		out[strings.ToUpper(ep.Method)+" "+ep.Path] = ep
	}
	return out
}

func mergeOverrides(parts ...json.RawMessage) json.RawMessage {
	headers := map[string]string{}
	variables := map[string]string{}
	var body any
	for _, raw := range parts {
		if len(raw) == 0 {
			continue
		}
		var chunk map[string]any
		if json.Unmarshal(raw, &chunk) != nil {
			continue
		}
		if h, ok := chunk["headers"].(map[string]any); ok {
			for k, v := range h {
				headers[k] = fmt.Sprint(v)
			}
		}
		if h, ok := chunk["headers"].(map[string]string); ok {
			for k, v := range h {
				headers[k] = v
			}
		}
		if v, ok := chunk["variables"].(map[string]any); ok {
			for k, val := range v {
				variables[k] = fmt.Sprint(val)
			}
		}
		if v, ok := chunk["variables"].(map[string]string); ok {
			for k, val := range v {
				variables[k] = val
			}
		}
		if b, ok := chunk["body"]; ok {
			body = b
		}
	}
	out := map[string]any{}
	if len(headers) > 0 {
		out["headers"] = headers
	}
	if len(variables) > 0 {
		out["variables"] = variables
	}
	if body != nil {
		out["body"] = body
	}
	if len(out) == 0 {
		return nil
	}
	raw, _ := json.Marshal(out)
	return raw
}

func (g *Generator) createScenario(ctx context.Context, projectID, serviceID uuid.UUID, plan ScenarioPlan) (*CreatedScenario, error) {
	sc, err := g.deps.Scenarios.Create(ctx, scenario.CreateScenarioInput{
		ProjectID:   projectID,
		ServiceID:   serviceID,
		Name:        plan.Name,
		Description: plan.Description,
	})
	if err != nil {
		return nil, err
	}

	created := make([]scenario.Step, 0, len(plan.Steps))
	for i, sp := range plan.Steps {
		var cfg json.RawMessage
		if sp.ExpectStatus != 0 && sp.ExpectStatus != 200 {
			cfg, _ = json.Marshal(map[string]any{
				"assertions": []map[string]any{{"type": "status_code", "expected": sp.ExpectStatus}},
			})
		}
		input := scenario.UpsertStepInput{
			StepOrder:  i + 1,
			StepType:   scenario.StepTypeAPI,
			Name:       sp.Name,
			Enabled:    boolPtr(true),
			TestCaseID: sp.CaseID,
			Config:     cfg,
		}
		if len(sp.RequestOverride) > 0 {
			input.RequestOverride = sp.RequestOverride
		}
		step, err := g.deps.Scenarios.UpsertStep(ctx, sc.ID, input)
		if err != nil {
			return nil, err
		}
		created = append(created, *step)
	}
	return &CreatedScenario{
		ScenarioID: sc.ID,
		Name:       sc.Name,
		StepCount:  len(created),
		Scenario:   sc,
		Steps:      created,
	}, nil
}
