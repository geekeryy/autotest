package genagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"autotest/internal/aiprovider"
	"autotest/internal/report"
	"autotest/internal/runner"
	"autotest/internal/scenario"
	"autotest/internal/scenariogen"

	"github.com/google/uuid"
)


const classifySystem = `你是接口自动化测试平台的「场景生成修复助手」。
根据场景步骤失败证据，将失败分类为以下三类之一（只输出 JSON）：
- test_data：测试数据/凭据/占位符不正确，可通过修改 requestOverride 或 case 请求体修复
- generation_bug：场景生成逻辑问题（步骤顺序、断言、变量引用错误），可通过 update_scenario_step 修复
- real_defect：被测 API 真实缺陷，不应自动修复

分类时请注意：
- 检查 assertionDiffs 中断言失败的具体原因，区分数据问题和逻辑问题
- 检查 previousStepOutputs 中前序步骤是否正常，变量引用是否正确
- 如果 previousRepairs 非空，参考之前的修复尝试，避免重复相同策略
- 如果同一类别已修复过但仍失败，考虑提升为 real_defect
- 如果 endpointRequiredFields 非空，检查这些字段是否都有有效值（非 __FILL_ 占位符）
- 如果 responseSnapshot 中的错误信息提到具体字段，优先将该字段纳入修复范围
- suggestedFix 应尽量具体，指出需要修改的字段名和期望值

输出格式（禁止 markdown 围栏）：
{"category":"test_data|generation_bug|real_defect","reason":"中文一句话","suggestedFix":"具体修复建议"}`

// AIClassifier classifies failures via LLM.
type AIClassifier interface {
	ClassifyFailure(ctx context.Context, projectID uuid.UUID, evidence FailureEvidence) (failureClass, error)
}

// FixGenerator generates concrete fixes via LLM when rule-based repair fails.
type FixGenerator interface {
	GenerateFix(ctx context.Context, projectID uuid.UUID, step scenario.Step, class failureClass, evidence FailureEvidence) (json.RawMessage, error)
}

type fixGeneratorImpl struct {
	ai *aiprovider.Service
}

func NewFixGenerator(ai *aiprovider.Service) FixGenerator {
	return &fixGeneratorImpl{ai: ai}
}

const fixGenSystem = `你是接口自动化测试平台的「测试数据修复助手」。
根据失败步骤的上下文和修复建议，生成修复后的 requestOverride JSON。

规则：
1. 只输出 JSON，禁止 markdown 围栏
2. 输出格式与 requestOverride 相同：{"body":{...},"headers":{...},...}
3. 保留原有 requestOverride 中正确的字段，只修改有问题的字段
4. 如果修复建议中提到了具体字段和值，直接使用
5. 如果 endpointRequiredFields 中有字段，确保修复后的 body 中这些字段都有有效值（不要用 __FILL_ 占位符）
6. 如果 responseSnapshot 中的错误信息提到某个字段无效或缺失，优先修复该字段
7. 如果无法确定修复方案，输出 {"error":"无法生成修复方案","reason":"原因"}

输入包含：
- stepRequestOverride：当前步骤的请求覆盖配置
- suggestedFix：分类阶段给出的修复建议
- responseSnapshot：实际的 API 响应（用于判断错误原因）
- assertionDiffs：断言失败详情
- endpointSummary：接口功能描述
- endpointRequiredFields：接口必填字段列表
- endpointEnumFields：接口枚举字段及已知有效值`

func (g *fixGeneratorImpl) GenerateFix(ctx context.Context, projectID uuid.UUID, step scenario.Step, class failureClass, evidence FailureEvidence) (json.RawMessage, error) {
	providerID, model, err := g.ai.ResolveDefaultChatProvider(ctx)
	if err != nil {
		return nil, err
	}
	input := map[string]any{
		"stepRequestOverride": step.RequestOverride,
		"suggestedFix":        class.SuggestedFix,
		"responseSnapshot":    evidence.ResponseSnapshot,
		"assertionDiffs":      evidence.AssertionDiffs,
		"stepConfig":          step.Config,
	}
	if evidence.EndpointSummary != "" {
		input["endpointSummary"] = evidence.EndpointSummary
	}
	if len(evidence.EndpointRequiredFields) > 0 {
		input["endpointRequiredFields"] = evidence.EndpointRequiredFields
	}
	if len(evidence.EndpointEnumFields) > 0 {
		input["endpointEnumFields"] = evidence.EndpointEnumFields
	}
	raw, _ := json.Marshal(input)
	resp, err := g.ai.Chat(ctx, projectID, aiprovider.ChatRequest{
		ProviderID:           providerID,
		Action:               aiprovider.ActionRaw,
		Prompt:               "请生成修复后的 requestOverride：\n" + string(raw),
		SystemPromptOverride: fixGenSystem,
		Model:                model,
	})
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(resp.Text)
	if start := strings.Index(text, "{"); start >= 0 {
		if end := strings.LastIndex(text, "}"); end > start {
			text = text[start : end+1]
		}
	}
	// Validate it's parseable JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("LLM 返回的修复方案无法解析: %w", err)
	}
	if _, hasErr := parsed["error"]; hasErr {
		return nil, fmt.Errorf("LLM 无法生成修复: %v", parsed["reason"])
	}
	return json.RawMessage(text), nil
}

