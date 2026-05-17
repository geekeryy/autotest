package aiprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"autotest/internal/aiprovider/client"
	"autotest/internal/aitools"

	"github.com/google/uuid"
)

// MaxToolHops caps how many round-trips the tool-calling loop will perform
// for a single AI call. Each hop is one LLM round-trip; after the cap the
// loop returns whatever text the model produced last (or a clear warning if
// the model still wanted to call tools).
const MaxToolHops = 6

// MockSetSummaryProvider 是 aiprovider 用来为 `generate_params` 注入项目级
// 命名值集合摘要的可选依赖。返回的 summary 仅供模型参考；任何错误都被静默
// 忽略以避免 AI 生成因为附属信息缺失而失败。
type MockSetSummaryProvider interface {
	SummariesForProject(ctx context.Context, projectID uuid.UUID) []MockSetSummary
}

// MockSetSummary 是单个命名值集合的精简描述（key + name + 前 10 条 values
// + 是否含 weights），用于嵌入 `generate_params` 的 user context。
type MockSetSummary struct {
	Key            string   `json:"key"`
	Name           string   `json:"name"`
	ValuesPreview  []string `json:"valuesPreview"`
	HasWeights     bool     `json:"hasWeights"`
	TotalValueSize int      `json:"totalValueSize"`
}

// Service applies validation, defaulting and orchestrates client-side AI calls.
type Service struct {
	repo     *Repository
	mockSets MockSetSummaryProvider
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// WithMockSets 注入项目级命名值集合摘要源（fluent 风格），便于 main.go
// 在所有依赖构造完毕后再连接二者，避免循环 import。
func (s *Service) WithMockSets(provider MockSetSummaryProvider) *Service {
	s.mockSets = provider
	return s
}

// SupportedTypes returns metadata about each provider type for the frontend create form.
func (s *Service) SupportedTypes() []ProviderTypeMeta {
	meta := supportedTypesMeta()
	out := make([]ProviderTypeMeta, len(meta))
	for i, m := range meta {
		// Models are offline fallbacks; live lists come from ListModels/DiscoverModels.
		m.Models = nil
		out[i] = m
	}
	return out
}

// List returns all providers visible to the project (with masked API keys).
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]Provider, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	rows, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]Provider, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r.toAPI())
	}
	return out, nil
}

// Create validates and inserts a new provider record.
func (s *Service) Create(ctx context.Context, projectID uuid.UUID, input CreateInput) (*Provider, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if err := normalizeCreate(&input); err != nil {
		return nil, err
	}
	row, err := s.repo.Create(ctx, projectID, input)
	if err != nil {
		return nil, err
	}
	return row.toAPI(), nil
}

// Update mutates an existing provider record.
func (s *Service) Update(ctx context.Context, projectID, providerID uuid.UUID, input UpdateInput) (*Provider, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if providerID == uuid.Nil {
		return nil, errors.New("providerId is required")
	}
	existing, err := s.repo.Get(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	if err := normalizeUpdate(&input, existing); err != nil {
		return nil, err
	}
	row, err := s.repo.Update(ctx, projectID, providerID, input)
	if err != nil {
		return nil, err
	}
	return row.toAPI(), nil
}

// Delete soft-deletes a provider.
func (s *Service) Delete(ctx context.Context, projectID, providerID uuid.UUID) error {
	if projectID == uuid.Nil || providerID == uuid.Nil {
		return errors.New("projectId and providerId are required")
	}
	return s.repo.Delete(ctx, projectID, providerID)
}

// TestConnection sends a minimal probe message and returns a short snippet of the model reply.
func (s *Service) TestConnection(ctx context.Context, projectID, providerID uuid.UUID) (*ChatResponse, error) {
	row, err := s.repo.Get(ctx, projectID, providerID)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, ErrProviderDisabled
	}
	cli, err := s.buildClient(row)
	if err != nil {
		return nil, err
	}
	temperature := 0.0
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	result, err := cli.Chat(timeoutCtx, []client.Message{
		{Role: "system", Content: "你是一个回声助手，仅回答用户的最后一句话。"},
		{Role: "user", Content: "ping"},
	}, client.Options{Temperature: &temperature, MaxTokens: 64})
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}
	return &ChatResponse{
		ProviderID:    row.ID,
		Action:        "test",
		Model:         result.Model,
		Text:          result.Text,
		ElapsedMillis: elapsed,
	}, nil
}

