package aiprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"autotest/internal/aiprovider/client"

	"github.com/google/uuid"
)

// ContextBudget manages token allocation for each LLM call.
type ContextBudget struct {
	// TotalTokens is the model's context window size.
	TotalTokens int
	// ReservedTokens is reserved for LLM output.
	ReservedTokens int
	// SystemTokens is the estimated token count for system prompt + profile + memories.
	SystemTokens int
	// AvailableTokens = TotalTokens - ReservedTokens - SystemTokens.
	AvailableTokens int
}

// DefaultContextBudget returns a conservative budget for models with unknown context windows.
func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		TotalTokens:     128000,
		ReservedTokens:  4096,
		SystemTokens:    0,
		AvailableTokens: 123904,
	}
}

// EstimateContextBudget estimates the budget based on model capabilities.
func EstimateContextBudget(modelID string, systemPromptTokens int) ContextBudget {
	total := estimateModelContextWindow(modelID)
	reserved := 4096
	system := systemPromptTokens
	available := total - reserved - system
	if available < 1024 {
		available = 1024
	}
	return ContextBudget{
		TotalTokens:     total,
		ReservedTokens:  reserved,
		SystemTokens:    system,
		AvailableTokens: available,
	}
}

// estimateModelContextWindow returns the context window size for known models.
func estimateModelContextWindow(modelID string) int {
	model := strings.ToLower(strings.TrimSpace(modelID))
	switch {
	case strings.Contains(model, "gpt-4o"):
		return 128000
	case strings.Contains(model, "gpt-4-turbo") || strings.Contains(model, "gpt-4-1106"):
		return 128000
	case strings.Contains(model, "gpt-4-32k"):
		return 32768
	case strings.Contains(model, "gpt-4"):
		return 8192
	case strings.Contains(model, "gpt-3.5-turbo-16k"):
		return 16384
	case strings.Contains(model, "gpt-3.5"):
		return 4096
	case strings.Contains(model, "claude-3-opus") || strings.Contains(model, "claude-3-sonnet") || strings.Contains(model, "claude-3-haiku"):
		return 200000
	case strings.Contains(model, "claude-3.5-sonnet") || strings.Contains(model, "claude-3-5-sonnet"):
		return 200000
	case strings.Contains(model, "claude-2") || strings.Contains(model, "claude-instant"):
		return 100000
	case strings.Contains(model, "deepseek"):
		return 64000
	case strings.Contains(model, "qwen"):
		return 131072
	case strings.Contains(model, "glm"):
		return 128000
	default:
		return 128000 // Conservative default
	}
}

// ContextManager builds message lists that fit within a token budget.
type ContextManager struct {
	budget     ContextBudget
	compressor Compressor
	layerSvc   PromptLayerService // optional: dynamic prompt layers from DB
}

// NewContextManager creates a new ContextManager with the given budget.
func NewContextManager(budget ContextBudget, compressor Compressor) *ContextManager {
	if compressor == nil {
		compressor = &RuleBasedCompressor{}
	}
	return &ContextManager{
		budget:     budget,
		compressor: compressor,
	}
}

// WithPromptLayer injects a dynamic prompt layer service.
func (cm *ContextManager) WithPromptLayer(svc PromptLayerService) *ContextManager {
	cm.layerSvc = svc
	return cm
}

// BuildContextMessages builds a message list that fits within the token budget.
// It preserves:
// - Layer 0: Pinned context (system prompt, profile, page context)
// - Layer 1: Active window (most recent messages)
// - Layer 2: Compressed history (older messages summarized)
func (cm *ContextManager) BuildContextMessages(
	history []StoredMessage,
	pageContext json.RawMessage,
	profileContext string,
	visionEnabled bool,
	skillName string,
) []client.Message {
	// Build pinned context (always included)
	pinned := cm.buildPinnedContext(pageContext, profileContext, skillName)

	// Estimate tokens for pinned context
	pinnedTokens := estimateMessagesTokens(pinned)

	// Adjust available tokens for history
	availableForHistory := cm.budget.AvailableTokens - pinnedTokens
	if availableForHistory < 512 {
		availableForHistory = 512
	}

	// Filter out pending_confirm messages
	filtered := make([]StoredMessage, 0, len(history))
	for _, m := range history {
		if m.Status != storedStatusPendingConfirm {
			filtered = append(filtered, m)
		}
	}

	// If history fits within budget, return as-is
	historyTokens := estimateStoredMessagesTokens(filtered)
	if historyTokens <= availableForHistory {
		return cm.buildFullMessages(pinned, filtered, visionEnabled)
	}

	// Need compression: split into active window and compressed history
	activeWindow, compressedHistory := cm.splitHistory(filtered, availableForHistory)

	// Compress older messages
	compressedMsg := cm.compressHistory(compressedHistory)

	// Build final message list
	return cm.buildCompressedMessages(pinned, compressedMsg, activeWindow, visionEnabled)
}