type failureClass struct {
	Category       string `json:"category"`
	Reason         string `json:"reason"`
	SuggestedFix   string `json:"suggestedFix"`
}

type aiproviderClassifier struct {
	ai *aiprovider.Service
}

func NewAIClassifier(ai *aiprovider.Service) AIClassifier {
	return &aiproviderClassifier{ai: ai}
}

func (c *aiproviderClassifier) ClassifyFailure(ctx context.Context, projectID uuid.UUID, evidence FailureEvidence) (failureClass, error) {
	providerID, model, err := c.ai.ResolveDefaultChatProvider(ctx)
	if err != nil {
		return failureClass{Category: "generation_bug", Reason: err.Error()}, nil
	}
	raw, _ := json.Marshal(evidence)
	resp, err := c.ai.Chat(ctx, projectID, aiprovider.ChatRequest{
		ProviderID:           providerID,
		Action:               aiprovider.ActionRaw,
		Prompt:               "请分类以下场景步骤失败证据：\n" + string(raw),
		SystemPromptOverride: classifySystem,
		Model:                model,
	})
	if err != nil {
		return failureClass{Category: "generation_bug", Reason: err.Error()}, nil
	}
	text := strings.TrimSpace(resp.Text)
	if start := strings.Index(text, "{"); start >= 0 {
		if end := strings.LastIndex(text, "}"); end > start {
			text = text[start : end+1]
		}
	}
	var out failureClass
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return failureClass{Category: "generation_bug", Reason: "无法解析分类结果"}, nil
	}
	out.Category = strings.TrimSpace(strings.ToLower(out.Category))
	switch out.Category {
	case "test_data", "generation_bug", "real_defect":
	default:
		out.Category = "generation_bug"
	}
	return out, nil
}

// Repairer applies automatic fixes for test_data and generation_bug failures.
type Repairer struct {
	Scenarios ScenarioRepairService
	AI        AIClassifier
	Fixer     FixGenerator // optional; when nil, LLM-assisted fixes are skipped
}

type ScenarioRepairService interface {
	ListSteps(ctx context.Context, scenarioID uuid.UUID) ([]scenario.Step, error)
	UpsertStep(ctx context.Context, scenarioID uuid.UUID, input scenario.UpsertStepInput) (*scenario.Step, error)
}

func applyRepair(ctx context.Context, r *Repairer, cfg RunConfig, projectID uuid.UUID, scID uuid.UUID, step *scenario.Step, stepResult runner.StepRunResult, class failureClass) (*RepairSummary, error) {
	if r == nil || step == nil {
		return nil, fmt.Errorf("repairer not configured")
	}
	switch class.Category {
	case "real_defect":
		return &RepairSummary{
			ScenarioID: scID,
			StepID:     step.ID,
			Category:   class.Category,
			Action:     "skip",
			Detail:     class.Reason,
		}, nil
	case "test_data":
		return repairTestData(ctx, r, cfg, scID, step, class)
	case "generation_bug":
		return repairGeneration(ctx, r, projectID, scID, step, stepResult, class)
	default:
		return nil, fmt.Errorf("unknown category %q", class.Category)
	}
}