// Chat runs a structured AI request for one of the supported actions.
func (s *Service) Chat(ctx context.Context, projectID uuid.UUID, req ChatRequest) (*ChatResponse, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if req.ProviderID == uuid.Nil {
		return nil, errors.New("providerId is required")
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		action = ActionRaw
	}
	if !validAction(action) {
		return nil, ErrProviderActionInvalid
	}
	if action == ActionGenerateAssertion && strings.TrimSpace(req.Prompt) == "" {
		return nil, ErrAssertionIntentRequired
	}

	row, err := s.repo.Get(ctx, projectID, req.ProviderID)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, ErrProviderDisabled
	}

	cli, err := s.buildClient(row)
	if err != nil {
		return nil, err
	}

	contextPayload := req.Context
	// 仅 `generate_params` 注入 availableMockSets：避免污染其它 action 的
	// context（例如 generate_assertion 不需要这些数据）。注入失败被静默
	// 忽略，保证主流程不被附属信息阻塞。
	if action == ActionGenerateParams && s.mockSets != nil {
		summaries := s.mockSets.SummariesForProject(ctx, projectID)
		if len(summaries) > 0 {
			contextPayload = mergeAvailableMockSets(req.Context, summaries)
		}
	}

	messages, jsonOnly := buildMessages(action, req.Prompt, contextPayload, req.SystemPromptOverride)
	opts := client.Options{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		JSONOnly:    jsonOnly,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	start := time.Now()
	result, err := cli.Chat(timeoutCtx, messages, opts)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}

	resp := &ChatResponse{
		ProviderID:    row.ID,
		Action:        action,
		Model:         result.Model,
		Text:          result.Text,
		ElapsedMillis: elapsed,
	}
	if jsonOnly || action == ActionGenerateParams || action == ActionGenerateCaseData {
		parsed, warning := extractParsedJSON(result.Text)
		resp.Parsed = parsed
		resp.ParseWarnings = warning
	}
	return resp, nil
}

// ChatWithTools is the multi-hop version of Chat. When the configured model
// returns tool_calls, each non-mutating tool is executed locally and the
// result is fed back to the model; the loop continues until the model
// produces a final text answer (or until MaxToolHops is reached).
//
// PR-1 contract: mutating tools are filtered out before being shown to the
// model. Human-in-the-loop confirmation for mutating tools lands together
// with the global assistant in a later PR.
//
// Errors from individual tool runs are not fatal — the error message is
// embedded into the tool result fed back to the model, which can then
// recover (e.g. try a different tool or stop calling).
func (s *Service) ChatWithTools(ctx context.Context, projectID uuid.UUID, req ChatRequest, tools []aitools.Tool) (*ChatResponse, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if req.ProviderID == uuid.Nil {
		return nil, errors.New("providerId is required")
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		action = ActionRaw
	}
	if !validAction(action) {
		return nil, ErrProviderActionInvalid
	}

	row, err := s.repo.Get(ctx, projectID, req.ProviderID)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, ErrProviderDisabled
	}

	cli, err := s.buildClient(row)
	if err != nil {
		return nil, err
	}

	readOnly := make([]aitools.Tool, 0, len(tools))
	toolByName := make(map[string]aitools.Tool, len(tools))
	for _, t := range tools {
		if t.Mutating {
			continue
		}
		readOnly = append(readOnly, t)
		toolByName[t.Name] = t
	}

	messages, _ := buildMessages(action, req.Prompt, req.Context, req.SystemPromptOverride)
	baseOpts := client.Options{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Tools:       aitools.Describe(readOnly),
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	var (
		finalText  string
		finalModel string
		totalMs    int64
		lastResult *client.Result
	)

	for hop := 0; hop < MaxToolHops; hop++ {
		start := time.Now()
		result, err := cli.Chat(timeoutCtx, messages, baseOpts)
		totalMs += time.Since(start).Milliseconds()
		if err != nil {
			return nil, err
		}
		lastResult = result
		finalModel = result.Model

		if len(result.ToolCalls) == 0 {
			finalText = result.Text
			break
		}

		// Append the assistant turn that requested the tool calls.
		messages = append(messages, client.Message{
			Role:             "assistant",
			Content:          result.Text,
			ReasoningContent: result.ReasoningContent,
			ToolCalls:        result.ToolCalls,
		})

		// Execute each tool call and append a corresponding tool result.
		for _, call := range result.ToolCalls {
			content := executeToolCall(ctx, toolByName, call)
			messages = append(messages, client.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    content,
			})
		}
	}

	if finalText == "" {
		if lastResult != nil && len(lastResult.ToolCalls) > 0 {
			finalText = fmt.Sprintf("AI 在 %d 轮工具调用后仍未给出结论，已停止。请缩小问题范围或在 Prompt 中给出更明确的指令。", MaxToolHops)
		} else {
			finalText = ""
		}
	}

	return &ChatResponse{
		ProviderID:    row.ID,
		Action:        action,
		Model:         finalModel,
		Text:          finalText,
		ElapsedMillis: totalMs,
	}, nil
}

// executeToolCall runs a single tool and serialises the result for the
// model. Both success and failure paths return a JSON string so the model
// always sees structured content under the "tool" role; this matches what
// OpenAI and Anthropic expect.
func executeToolCall(ctx context.Context, tools map[string]aitools.Tool, call client.ToolCall) string {
	tool, ok := tools[call.Name]
	if !ok {
		return encodeToolError(fmt.Sprintf("未知工具: %s", call.Name))
	}
	args := call.Arguments
	if len(strings.TrimSpace(string(args))) == 0 {
		args = json.RawMessage("{}")
	}
	value, err := tool.Run(ctx, args)
	if err != nil {
		return encodeToolError(err.Error())
	}
	body, err := json.Marshal(value)
	if err != nil {
		return encodeToolError(fmt.Sprintf("序列化工具结果失败: %s", err))
	}
	return string(body)
}

