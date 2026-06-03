package ainl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

// ChatProvider 是调用 LLM 的抽象接口。
// 使用 any 类型避免循环导入（aiprovider.Service 通过适配器实现此接口）。
type ChatProvider interface {
	// Chat 发送请求到 LLM 并返回响应。
	// req 是 aiprovider.ChatRequest，resp 是 *aiprovider.ChatResponse。
	// 使用 any 类型是因为 aiprovider 包不能被此包直接导入。
	Chat(ctx context.Context, projectID uuid.UUID, req any) (any, error)
	// ResolveDefaultChatProvider 获取平台默认的 AI 提供商 ID 和模型。
	ResolveDefaultChatProvider(ctx context.Context) (uuid.UUID, string, error)
}

// ChatResponseData 是从 LLM 响应中提取的文本数据。
type ChatResponseData struct {
	Model string `json:"model"`
	Text  string `json:"text"`
}

// extractChatText 从 aiprovider.ChatResponse 中提取文本。
func extractChatText(resp any) (*ChatResponseData, error) {
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("序列化 ChatResponse 失败: %w", err)
	}
	var data ChatResponseData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("解析 ChatResponse 失败: %w", err)
	}
	return &data, nil
}

// SpecRepository 是获取端点列表的接口。
type SpecRepository interface {
	ListEndpoints(ctx context.Context, projectID, serviceID uuid.UUID) ([]spec.Endpoint, error)
}

// OrchestratorInput 是编排引擎的输入。
type OrchestratorInput struct {
	Description   string     `json:"description"`     // 自然语言描述
	ProjectID     uuid.UUID  `json:"projectId"`       // 项目 ID
	ServiceID     uuid.UUID  `json:"serviceId"`       // 服务 ID
	EnvironmentID *uuid.UUID `json:"environmentId,omitempty"` // 可选的环境 ID
}