func repairTestData(ctx context.Context, r *Repairer, cfg RunConfig, scID uuid.UUID, step *scenario.Step, class failureClass) (*RepairSummary, error) {
	override := step.RequestOverride
	var ro map[string]any
	if len(override) > 0 {
		_ = json.Unmarshal(override, &ro)
	}
	if ro == nil {
		ro = map[string]any{}
	}
	body, _ := ro["body"].(map[string]any)
	if body == nil {
		body = map[string]any{}
	}
	admin := adminStep(step)
	changed := false
	for k, v := range body {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "__FILL_") {
			replacement := placeholderReplacement(k, cfg.LoginCredentials, admin)
			if replacement != "" {
				body[k] = replacement
				changed = true
			}
		}
	}
	if !changed {
		if pair := loginPairForRepair(cfg.LoginCredentials, admin); pair != nil {
			fieldsApplied := applyRepairPair(body, pair)
			if fieldsApplied {
				changed = true
			}
		}
	}
	if !changed {
		// Rule-based repair failed; try LLM-assisted fix if available.
		if r.Fixer != nil && class.SuggestedFix != "" {
			return repairTestDataWithLLM(ctx, r, scID, step, class, FailureEvidence{})
		}
		return &RepairSummary{
			ScenarioID: scID,
			StepID:     step.ID,
			Category:   "test_data",
			Action:     "noop",
			Detail:     class.Reason,
		}, nil
	}
	ro["body"] = body
	raw, _ := json.Marshal(ro)
	_, err := r.Scenarios.UpsertStep(ctx, scID, scenario.UpsertStepInput{
		StepOrder:       step.StepOrder,
		StepType:        step.StepType,
		Name:            step.Name,
		Enabled:         boolPtr(step.Enabled),
		TestCaseID:      step.TestCaseID,
		Config:          step.Config,
		RequestOverride: raw,
	})
	if err != nil {
		return nil, err
	}
	return &RepairSummary{
		ScenarioID: scID,
		StepID:     step.ID,
		Category:   "test_data",
		Action:     "update_request_override",
		Detail:     class.SuggestedFix,
	}, nil
}

func repairTestDataWithLLM(ctx context.Context, r *Repairer, scID uuid.UUID, step *scenario.Step, class failureClass, evidence FailureEvidence) (*RepairSummary, error) {
	fixed, err := r.Fixer.GenerateFix(ctx, uuid.Nil, *step, class, evidence)
	if err != nil {
		return &RepairSummary{
			ScenarioID: scID,
			StepID:     step.ID,
			Category:   "test_data",
			Action:     "llm_fix_failed",
			Detail:     err.Error(),
		}, nil
	}
	// Merge the LLM-generated fix into the existing requestOverride.
	override := step.RequestOverride
	var ro map[string]any
	if len(override) > 0 {
		_ = json.Unmarshal(override, &ro)
	}
	if ro == nil {
		ro = map[string]any{}
	}
	var fix map[string]any
	if err := json.Unmarshal(fixed, &fix); err != nil {
		return &RepairSummary{
			ScenarioID: scID,
			StepID:     step.ID,
			Category:   "test_data",
			Action:     "llm_fix_invalid",
			Detail:     err.Error(),
		}, nil
	}
	// Shallow merge: LLM fix fields override existing fields.
	for k, v := range fix {
		ro[k] = v
	}
	raw, _ := json.Marshal(ro)
	if _, err := r.Scenarios.UpsertStep(ctx, scID, scenario.UpsertStepInput{
		StepOrder:       step.StepOrder,
		StepType:        step.StepType,
		Name:            step.Name,
		Enabled:         boolPtr(step.Enabled),
		TestCaseID:      step.TestCaseID,
		Config:          step.Config,
		RequestOverride: raw,
	}); err != nil {
		return nil, err
	}
	return &RepairSummary{
		ScenarioID: scID,
		StepID:     step.ID,
		Category:   "test_data",
		Action:     "llm_fix_applied",
		Detail:     class.SuggestedFix,
	}, nil
}

func adminStep(step *scenario.Step) bool {
	if step == nil || step.Name == "" {
		return false
	}
	return strings.Contains(strings.ToLower(step.Name), "admin")
}

func applyRepairPair(body map[string]any, pair *scenariogen.CredentialPair) bool {
	if pair == nil {
		return false
	}
	changed := false
	if pair.Username != "" {
		for _, key := range []string{"username", "userName", "user", "email", "login"} {
			if _, ok := body[key]; ok {
				body[key] = pair.Username
				changed = true
			}
		}
	}
	if pair.Password != "" {
		for _, key := range []string{"password", "pass", "pwd"} {
			if _, ok := body[key]; ok {
				body[key] = pair.Password
				changed = true
			}
		}
	}
	for k, v := range pair.Body {
		if v != nil && fmt.Sprint(v) != "" {
			body[k] = v
			changed = true
		}
	}
	return changed
}