func encodeToolError(message string) string {
	b, err := json.Marshal(map[string]string{"error": message})
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, message)
	}
	return string(b)
}

func (s *Service) buildClient(row *providerRow) (client.Client, error) {
	if row == nil {
		return nil, ErrProviderNotFound
	}
	extra, err := client.SpecFromExtra(row.ExtraConfig)
	if err != nil {
		return nil, err
	}
	spec := client.Spec{
		Type:         row.ProviderType,
		BaseURL:      row.BaseURL,
		APIKey:       row.APIKey,
		DefaultModel: row.DefaultModel,
		ExtraConfig:  extra,
	}
	if row.ProviderType != ProviderTypeOllama && row.APIKey == "" {
		return nil, ErrProviderEmptyKey
	}
	return client.New(spec)
}

func normalizeCreate(input *CreateInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.ProviderType = strings.ToLower(strings.TrimSpace(input.ProviderType))
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.DefaultModel = strings.TrimSpace(input.DefaultModel)
	input.APIKey = strings.TrimSpace(input.APIKey)

	if input.Name == "" {
		return errors.New("name is required")
	}
	if !validProviderType(input.ProviderType) {
		return ErrProviderTypeInvalid
	}
	if input.BaseURL == "" {
		return errors.New("baseUrl is required")
	}
	if input.ProviderType != ProviderTypeOllama && input.APIKey == "" {
		return ErrProviderEmptyKey
	}
	merged, err := mergeInputModalityModels(input.ExtraConfig, input.ModalityModels)
	if err != nil {
		return err
	}
	input.ExtraConfig = merged
	if err := validateExtra(input.ExtraConfig); err != nil {
		return err
	}
	return nil
}

func normalizeUpdate(input *UpdateInput, existing *providerRow) error {
	input.Name = strings.TrimSpace(input.Name)
	input.ProviderType = strings.ToLower(strings.TrimSpace(input.ProviderType))
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.DefaultModel = strings.TrimSpace(input.DefaultModel)

	if input.Name == "" {
		return errors.New("name is required")
	}
	if !validProviderType(input.ProviderType) {
		return ErrProviderTypeInvalid
	}
	if input.BaseURL == "" {
		return errors.New("baseUrl is required")
	}

	effectiveKey := existing.APIKey
	if input.APIKey != nil {
		trimmed := strings.TrimSpace(*input.APIKey)
		input.APIKey = &trimmed
		effectiveKey = trimmed
	}
	if input.ProviderType != ProviderTypeOllama && effectiveKey == "" {
		return ErrProviderEmptyKey
	}
	merged, err := mergeInputModalityModels(input.ExtraConfig, input.ModalityModels)
	if err != nil {
		return err
	}
	input.ExtraConfig = merged
	if err := validateExtra(input.ExtraConfig); err != nil {
		return err
	}
	return nil
}

func mergeInputModalityModels(extra json.RawMessage, models *ProviderModalityModels) (json.RawMessage, error) {
	mm := parseModalityModels(extra)
	if models != nil {
		mm = *models
	}
	mm.Image = strings.TrimSpace(mm.Image)
	mm.Audio = strings.TrimSpace(mm.Audio)
	mm.Video = strings.TrimSpace(mm.Video)
	return mergeModalityModelsIntoExtra(extra, mm)
}

func validProviderType(t string) bool {
	switch t {
	case ProviderTypeDeepSeek, ProviderTypeXiaomi, ProviderTypeOpenAI,
		ProviderTypeAnthropic, ProviderTypeKimi, ProviderTypeOllama:
		return true
	default:
		return false
	}
}

func validAction(action string) bool {
	switch action {
	case ActionGenerateParams, ActionGenerateAssertion, ActionGenerateCaseData,
		ActionAnalyzeFailure, ActionAnalyzeSpecChanges, ActionRaw:
		return true
	default:
		return false
	}
}

func validateExtra(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("extraConfig must be a JSON object: %w", err)
	}
	return nil
}

// mergeAvailableMockSets 把 summaries 写入 raw context 的 `availableMockSets`
// 字段。若 raw 不是 JSON 对象（比如数组、null、空），构造一个新对象只承载
// availableMockSets。所有 marshal/unmarshal 错误都退回原始 raw，绝不中断
// 主流程。
func mergeAvailableMockSets(raw json.RawMessage, summaries []MockSetSummary) json.RawMessage {
	if len(summaries) == 0 {
		return raw
	}
	var ctxObj map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &ctxObj); err != nil || ctxObj == nil {
			ctxObj = nil
		}
	}
	if ctxObj == nil {
		ctxObj = map[string]any{}
	}
	ctxObj["availableMockSets"] = summaries
	out, err := json.Marshal(ctxObj)
	if err != nil {
		return raw
	}
	return out
}