// OrchestratorOutput 是编排引擎的输出。
type OrchestratorOutput struct {
	ScenarioName     string            `json:"scenarioName"`
	Description      string            `json:"description"`
	Steps            []ParsedStep      `json:"steps"`
	Preview          string            `json:"preview"`
	Notes            string            `json:"notes,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	Valid            bool              `json:"valid"`
	ValidationErrors []ValidationError `json:"validationErrors,omitempty"`
}

// ParsedStep 是解析后的单个步骤。
type ParsedStep struct {
	StepOrder       int              `json:"stepOrder"`
	StepType        string           `json:"stepType"`
	Name            string           `json:"name"`
	EndpointMethod  string           `json:"endpointMethod,omitempty"`
	EndpointPath    string           `json:"endpointPath,omitempty"`
	EndpointID      *uuid.UUID       `json:"endpointId,omitempty"`
	MatchConfidence string           `json:"matchConfidence,omitempty"`
	RequestOverride map[string]any   `json:"requestOverride,omitempty"`
	Assertions      []map[string]any `json:"assertions,omitempty"`
	Description     string           `json:"description,omitempty"`
}

// Orchestrator 是自然语言编排引擎。
type Orchestrator struct {
	ai   ChatProvider
	spec SpecRepository
}

// NewOrchestrator 创建编排引擎实例。
func NewOrchestrator(ai ChatProvider, specRepo SpecRepository) *Orchestrator {
	return &Orchestrator{
		ai:   ai,
		spec: specRepo,
	}
}

// Orchestrate 执行自然语言编排的完整流程：
// 1. 获取可用端点列表
// 2. 调用 LLM 解析自然语言
// 3. 匹配端点
// 4. 校验结果
// 5. 构建输出
func (o *Orchestrator) Orchestrate(ctx context.Context, input OrchestratorInput) (*OrchestratorOutput, error) {
	// 1. 获取可用端点列表
	endpoints, err := o.spec.ListEndpoints(ctx, input.ProjectID, input.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("获取端点列表失败: %w", err)
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("服务 %s 下没有已导入的接口，请先导入 OpenAPI 文档", input.ServiceID)
	}

	// 2. 调用 LLM 解析自然语言
	llmResult, err := o.callLLMForParsing(ctx, input, endpoints)
	if err != nil {
		return nil, fmt.Errorf("LLM 解析失败: %w", err)
	}

	// 3. 端点匹配
	matcher := NewEndpointMatcher(endpoints)
	matchResults := matcher.MatchAll(llmResult.Steps)

	// 对低置信度匹配尝试 LLM 辅助匹配
	for i, mr := range matchResults {
		if mr.Confidence == "none" && llmResult.Steps[i].StepType == "api" {
			bestEP := o.fuzzyMatchWithLLM(ctx, input.ProjectID, llmResult.Steps[i], endpoints)
			if bestEP != nil {
				matchResults[i] = MatchResult{
					Endpoint:   bestEP,
					Confidence: "medium",
					Reason:     "LLM 辅助匹配",
				}
			}
		}
	}

	// 4. 校验结果
	validation := Validate(llmResult.Steps, matchResults)

	// 5. 构建输出
	output := o.buildOutput(llmResult, matchResults, validation)
	return output, nil
}

// llmChatRequest 是发送给 LLM 的请求（序列化为 aiprovider.ChatRequest 兼容格式）。
type llmChatRequest struct {
	ProviderID uuid.UUID `json:"providerId"`
	Action     string    `json:"action"`
	Prompt     string    `json:"prompt"`
}

// callLLMForParsing 调用 LLM 解析自然语言描述。
func (o *Orchestrator) callLLMForParsing(ctx context.Context, input OrchestratorInput, endpoints []spec.Endpoint) (*LLMOperations, error) {
	briefs := toEndpointBriefs(endpoints)
	prompt := BuildParsePrompt(input.Description, briefs)

	providerID, _, err := o.ai.ResolveDefaultChatProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取 AI 提供商失败: %w", err)
	}

	req := llmChatRequest{
		ProviderID: providerID,
		Action:     "raw",
		Prompt:     prompt,
	}

	resp, err := o.ai.Chat(ctx, input.ProjectID, req)
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	chatResp, err := extractChatText(resp)
	if err != nil {
		return nil, fmt.Errorf("提取 LLM 响应失败: %w", err)
	}

	// 解析 LLM 返回的 JSON
	var ops LLMOperations
	text := extractJSON(chatResp.Text)
	if err := json.Unmarshal([]byte(text), &ops); err != nil {
		return nil, fmt.Errorf("LLM 返回的 JSON 解析失败: %w\n原始响应: %s", err, truncate(chatResp.Text, 500))
	}

	if len(ops.Steps) == 0 {
		return nil, fmt.Errorf("LLM 未能从描述中提取出任何操作步骤，请尝试更详细地描述测试流程")
	}

	return &ops, nil
}

// fuzzyMatchWithLLM 对无法精确匹配的步骤，使用 LLM 从候选中选择最佳匹配。
func (o *Orchestrator) fuzzyMatchWithLLM(ctx context.Context, projectID uuid.UUID, step LLMStep, endpoints []spec.Endpoint) *spec.Endpoint {
	// 过滤同 method 的候选
	var candidates []EndpointBrief
	for _, ep := range endpoints {
		if strings.EqualFold(ep.Method, step.Method) {
			candidates = append(candidates, EndpointBrief{
				Method:      ep.Method,
				Path:        ep.Path,
				Summary:     ep.Summary,
				OperationID: ep.OperationID,
			})
		}
	}
	if len(candidates) == 0 || len(candidates) > 20 {
		return nil
	}

	prompt := BuildMatchPrompt(step.Method, step.Path, candidates)

	providerID, _, err := o.ai.ResolveDefaultChatProvider(ctx)
	if err != nil {
		return nil
	}

	req := llmChatRequest{
		ProviderID: providerID,
		Action:     "raw",
		Prompt:     prompt,
	}

	resp, err := o.ai.Chat(ctx, projectID, req)
	if err != nil {
		return nil
	}

	chatResp, err := extractChatText(resp)
	if err != nil {
		return nil
	}

	var match struct {
		MatchedIndex int    `json:"matchedIndex"`
		Confidence   string `json:"confidence"`
		Reason       string `json:"reason"`
	}
	text := extractJSON(chatResp.Text)
	if err := json.Unmarshal([]byte(text), &match); err != nil {
		return nil
	}

	if match.MatchedIndex < 1 || match.MatchedIndex > len(candidates) {
		return nil
	}

	// 通过 method + path 反查原始 Endpoint
	selected := candidates[match.MatchedIndex-1]
	for i := range endpoints {
		if endpoints[i].Method == selected.Method && endpoints[i].Path == selected.Path {
			return &endpoints[i]
		}
	}
	return nil
}

// buildOutput 构建最终输出。
func (o *Orchestrator) buildOutput(llmResult *LLMOperations, matchResults []MatchResult, validation ValidationResult) *OrchestratorOutput {
	steps := make([]ParsedStep, 0, len(llmResult.Steps))
	var warnings []string

	for i, step := range llmResult.Steps {
		ps := ParsedStep{
			StepOrder:   step.StepOrder,
			StepType:    step.StepType,
			Name:        step.Name,
			Description: step.Description,
		}

		if step.StepType == "api" || step.StepType == "" {
			ps.EndpointMethod = step.Method
			ps.EndpointPath = step.Path

			if i < len(matchResults) && matchResults[i].Endpoint != nil {
				ep := matchResults[i].Endpoint
				ps.EndpointID = &ep.ID
				ps.EndpointMethod = ep.Method
				ps.EndpointPath = ep.Path
				ps.MatchConfidence = matchResults[i].Confidence

				if matchResults[i].Confidence != "exact" {
					warnings = append(warnings, fmt.Sprintf("步骤 %d（%s）的端点 %s %s 为%s匹配（%s）",
						step.StepOrder, step.Name, step.Method, step.Path,
						confidenceLabel(matchResults[i].Confidence), matchResults[i].Reason))
				}
			} else if i < len(matchResults) {
				warnings = append(warnings, fmt.Sprintf("步骤 %d（%s）的端点 %s %s 无法匹配到已知接口",
					step.StepOrder, step.Name, step.Method, step.Path))
			}
		}

		// 转换 request 为 requestOverride
		if step.Request != nil {
			ps.RequestOverride = step.Request
		}

		// 转换 assertions
		if len(step.Assertions) > 0 {
			ps.Assertions = step.Assertions
		}

		steps = append(steps, ps)
	}

	// 收集校验警告
	for _, ve := range validation.Errors {
		if ve.Field == "assertions" {
			warnings = append(warnings, ve.Message)
		}
	}

	// 构建预览
	preview := buildPreview(llmResult, steps)

	return &OrchestratorOutput{
		ScenarioName:     llmResult.ScenarioName,
		Description:      llmResult.ScenarioDescription,
		Steps:            steps,
		Preview:          preview,
		Notes:            llmResult.Notes,
		Warnings:         warnings,
		Valid:            validation.Valid,
		ValidationErrors: validationErrorsForOutput(validation),
	}
}

func validationErrorsForOutput(validation ValidationResult) []ValidationError {
	if validation.Valid {
		return nil
	}
	out := make([]ValidationError, 0, len(validation.Errors))
	for _, ve := range validation.Errors {
		if ve.Field == "assertions" {
			continue
		}
		out = append(out, ve)
	}
	return out
}

// buildPreview 构建人类可读的场景预览。
func buildPreview(llmResult *LLMOperations, steps []ParsedStep) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("场景名称：%s\n", llmResult.ScenarioName))
	if llmResult.ScenarioDescription != "" {
		sb.WriteString(fmt.Sprintf("场景描述：%s\n", llmResult.ScenarioDescription))
	}
	sb.WriteString(fmt.Sprintf("\n共 %d 个步骤：\n", len(steps)))

	for _, step := range steps {
		sb.WriteString(fmt.Sprintf("\n  %d. [%s] %s", step.StepOrder, step.StepType, step.Name))
		if step.EndpointMethod != "" && step.EndpointPath != "" {
			sb.WriteString(fmt.Sprintf("\n     %s %s", step.EndpointMethod, step.EndpointPath))
		}
		if step.MatchConfidence != "" && step.MatchConfidence != "exact" {
			sb.WriteString(fmt.Sprintf(" (%s匹配)", confidenceLabel(step.MatchConfidence)))
		}
		if step.Description != "" {
			sb.WriteString(fmt.Sprintf("\n     %s", step.Description))
		}
		if len(step.Assertions) > 0 {
			sb.WriteString(fmt.Sprintf("\n     断言: %d 条", len(step.Assertions)))
		}
		sb.WriteString("\n")
	}

	if llmResult.Notes != "" {
		sb.WriteString(fmt.Sprintf("\n编排说明：%s\n", llmResult.Notes))
	}

	return sb.String()
}

// confidenceLabel 返回置信度的中文标签。
func confidenceLabel(confidence string) string {
	switch confidence {
	case "exact":
		return "精确"
	case "high":
		return "高置信度"
	case "medium":
		return "中等置信度"
	case "low":
		return "低置信度"
	case "none":
		return "未"
	default:
		return confidence
	}
}

// extractJSON 从 LLM 响应中提取 JSON 字符串。
// 处理 markdown 围栏（```json ... ```）包裹的情况。
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// 尝试提取 markdown 围栏中的 JSON
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + len("```json")
		end := strings.Index(text[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	if idx := strings.Index(text, "```"); idx >= 0 {
		start := idx + len("```")
		// 跳过可能的语言标识符（如 ```json）
		if nlIdx := strings.Index(text[start:], "\n"); nlIdx >= 0 {
			start = start + nlIdx + 1
		}
		end := strings.Index(text[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// 尝试找最外层的 JSON 对象
	start := strings.Index(text, "{")
	if start < 0 {
		return text
	}
	end := strings.LastIndex(text, "}")
	if end < start {
		return text
	}
	return text[start : end+1]
}

// truncate 截断字符串到指定长度。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