func placeholderReplacement(field string, creds *scenariogen.LoginCredentialBundle, admin bool) string {
	if val, ok := scenariogen.CredentialValueForField(field, creds, admin); ok {
		return val
	}
	user, pass := credentialPairValues(creds, admin)
	switch strings.ToLower(field) {
	case "username", "user", "email", "login":
		if user != "" {
			return user
		}
	case "password", "pass", "pwd":
		if pass != "" {
			return pass
		}
	}
	return ""
}

func loginPairForRepair(creds *scenariogen.LoginCredentialBundle, admin bool) *scenariogen.CredentialPair {
	if creds == nil {
		return nil
	}
	if admin && creds.Admin != nil && hasRepairCredentials(creds.Admin) {
		return creds.Admin
	}
	if creds.User != nil && hasRepairCredentials(creds.User) {
		return creds.User
	}
	return nil
}

func hasRepairCredentials(pair *scenariogen.CredentialPair) bool {
	if pair == nil {
		return false
	}
	if strings.TrimSpace(pair.Username) != "" && strings.TrimSpace(pair.Password) != "" {
		return true
	}
	for _, v := range pair.Body {
		if v != nil && strings.TrimSpace(fmt.Sprint(v)) != "" {
			return true
		}
	}
	return false
}

func credentialPairValues(creds *scenariogen.LoginCredentialBundle, admin bool) (user, pass string) {
	if pair := loginPairForRepair(creds, admin); pair != nil {
		return pair.Username, pair.Password
	}
	return "", ""
}

func repairGeneration(ctx context.Context, r *Repairer, projectID uuid.UUID, scID uuid.UUID, step *scenario.Step, stepResult runner.StepRunResult, class failureClass) (*RepairSummary, error) {
	cfg := step.Config
	var cfgMap map[string]any
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &cfgMap)
	}
	if cfgMap == nil {
		cfgMap = map[string]any{}
	}
	assertions, _ := cfgMap["assertions"].([]any)
	if len(assertions) == 0 {
		actual := extractActualStatus(stepResult)
		if actual > 0 && actual != 200 {
			cfgMap["assertions"] = []map[string]any{{"type": "status_code", "expected": actual}}
		}
	} else if r.Fixer != nil && class.SuggestedFix != "" {
		// Existing assertions but still failing — try LLM-assisted fix.
		evidence := FailureEvidence{
			StepConfig:          step.Config,
			StepRequestOverride: step.RequestOverride,
		}
		if stepResult.Result != nil {
			evidence.ResponseSnapshot = stepResult.Result.ResponseSnapshot
		}
		fixed, err := r.Fixer.GenerateFix(ctx, projectID, *step, class, evidence)
		if err == nil {
			var fixMap map[string]any
			if json.Unmarshal(fixed, &fixMap) == nil {
				// If LLM returned a config-like structure, merge assertions.
				if newAsserts, ok := fixMap["assertions"]; ok {
					cfgMap["assertions"] = newAsserts
				}
			}
		}
	}
	rawCfg, _ := json.Marshal(cfgMap)
	_, err := r.Scenarios.UpsertStep(ctx, scID, scenario.UpsertStepInput{
		StepOrder:       step.StepOrder,
		StepType:        step.StepType,
		Name:            step.Name,
		Enabled:         boolPtr(step.Enabled),
		TestCaseID:      step.TestCaseID,
		Config:          rawCfg,
		RequestOverride: step.RequestOverride,
	})
	if err != nil {
		return nil, err
	}
	return &RepairSummary{
		ScenarioID: scID,
		StepID:     step.ID,
		Category:   "generation_bug",
		Action:     "adjust_assertions",
		Detail:     class.Reason,
	}, nil
}

func extractActualStatus(sr runner.StepRunResult) int {
	if sr.Result == nil || len(sr.Result.ResponseSnapshot) == 0 {
		return 0
	}
	var snap map[string]any
	if json.Unmarshal(sr.Result.ResponseSnapshot, &snap) != nil {
		return 0
	}
	if code, ok := snap["statusCode"].(float64); ok {
		return int(code)
	}
	return 0
}