// buildPinnedContext builds the system messages that are always included.
func (cm *ContextManager) buildPinnedContext(pageContext json.RawMessage, profileContext string, skillName string) []client.Message {
	out := make([]client.Message, 0, 4)

	// Use PromptBuilder for layered prompt assembly.
	// When layerSvc is available, include dynamic layers from the database.
	promptBuilder := NewAssistantPromptBuilderWithLayers(cm.layerSvc)
	promptCtx := PromptContext{
		Action:            "assistant_chat",
		HasPageContext:    len(pageContext) > 0,
		HasProfileContext: profileContext != "",
	}
	if skillName != "" {
		promptCtx.HasSkill = true
		promptCtx.SkillName = skillName
	}
	systemPrompt := promptBuilder.Build(promptCtx)
	out = append(out, client.Message{Role: "system", Content: systemPrompt})

	if profileContext != "" {
		out = append(out, client.Message{Role: "system", Content: "## 项目测试画像\n" + profileContext})
	}
	if msg, ok := renderPageContextSystem(pageContext); ok {
		out = append(out, msg)
	}
	return out
}

// splitHistory splits messages into active window (recent) and compressed history (older).
// Returns (activeWindow, compressedHistory).
func (cm *ContextManager) splitHistory(messages []StoredMessage, availableTokens int) ([]StoredMessage, []StoredMessage) {
	if len(messages) == 0 {
		return nil, nil
	}

	// Walk backwards from the most recent message, accumulating tokens
	tokenBudget := availableTokens
	activeStart := len(messages)

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := estimateStoredMessageTokens(&messages[i])
		if tokenBudget-msgTokens < 0 {
			// Can't fit this message, stop here
			break
		}
		tokenBudget -= msgTokens
		activeStart = i

		// Ensure the active window starts at a user message boundary.
		// If this is a user message and there's an assistant reply already
		// in the active window, stop expanding — we have a complete pair.
		if messages[i].Role == "user" {
			hasAssistant := false
			for j := i + 1; j < len(messages); j++ {
				if messages[j].Role == "assistant" {
					hasAssistant = true
					break
				}
			}
			if hasAssistant {
				break
			}
		}
	}

	// Ensure we keep at least the last message
	if activeStart >= len(messages) {
		activeStart = len(messages) - 1
	}

	return messages[activeStart:], messages[:activeStart]
}

// compressHistory compresses older messages into a summary.
func (cm *ContextManager) compressHistory(messages []StoredMessage) string {
	if len(messages) == 0 {
		return ""
	}

	// Use rule-based compression for tool results
	compressed := cm.compressor.Compress(context.Background(), messages)
	return compressed
}

// buildFullMessages builds messages when history fits within budget.
func (cm *ContextManager) buildFullMessages(pinned []client.Message, history []StoredMessage, visionEnabled bool) []client.Message {
	out := make([]client.Message, 0, len(pinned)+len(history))
	out = append(out, pinned...)
	out = append(out, convertStoredMessages(history, visionEnabled)...)
	return out
}

// buildCompressedMessages builds messages with compressed history.
func (cm *ContextManager) buildCompressedMessages(pinned []client.Message, compressed string, activeWindow []StoredMessage, visionEnabled bool) []client.Message {
	out := make([]client.Message, 0, len(pinned)+1+len(activeWindow))
	out = append(out, pinned...)

	// Add compressed history as a system message
	if compressed != "" {
		out = append(out, client.Message{
			Role:    "system",
			Content: fmt.Sprintf("## 历史摘要\n%s", compressed),
		})
	}

	// Add active window messages
	out = append(out, convertStoredMessages(activeWindow, visionEnabled)...)
	return out
}

