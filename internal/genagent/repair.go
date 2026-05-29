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
根据单次场景步骤失败证据，将失败分类为以下三类之一（只输出 JSON）：
- test_data：测试数据/凭据/占位符不正确，可通过修改 requestOverride 或 case 请求体修复
- generation_bug：场景生成逻辑问题（步骤顺序、断言、变量引用错误），可通过 update_scenario_step 修复
- real_defect：被测 API 真实缺陷，不应自动修复

输出格式（禁止 markdown 围栏）：
{"category":"test_data|generation_bug|real_defect","reason":"中文一句话","suggestedFix":"可选修复建议"}`

// AIClassifier classifies failures via LLM.
type AIClassifier interface {
	ClassifyFailure(ctx context.Context, projectID uuid.UUID, evidence map[string]any) (failureClass, error)
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

func (c *aiproviderClassifier) ClassifyFailure(ctx context.Context, projectID uuid.UUID, evidence map[string]any) (failureClass, error) {
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

func buildFailureEvidence(scName string, step scenario.Step, sr runner.StepRunResult) map[string]any {
	ev := map[string]any{
		"scenarioName": scName,
		"stepName":     step.Name,
		"stepOrder":    step.StepOrder,
		"stepType":     step.StepType,
	}
	if sr.Result != nil {
		ev["status"] = sr.Result.Status
		ev["error"] = sr.Result.Error
		ev["assertions"] = sr.Result.Assertions
		ev["requestSnapshot"] = json.RawMessage(sr.Result.RequestSnapshot)
		ev["responseSnapshot"] = json.RawMessage(sr.Result.ResponseSnapshot)
	}
	if len(sr.StepErrors) > 0 {
		ev["stepErrors"] = sr.StepErrors
	}
	return ev
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