func buildFailureEvidence(scName string, step scenario.Step, sr runner.StepRunResult, runOutput *runner.RunScenarioOutput, prevRepairs []RepairAttempt) FailureEvidence {
	ev := FailureEvidence{
		ScenarioName:        scName,
		StepName:            step.Name,
		StepOrder:           step.StepOrder,
		StepType:            string(step.StepType),
		StepConfig:          step.Config,
		StepRequestOverride: step.RequestOverride,
		PreviousRepairs:     prevRepairs,
	}
	if sr.Result != nil {
		ev.Status = string(sr.Result.Status)
		ev.Error = sr.Result.Error
		ev.Assertions = sr.Result.Assertions
		ev.RequestSnapshot = sr.Result.RequestSnapshot
		ev.ResponseSnapshot = sr.Result.ResponseSnapshot
		ev.AssertionDiffs = buildAssertionDiffs(sr.Result.Assertions)
	}
	ev.StepOutput = sr.Output
	if runOutput != nil {
		ev.PreviousStepOutputs = buildPreviousStepOutputs(runOutput, step.StepOrder)
	}
	// Extract endpoint metadata from the request override for LLM context.
	ev.EndpointSummary, ev.EndpointRequiredFields, ev.EndpointEnumFields = extractEndpointMeta(step)
	return ev
}

// extractEndpointMeta extracts useful endpoint information from the step's
// request override and config to help the LLM understand the API contract.
func extractEndpointMeta(step scenario.Step) (summary string, requiredFields []string, enumFields map[string][]any) {
	if len(step.RequestOverride) == 0 {
		return "", nil, nil
	}
	var ro map[string]any
	if json.Unmarshal(step.RequestOverride, &ro) != nil {
		return "", nil, nil
	}

	// Extract required and enum fields from body schema if present.
	body, _ := ro["body"].(map[string]any)
	if body != nil {
		requiredFields = extractRequiredFieldNames(body)
		enumFields = extractEnumFieldValues(body)
	}
	return summary, requiredFields, enumFields
}

// extractRequiredFieldNames finds fields with __FILL_ placeholders (indicating
// they are required but not yet filled).
func extractRequiredFieldNames(body map[string]any) []string {
	var required []string
	for k, v := range body {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "__FILL_") {
			required = append(required, k)
		}
	}
	return required
}

// extractEnumFieldValues finds fields with enum-like values (small fixed sets).
func extractEnumFieldValues(body map[string]any) map[string][]any {
	// This is a heuristic: if a field value is one of a known set, include it.
	// Real enum detection happens at the profile level; this is a fallback.
	return nil
}

func buildAssertionDiffs(assertions json.RawMessage) []AssertionDiff {
	if len(assertions) == 0 {
		return nil
	}
	var results []assertionResult
	if err := json.Unmarshal(assertions, &results); err != nil {
		return nil
	}
	var diffs []AssertionDiff
	for _, r := range results {
		if r.Passed {
			continue
		}
		diffs = append(diffs, AssertionDiff{
			Type:    r.Type,
			Name:    r.Name,
			Message: r.Message,
		})
	}
	if len(diffs) == 0 {
		return nil
	}
	return diffs
}

// assertionResult mirrors assertion.Result for JSON unmarshalling.
type assertionResult struct {
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

func buildPreviousStepOutputs(output *runner.RunScenarioOutput, currentStepOrder int) []StepOutputSummary {
	if output == nil {
		return nil
	}
	var summaries []StepOutputSummary
	for _, sr := range output.StepResults {
		if sr.Step.StepOrder >= currentStepOrder {
			break
		}
		status := ""
		if sr.Result != nil {
			status = string(sr.Result.Status)
		}
		summaries = append(summaries, StepOutputSummary{
			StepOrder: sr.Step.StepOrder,
			StepName:  sr.Step.Name,
			Status:    status,
			Output:    truncateOutput(sr.Output),
		})
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}

// truncateOutput returns a shallow copy of the output map with string values
// truncated to 500 chars to keep the evidence payload compact while preserving
// enough detail for the LLM to diagnose root causes.
func truncateOutput(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok && len(s) > 500 {
			out[k] = s[:500] + "…"
		} else {
			out[k] = v
		}
	}
	return out
}

func countRealDefects(repairs []RepairSummary) int {
	n := 0
	for _, r := range repairs {
		if r.Category == "real_defect" {
			n++
		}
	}
	return n
}

func summarizeRun(output *runner.RunScenarioOutput) (passed bool, failures []string) {
	if output == nil || output.Run == nil {
		return false, []string{"run output missing"}
	}
	if output.Run.Status == report.RunPassed {
		return true, nil
	}
	for _, sr := range output.StepResults {
		if sr.Result == nil {
			continue
		}
		if sr.Result.Status == report.ResultPassed {
			continue
		}
		msg := sr.Result.Error
		if msg == "" {
			msg = string(sr.Result.Status)
		}
		failures = append(failures, fmt.Sprintf("步骤 %q: %s", sr.Step.Name, msg))
	}
	return false, failures
}

func boolPtr(v bool) *bool {
	return &v
}
