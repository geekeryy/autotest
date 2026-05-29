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
	"autotest/internal/scenariogen/depgraph"
	"autotest/internal/spec"
	"autotest/internal/sampler"

	"github.com/google/uuid"
)

// StepPlan is one API step in a generated scenario.
type StepPlan struct {
	Name            string
	Endpoint        spec.Endpoint
	CaseID          uuid.UUID
	Config          json.RawMessage
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
	ScenarioID uuid.UUID          `json:"scenarioId"`
	Name       string             `json:"name"`
	StepCount  int                `json:"stepCount"`
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
func (g *Generator) GenerateCoverage(ctx context.Context, projectID, serviceID uuid.UUID, opts *CoverageOptions) (*CoverageResult, error) {
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
	listedCases, err := g.deps.Cases.List(ctx, testcase.ListFilter{ProjectID: projectID, ServiceID: serviceID})
	if err != nil {
		return nil, err
	}

	plans := planCoverage(endpoints, buildPlanContext(caseMap, listedCases, opts))
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

func buildPlanContext(caseMap map[uuid.UUID]uuid.UUID, cases []testcase.TestCase, opts *CoverageOptions) planContext {
	ctx := planContext{caseMap: caseMap, caseRequest: map[uuid.UUID]json.RawMessage{}}
	for _, tc := range cases {
		ctx.caseRequest[tc.ID] = tc.Request
	}
	if opts != nil {
		ctx.creds = opts.LoginCredentials
	}
	return ctx
}

type planContext struct {
	caseMap     map[uuid.UUID]uuid.UUID
	caseRequest map[uuid.UUID]json.RawMessage
	creds       *LoginCredentialBundle
}

func planCoverage(endpoints []spec.Endpoint, pctx planContext) []ScenarioPlan {
	graph := depgraph.Build(endpoints)
	var plans []ScenarioPlan

	// Public / health endpoints without auth in a dedicated scenario.
	var publicSteps []StepPlan
	publicIdx := map[int]bool{}
	for _, grp := range graph.Groups {
		if grp.HasAuth {
			continue
		}
		for _, idx := range grp.Endpoint {
			publicIdx[idx] = true
			sp := stepFromEndpoint(graph, idx, pctx, map[int]int{})
			publicSteps = append(publicSteps, sp)
		}
	}
	if len(publicSteps) > 0 {
		plans = append(plans, ScenarioPlan{
			Name:        "公共接口",
			Description: "无鉴权接口自动覆盖",
			Steps:       publicSteps,
		})
	}

	for _, grp := range graph.Groups {
		if !grp.HasAuth {
			continue
		}
		steps := planGroupSteps(graph, grp, pctx, publicIdx)
		if len(steps) == 0 {
			continue
		}
		plans = append(plans, ScenarioPlan{
			Name:        fmt.Sprintf("%s 接口覆盖", grp.Name),
			Description: fmt.Sprintf("依赖图驱动，按标签 %s 自动生成 %d 个步骤", grp.Name, len(steps)),
			Steps:       steps,
		})
	}
	if len(plans) == 0 {
		// fallback: single scenario in topo order
		stepSeq := map[int]int{}
		var steps []StepPlan
		for i, idx := range graph.TopoOrder {
			stepSeq[idx] = i + 1
			steps = append(steps, stepFromEndpoint(graph, idx, pctx, stepSeq))
		}
		if len(steps) > 0 {
			plans = append(plans, ScenarioPlan{
				Name:        "接口全覆盖",
				Description: "按依赖拓扑序自动生成",
				Steps:       steps,
			})
		}
	}
	return plans
}

func planGroupSteps(graph *depgraph.Graph, grp depgraph.ResourceGroup, pctx planContext, skip map[int]bool) []StepPlan {
	indices := append([]int(nil), grp.Endpoint...)
	sort.Slice(indices, func(i, j int) bool {
		return depgraphMethodPriority(graph.Endpoints[indices[i]]) < depgraphMethodPriority(graph.Endpoints[indices[j]])
	})

	loginIdx := graph.LoginForGroup(grp)
	stepSeq := map[int]int{}
	var steps []StepPlan
	seq := 0
	if loginIdx >= 0 && !skip[loginIdx] {
		seq++
		stepSeq[loginIdx] = seq
		steps = append(steps, stepFromEndpoint(graph, loginIdx, pctx, stepSeq))
	}
	for _, idx := range indices {
		if skip[idx] || idx == loginIdx {
			continue
		}
		seq++
		stepSeq[idx] = seq
		steps = append(steps, stepFromEndpoint(graph, idx, pctx, stepSeq))
	}
	return steps
}

func stepFromEndpoint(graph *depgraph.Graph, idx int, pctx planContext, stepSeq map[int]int) StepPlan {
	ep := graph.Endpoints[idx]
	sp := StepPlan{
		Name:         caseName(ep),
		Endpoint:     ep,
		CaseID:       pctx.caseMap[ep.ID],
		ExpectStatus: expectedStatus(ep),
	}
	var parts []json.RawMessage
	if isLoginEndpoint(ep) {
		parts = append(parts, loginBodyForEndpoint(ep, pctx))
		sp.Config = loginStepConfig(ep)
	} else {
		if findLoginSeq(graph, idx, stepSeq) > 0 && needsBearer(ep) {
			parts = append(parts, bearerAuthOverride())
		}
		for _, m := range graph.MappingsForConsumer(idx) {
			prodSeq := stepSeq[m.ProducerIndex]
			if prodSeq <= 0 {
				continue
			}
			switch m.ConsumerKind {
			case "path_param":
				parts = append(parts, pathIDOverride(prodSeq, m.ProducerPath))
			case "body_field":
				parts = append(parts, bodyFieldOverride(prodSeq, m.ProducerPath, m.ConsumerTarget))
			}
		}
		if ep.Method == "POST" && strings.Contains(strings.ToLower(ep.Path), "/users") {
			parts = append(parts, jsonRequestOverride(map[string]any{
				"body": map[string]any{
					"name":  "Generated User",
					"email": "{{$mock.email}}",
					"role":  "tester",
				},
			}))
		}
	}
	sp.RequestOverride = mergeOverrides(parts...)
	return sp
}

func findLoginSeq(graph *depgraph.Graph, idx int, stepSeq map[int]int) int {
	for _, loginIdx := range graph.LoginIndices {
		if seq, ok := stepSeq[loginIdx]; ok {
			return seq
		}
	}
	for prodIdx, seq := range stepSeq {
		if isLoginEndpoint(graph.Endpoints[prodIdx]) && prodIdx < idx {
			return seq
		}
	}
	return 0
}

func depgraphMethodPriority(ep spec.Endpoint) int {
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

func loginBodyForEndpoint(ep spec.Endpoint, pctx planContext) json.RawMessage {
	admin := strings.Contains(strings.ToLower(ep.Path), "/admin/")
	var savedCaseRaw json.RawMessage
	if caseID := pctx.caseMap[ep.ID]; caseID != uuid.Nil {
		savedCaseRaw = pctx.caseRequest[caseID]
	}
	raw, _, _ := BuildLoginRequestBody(ep, savedCaseRaw, pctx.creds, admin)
	return raw
}

func loginCredentials(ep spec.Endpoint) (string, string) {
	user, pass, _ := ResolveLoginCredentials(
		strings.Contains(strings.ToLower(ep.Path), "/admin/"),
		nil, "", "",
	)
	return user, pass
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
		cfg := mergeStepConfig(sp.Config, statusAssertionConfig(sp.ExpectStatus))
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