// convertStoredMessages converts StoredMessage slice to client.Message slice.
func convertStoredMessages(messages []StoredMessage, visionEnabled bool) []client.Message {
	out := make([]client.Message, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case "user":
			msg := client.Message{Role: "user", Content: m.Content}
			if visionEnabled {
				msg.Images = clientImagesFromAttachments(attachmentsFromRaw(m.Attachments))
			}
			out = append(out, msg)
		case "assistant":
			msg := client.Message{Role: "assistant", Content: m.Content, ReasoningContent: m.ReasoningContent}
			if len(m.ToolCalls) > 0 {
				var stored []StoredToolCall
				if err := json.Unmarshal(m.ToolCalls, &stored); err == nil {
					for _, c := range stored {
						msg.ToolCalls = append(msg.ToolCalls, client.ToolCall{
							ID:        c.ID,
							Name:      c.Name,
							Arguments: c.Arguments,
						})
					}
				}
			}
			out = append(out, msg)
		case "tool":
			out = append(out, client.Message{
				Role:       "tool",
				ToolCallID: m.ToolCallID,
				Content:    m.Content,
			})
		}
	}
	return out
}

// Compressor compresses messages into a summary.
type Compressor interface {
	Compress(ctx context.Context, messages []StoredMessage) string
}

// RuleBasedCompressor compresses messages using deterministic rules.
// It keeps tool names and one-line summaries, and truncates long content.
type RuleBasedCompressor struct {
	// MaxToolResultLength is the maximum length for tool results in the summary.
	MaxToolResultLength int
	// MaxMessagesInSummary is the maximum number of messages to include in the summary.
	MaxMessagesInSummary int
}

// LLMCompressor uses an LLM to produce higher-quality conversation summaries.
// It falls back to RuleBasedCompressor when the LLM call fails or when the
// message count is too low to justify the LLM cost.
type LLMCompressor struct {
	ai         *Service
	fallback   *RuleBasedCompressor
	minMsgs    int // minimum messages to trigger LLM compression
	maxTokens  int
}

// NewLLMCompressor creates an LLM-backed compressor. The ai service is used
// to call the platform's default chat provider for summarization.
func NewLLMCompressor(ai *Service) *LLMCompressor {
	return &LLMCompressor{
		ai:        ai,
		fallback:  &RuleBasedCompressor{},
		minMsgs:   5,
		maxTokens: 500,
	}
}

// Compress produces a summary of the given messages. For small histories it
// delegates to the rule-based compressor; for larger ones it calls the LLM.
func (c *LLMCompressor) Compress(ctx context.Context, messages []StoredMessage) string {
	if len(messages) == 0 {
		return ""
	}

	// Always produce a rule-based preliminary compression first.
	preliminary := c.fallback.Compress(ctx, messages)

	// Skip LLM for small histories.
	if len(messages) < c.minMsgs {
		return preliminary
	}
	if c.ai == nil {
		return preliminary
	}

	providerID, model, err := c.ai.ResolveDefaultChatProvider(ctx)
	if err != nil {
		return preliminary
	}

	prompt := "请将以下 AI 助理对话历史压缩为结构化摘要。保留：\n" +
		"1. 用户的关键决策和偏好\n" +
		"2. 工具调用的重要结果（成功/失败）\n" +
		"3. 未完成的待办事项\n" +
		"格式要求：简短的要点列表，不超过 300 字。\n\n" +
		"原始对话：\n" + preliminary

	resp, err := c.ai.Chat(ctx, uuid.Nil, ChatRequest{
		ProviderID: providerID,
		Action:     ActionRaw,
		Prompt:     prompt,
		Model:      model,
		MaxTokens:  c.maxTokens,
	})
	if err != nil || resp.Text == "" {
		return preliminary
	}
	return resp.Text
}

func (r *RuleBasedCompressor) Compress(ctx context.Context, messages []StoredMessage) string {
	if len(messages) == 0 {
		return ""
	}

	maxResultLen := r.MaxToolResultLength
	if maxResultLen <= 0 {
		maxResultLen = 200
	}
	maxMessages := r.MaxMessagesInSummary
	if maxMessages <= 0 {
		maxMessages = 20
	}

	var sb strings.Builder
	round := 0
	msgCount := 0

	for _, m := range messages {
		if msgCount >= maxMessages {
			sb.WriteString(fmt.Sprintf("\n... (还有 %d 条更早的消息)", len(messages)-msgCount))
			break
		}

		switch m.Role {
		case "user":
			round++
			content := truncateString(m.Content, 100)
			sb.WriteString(fmt.Sprintf("\n第 %d 轮 - 用户: %s", round, content))
		case "assistant":
			content := truncateString(m.Content, 100)
			if content != "" {
				sb.WriteString(fmt.Sprintf("\n第 %d 轮 - 助理: %s", round, content))
			}
			// Summarize tool calls
			if len(m.ToolCalls) > 0 {
				var calls []StoredToolCall
				if err := json.Unmarshal(m.ToolCalls, &calls); err == nil {
					for _, c := range calls {
						sb.WriteString(fmt.Sprintf("\n  → 调用工具: %s", c.Name))
					}
				}
			}
		case "tool":
			// Compress tool results
			content := compressToolResult(m.ToolCallID, m.Content, maxResultLen)
			sb.WriteString(fmt.Sprintf("\n  ← 工具结果: %s", content))
		}
		msgCount++
	}

	return sb.String()
}

// compressToolResult compresses a tool result to a one-line summary.
func compressToolResult(callID string, content string, maxLen int) string {
	// Try to parse as JSON and extract key info
	var result map[string]any
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		// Check for error
		if errVal, ok := result["error"]; ok {
			return fmt.Sprintf("错误: %v", errVal)
		}
		// Check for common patterns
		if data, ok := result["data"]; ok {
			switch v := data.(type) {
			case []any:
				return fmt.Sprintf("返回 %d 条记录", len(v))
			case map[string]any:
				if id, ok := v["id"]; ok {
					return fmt.Sprintf("返回对象 (id: %v)", id)
				}
			}
		}
		if total, ok := result["total"]; ok {
			return fmt.Sprintf("返回 %v 条记录", total)
		}
	}

	// Fallback: truncate the content
	return truncateString(content, maxLen)
}

// truncateString truncates a string to maxLen bytes, ensuring valid UTF-8.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Walk back to a valid rune boundary.
	end := maxLen - 3
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "..."
}

// estimateMessagesTokens estimates the token count for a slice of client.Message.
func estimateMessagesTokens(messages []client.Message) int {
	total := 0
	for _, m := range messages {
		total += estimateClientMessageTokens(&m)
	}
	return total
}

// estimateClientMessageTokens estimates the token count for a single client.Message.
func estimateClientMessageTokens(m *client.Message) int {
	// Rough estimation: 1 token ≈ 4 characters for English, 2 characters for Chinese
	// We use a conservative estimate of 3 characters per token
	tokens := 4 // Base overhead per message (role, formatting)
	tokens += estimateStringTokens(m.Content)
	tokens += estimateStringTokens(m.ReasoningContent)
	for _, tc := range m.ToolCalls {
		tokens += estimateStringTokens(tc.Name)
		tokens += estimateStringTokens(string(tc.Arguments))
	}
	return tokens
}

// estimateStoredMessagesTokens estimates the token count for a slice of StoredMessage.
func estimateStoredMessagesTokens(messages []StoredMessage) int {
	total := 0
	for i := range messages {
		total += estimateStoredMessageTokens(&messages[i])
	}
	return total
}

// estimateStoredMessageTokens estimates the token count for a single StoredMessage.
func estimateStoredMessageTokens(m *StoredMessage) int {
	tokens := 4 // Base overhead
	tokens += estimateStringTokens(m.Content)
	tokens += estimateStringTokens(m.ReasoningContent)
	tokens += estimateStringTokens(m.ToolCallID)
	if len(m.ToolCalls) > 0 {
		tokens += len(m.ToolCalls) * 10 // Rough estimate for tool call overhead
		var calls []StoredToolCall
		if err := json.Unmarshal(m.ToolCalls, &calls); err == nil {
			for _, c := range calls {
				tokens += estimateStringTokens(c.Name)
				tokens += estimateStringTokens(string(c.Arguments))
			}
		}
	}
	return tokens
}

// estimateStringTokens estimates the token count for a string.
// Uses byte-length heuristic: ~1 token per 3 bytes (works for both ASCII and
// CJK text since CJK characters are ~3 bytes in UTF-8 and ~1-2 tokens each).
func estimateStringTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 2) / 3
}
