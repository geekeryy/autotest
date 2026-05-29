package aiprovider

// This file implements the streaming, tool-aware conversational AI chat
// that powers the global assistant floating window. The non-streaming
// `Chat` / `ChatWithTools` flows in service.go are unaffected.
//
// The big picture:
//
//	┌──────────┐           ┌──────────────────────────┐
//	│ Handler  │── stream─▶│ ChatStreamWithTools (this)│
//	└──────────┘   events  └──────────────────────────┘
//	                              │
//	                              │  load history / append user / tool messages via SessionStore
//	                              │  run cli.ChatStream (text + tool_calls)
//	                              │  on mutating tool: persist pending_confirm + emit pending event
//	                              │  on readonly tool: run + persist tool result + next hop
//	                              ▼
//	                       ai_sessions / ai_messages (db)
//
// Mutating tools never run inside the same SSE response — the loop
// suspends and the handler returns with a pending event. The frontend
// later POSTs to /tool-calls/{id}/confirm, which calls
// ContinueAfterConfirm to either execute or reject the tool and resume the
// loop in a fresh SSE response.

import (
	"bytes"
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

// AssistantStreamRequest is the SSE entrypoint payload. ProviderID is
// optional; when zero the service falls back to the project's default
// provider via repo.Get / repo.GetDefaultProvider.
type AssistantStreamRequest struct {
	SessionID       uuid.UUID `json:"sessionId"`
	ProviderID      uuid.UUID `json:"providerId"`
	UserMessage     string           `json:"userMessage"`
	Images          []UserImageInput `json:"images,omitempty"`
	Model           string           `json:"model,omitempty"`
	Temperature     *float64  `json:"temperature,omitempty"`
	MaxTokens       int       `json:"maxTokens,omitempty"`
	ThinkingEnabled   *bool `json:"thinkingEnabled,omitempty"`
	ReasoningEffort   string `json:"reasoningEffort,omitempty"`
	WebSearchEnabled  *bool  `json:"webSearchEnabled,omitempty"`
	// PageContext is the frontend's "what page is the user looking at
	// right now" snapshot. It is opaque JSON (the frontend owns the
	// shape) and is **not** persisted — every request brings the latest
	// snapshot so the AI sees the live state, not a stale one.
	PageContext json.RawMessage `json:"pageContext,omitempty"`
	// DebugEnabled streams per-hop usage events over SSE for the chat UI.
	// Token usage is always collected and persisted regardless of this flag.
	DebugEnabled *bool `json:"debugEnabled,omitempty"`
}

// StreamEventKind enumerates the events emitted by ChatStreamWithTools.
// They are 1:1 with the SSE `event:` field exposed to the frontend.
type StreamEventKind string

const (
	StreamEventMessage        StreamEventKind = "message"         // 持久化好的一条 ai_messages 记录（user/assistant/tool）
	StreamEventText           StreamEventKind = "text"            // 助理文本增量
	StreamEventThinking       StreamEventKind = "thinking"        // 模型正在输出 reasoning_content（不透传内容）
	StreamEventToolCall       StreamEventKind = "tool_call"       // 助理决定调用工具（只读工具——会立即执行）
	StreamEventToolResult     StreamEventKind = "tool_result"     // 只读工具执行结果
	StreamEventPendingConfirm StreamEventKind = "pending_confirm" // 待用户确认的写工具
	StreamEventDone           StreamEventKind = "done"            // 本次流结束
	StreamEventError          StreamEventKind = "error"           // 致命错误（流提前结束）
	StreamEventSession        StreamEventKind = "session"         // 会话元数据更新（如 AI 生成标题）
	StreamEventUsage          StreamEventKind = "usage"           // 单轮 LLM 调用的 token / 缓存明细（debug）
)

// AssistantStreamEvent is the high-level event that the SSE handler
// serialises to the frontend.
type AssistantStreamEvent struct {
	Kind StreamEventKind `json:"kind"`
	// Text is set when Kind == StreamEventText.
	Text string `json:"text,omitempty"`
	// Thinking is set when Kind == StreamEventThinking.
	Thinking *AssistantThinkingState `json:"thinking,omitempty"`
	// ToolCall is set when Kind == StreamEventToolCall / PendingConfirm.
	ToolCall *AssistantToolCall `json:"toolCall,omitempty"`
	// ToolResult is set when Kind == StreamEventToolResult.
	ToolResult *AssistantToolResult `json:"toolResult,omitempty"`
	// Message is set when Kind == StreamEventMessage and carries the
	// snapshot of the row just persisted.
	Message *PersistedMessage `json:"message,omitempty"`
	// Model / Finish are set when Kind == StreamEventDone.
	Model  string `json:"model,omitempty"`
	Finish string `json:"finish,omitempty"`
	// Error is set when Kind == StreamEventError.
	Error string `json:"error,omitempty"`
	// Session is set when Kind == StreamEventSession.
	Session *StoredSession `json:"session,omitempty"`
	// Usage is set when Kind == StreamEventUsage (debug mode).
	Usage *AssistantUsageDetail `json:"usage,omitempty"`
}

// AssistantThinkingState reports reasoning activity without exposing the
// reasoning_content itself. The content is persisted server-side only when a
// provider needs it for tool-call continuation.
type AssistantThinkingState struct {
	Active        bool  `json:"active"`
	ElapsedMillis int64 `json:"elapsedMillis,omitempty"`
}

// AssistantToolCall is the public representation of a tool invocation.
// RequiresConfirm is true for destructive writes (delete_*) that must be
// approved before execution. Mutating is kept for backward-compatible UI.
type AssistantToolCall struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Arguments       json.RawMessage `json:"arguments"`
	Mutating        bool            `json:"mutating,omitempty"`
	RequiresConfirm bool            `json:"requiresConfirm,omitempty"`
}

// AssistantToolResult mirrors a single tool execution result.
type AssistantToolResult struct {
	CallID  string `json:"callId"`
	Name    string `json:"name"`
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

// PersistedMessage is a flattened projection of ai_messages we forward to
// the frontend so it can update its local store without a second round-trip.
type PersistedMessage struct {
	ID         uuid.UUID       `json:"id"`
	Seq        int             `json:"seq"`
	Role       string          `json:"role"`
	Content     string          `json:"content"`
	Attachments json.RawMessage `json:"attachments,omitempty"`
	ToolCallID  string          `json:"toolCallId,omitempty"`
	ToolCalls  json.RawMessage `json:"toolCalls,omitempty"`
	Status     string          `json:"status"`
	Model        string          `json:"model,omitempty"`
	UsageDetails json.RawMessage `json:"usageDetails,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// SessionStore decouples the streaming engine from the concrete aisession
// package, sidestepping an import cycle and keeping aiprovider testable
// without spinning up a real database.
type SessionStore interface {
	// ListMessages returns persisted history for sessionID in seq order.
	ListMessages(ctx context.Context, sessionID uuid.UUID) ([]StoredMessage, error)
	// AppendMessage atomically appends one message and returns the
	// fully-populated record.
	AppendMessage(ctx context.Context, sessionID uuid.UUID, msg StoredMessageInput) (*StoredMessage, error)
	// FindPendingToolCall locates a pending mutating tool call inside the
	// session and returns its parent message plus the matched call.
	FindPendingToolCall(ctx context.Context, sessionID uuid.UUID, callID string) (*StoredMessage, *StoredToolCall, error)
	// MarkPendingFinal flips a pending_confirm message to final and
	// rewrites its tool_calls payload (the latter so we can drop the
	// in-flight call from the persisted history once handled).
	MarkPendingFinal(ctx context.Context, messageID uuid.UUID, toolCalls json.RawMessage) error
	// MarkPendingRejected marks a pending message as rejected.
	MarkPendingRejected(ctx context.Context, messageID uuid.UUID) error
	// GetSession returns session metadata for ownership checks and titles.
	GetSession(ctx context.Context, projectID, userID, sessionID uuid.UUID) (*StoredSession, error)
	// UpdateSessionTitle renames a session owned by the user.
	UpdateSessionTitle(ctx context.Context, projectID, userID, sessionID uuid.UUID, title string) (*StoredSession, error)
}

// StoredMessage is the abstract record returned by the session store.
// Mirrors aisession.Message but lives here so the package does not import
// aisession directly.
type StoredMessage struct {
	ID               uuid.UUID       `json:"id"`
	SessionID        uuid.UUID       `json:"sessionId"`
	Seq              int             `json:"seq"`
	Role             string          `json:"role"`
	Content          string          `json:"content"`
	Attachments      json.RawMessage `json:"attachments,omitempty"`
	ReasoningContent string          `json:"-"`
	ToolCallID       string          `json:"toolCallId,omitempty"`
	ToolCalls        json.RawMessage `json:"toolCalls,omitempty"`
	Status           string          `json:"status"`
	Model            string          `json:"model,omitempty"`
	UsageDetails     json.RawMessage `json:"usageDetails,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// StoredMessageInput is the projection used to append a message.
type StoredMessageInput struct {
	Role             string
	Content          string
	Attachments      json.RawMessage
	ReasoningContent string
	ToolCallID       string
	ToolCalls        json.RawMessage
	Status           string
	Model            string
	ElapsedMillis    int
	UsageDetails     json.RawMessage
}

// StoredToolCall represents a single tool call entry persisted inside
// ai_messages.tool_calls. The Mutating flag is propagated so the handler
// can react without re-checking the tool registry.
type StoredToolCall struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Arguments       json.RawMessage `json:"arguments"`
	Mutating        bool            `json:"mutating,omitempty"`
	RequiresConfirm bool            `json:"requiresConfirm,omitempty"`
}

// Status constants mirror the CHECK on ai_messages.status. We duplicate
// them here rather than importing aisession to keep packages decoupled.
const (
	storedStatusFinal          = "final"
	storedStatusPendingConfirm = "pending_confirm"
	storedStatusRejected       = "rejected"
)

// StreamCallback receives high-level events. Returning a non-nil error
// aborts the stream and is surfaced from ChatStreamWithTools.
type StreamCallback func(event AssistantStreamEvent) error

// streamConfig groups the inputs that flow through every helper in this file.
type streamConfig struct {
	ProjectID uuid.UUID
	SessionID uuid.UUID
	UserID    uuid.UUID
	Provider  *providerRow
	Cli       client.Client
	Store     SessionStore
	Sink      StreamCallback
	Tools     map[string]aitools.Tool
	ToolDefs  []client.ToolDefinition
	BaseOpts  client.Options
	HopBudget int // remaining hops (loop terminates when this hits 0)
	// PageContext mirrors AssistantStreamRequest.PageContext so it can
	// be injected on every LLM call inside runStreamLoop without
	// threading the request through every helper.
	PageContext    json.RawMessage
	// ProfileContext is the formatted project test profile injected as an
	// additional system message to give the AI project-specific knowledge.
	ProfileContext string
	VisionEnabled  bool
	// TurnHasImages is true when the current chat request included images
	// (set before runStreamLoop even if history reload lags).
	TurnHasImages bool
	DebugEnabled  bool
}

// ChatStreamWithTools is the entry point for SSE assistant chats. The
// caller is expected to have already verified that:
//   - the session belongs to (projectID, userID),
//   - the user message has been validated.
//
// On success this method returns nil and the SSE response is fully written
// via sink. On failure it returns an error after also pushing an
// StreamEventError to sink (so the SSE handler can flush a final frame).
func (s *Service) ChatStreamWithTools(
	ctx context.Context,
	projectID, userID uuid.UUID,
	req AssistantStreamRequest,
	store SessionStore,
	tools []aitools.Tool,
	sink StreamCallback,
) error {
	if projectID == uuid.Nil || userID == uuid.Nil {
		return s.streamFail(sink, errors.New("projectId / userId 不能为空"))
	}
	if req.SessionID == uuid.Nil {
		return s.streamFail(sink, errors.New("sessionId 不能为空"))
	}
	if sink == nil {
		return errors.New("stream callback is required")
	}
	if store == nil {
		return s.streamFail(sink, errors.New("session store 未配置"))
	}

	provider, err := s.pickProvider(ctx, req.ProviderID)
	if err != nil {
		return s.streamFail(sink, err)
	}
	cli, err := s.buildClient(provider)
	if err != nil {
		return s.streamFail(sink, err)
	}

	cfg, err := s.prepareStreamConfig(projectID, userID, req, provider, cli, store, sink, tools)
	if err != nil {
		return s.streamFail(sink, err)
	}

	if s.profileContext != nil {
		if pc, err := s.profileContext.GetPromptContext(ctx, projectID); err == nil && pc != "" {
			cfg.ProfileContext = pc
		}
	}

	userText := strings.TrimSpace(req.UserMessage)
	imageAtts, err := normalizeUserImages(req.Images)
	if err != nil {
		return s.streamFail(sink, err)
	}
	if len(imageAtts) > 0 {
		if err := providerSupportsImageInput(provider); err != nil {
			return s.streamFail(sink, err)
		}
	}
	if userText == "" && len(imageAtts) == 0 {
		return s.streamFail(sink, errors.New("userMessage 不能为空"))
	}
	rawAtts, err := attachmentsToRaw(imageAtts)
	if err != nil {
		return s.streamFail(sink, err)
	}
	// Persist the user message before contacting the LLM so that even if
	// the upstream fails the conversation history captures what was said.
	userMsg, err := store.AppendMessage(ctx, req.SessionID, StoredMessageInput{
		Role:        "user",
		Content:     userText,
		Attachments: rawAtts,
		Status:      storedStatusFinal,
	})
	if err != nil {
		return s.streamFail(sink, fmt.Errorf("持久化用户消息失败: %w", err))
	}
	if err := sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(userMsg)}); err != nil {
		return err
	}
	cfg.TurnHasImages = len(imageAtts) > 0

	titleDone := closedDoneChan()
	if session, err := store.GetSession(ctx, projectID, userID, req.SessionID); err == nil {
		userCount, _ := userMessageSnapshot(ctx, store, req.SessionID, 2)
		if shouldScheduleSessionTitle(userCount, session.Title) {
			titleDone = s.scheduleSessionTitleIfNeeded(ctx, cfg)
		}
	}
	streamErr := s.runStreamLoop(ctx, cfg)
	// Title generation runs in parallel with the main stream but must finish
	// (or time out) before the handler returns; otherwise the SSE connection
	// closes and the client never receives StreamEventSession.
	select {
	case <-titleDone:
	case <-time.After(18 * time.Second):
	}
	return streamErr
}

// ContinueAfterConfirm resumes a previously-suspended stream after the
// user decides on a pending mutating tool call. It re-opens an SSE
// connection (the handler hands sink to us) and either runs the tool or
// records the rejection, then continues the chat loop.
//
// pageContext is the latest "what page is the user on now" snapshot
// from the frontend at confirm time; it is injected into the LLM
// follow-up call the same way ChatStreamWithTools does it.
func (s *Service) ContinueAfterConfirm(
	ctx context.Context,
	projectID, userID uuid.UUID,
	sessionID uuid.UUID,
	callID string,
	decision AssistantConfirmDecision,
	providerID uuid.UUID,
	model string,
	thinkingEnabled *bool,
	reasoningEffort string,
	webSearchEnabled *bool,
	pageContext json.RawMessage,
	debugEnabled *bool,
	store SessionStore,
	tools []aitools.Tool,
	sink StreamCallback,
) error {
	if projectID == uuid.Nil || userID == uuid.Nil {
		return s.streamFail(sink, errors.New("projectId / userId 不能为空"))
	}
	if sessionID == uuid.Nil {
		return s.streamFail(sink, errors.New("sessionId 不能为空"))
	}
	if strings.TrimSpace(callID) == "" {
		return s.streamFail(sink, errors.New("callId 不能为空"))
	}
	if sink == nil {
		return errors.New("stream callback is required")
	}
	if store == nil {
		return s.streamFail(sink, errors.New("session store 未配置"))
	}

	pendingMsg, pendingCall, err := store.FindPendingToolCall(ctx, sessionID, callID)
	if err != nil {
		return s.streamFail(sink, err)
	}

	provider, err := s.pickProvider(ctx, providerID)
	if err != nil {
		return s.streamFail(sink, err)
	}
	cli, err := s.buildClient(provider)
	if err != nil {
		return s.streamFail(sink, err)
	}

	cfg, err := s.prepareStreamConfig(projectID, userID, AssistantStreamRequest{
		SessionID:        sessionID,
		ProviderID:       provider.ID,
		Model:            model,
		ThinkingEnabled:  thinkingEnabled,
		ReasoningEffort:  reasoningEffort,
		WebSearchEnabled: webSearchEnabled,
		PageContext:      pageContext,
		DebugEnabled:     debugEnabled,
	}, provider, cli, store, sink, tools)
	if err != nil {
		return s.streamFail(sink, err)
	}

	// Execute (or skip) the mutating tool and persist a corresponding tool
	// message regardless of outcome — both the model and the user need a
	// record that the call was decided.
	toolContent, isError := s.applyDecision(ctx, cfg, *pendingCall, decision)

	toolMsg, err := store.AppendMessage(ctx, sessionID, StoredMessageInput{
		Role:       "tool",
		Content:    toolContent,
		ToolCallID: pendingCall.ID,
		Status:     storedStatusFinal,
	})
	if err != nil {
		return s.streamFail(sink, fmt.Errorf("持久化工具结果失败: %w", err))
	}
	// IMPORTANT: emit the tool message to the frontend so it can drop the
	// matching entry from `pendingCalls`. Without this the approve button
	// stays visible and a second click triggers ErrPendingNotFound.
	if err := sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(toolMsg)}); err != nil {
		return err
	}
	if err := sink(AssistantStreamEvent{Kind: StreamEventToolResult, ToolResult: &AssistantToolResult{
		CallID:  pendingCall.ID,
		Name:    pendingCall.Name,
		Content: toolContent,
		IsError: isError,
	}}); err != nil {
		return err
	}

	// Update the assistant message: if approved we keep only the decided call
	// so the follow-up request has a protocol-valid assistant tool_call +
	// tool result pair. Other calls from the same suspended turn are not
	// replayed because this endpoint decides one call at a time.
	if decision.Approve {
		updated := keepOnlyCall(pendingMsg.ToolCalls, pendingCall.ID)
		if err := store.MarkPendingFinal(ctx, pendingMsg.ID, updated); err != nil {
			return s.streamFail(sink, err)
		}
		// Echo the status transition so the frontend's local copy of the
		// pending message flips to "final" (the row was rewritten in the
		// DB but we have not received a fresh snapshot from there).
		mirror := toPersisted(pendingMsg)
		mirror.Status = storedStatusFinal
		mirror.ToolCalls = updated
		if err := sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: mirror}); err != nil {
			return err
		}
	} else {
		if err := store.MarkPendingRejected(ctx, pendingMsg.ID); err != nil {
			return s.streamFail(sink, err)
		}
		mirror := toPersisted(pendingMsg)
		mirror.Status = storedStatusRejected
		if err := sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: mirror}); err != nil {
			return err
		}
	}

	return s.runStreamLoop(ctx, cfg)
}

// AssistantConfirmDecision captures the user's resolution of a mutating
// tool call.
type AssistantConfirmDecision struct {
	Approve bool
	Reason  string
}

// prepareStreamConfig wires together everything the loop needs.
func (s *Service) prepareStreamConfig(
	projectID, userID uuid.UUID,
	req AssistantStreamRequest,
	provider *providerRow,
	cli client.Client,
	store SessionStore,
	sink StreamCallback,
	tools []aitools.Tool,
) (*streamConfig, error) {
	toolMap := make(map[string]aitools.Tool, len(tools))
	for _, t := range tools {
		toolMap[t.Name] = t
	}
	temperature := req.Temperature
	maxTokens := req.MaxTokens
	model := resolveAssistantModel(provider, req.Model)
	thinking := assistantThinkingOverride(provider, req.ThinkingEnabled, req.ReasoningEffort)
	toolDefs := mergeAssistantToolDefs(aitools.Describe(tools), provider, req.WebSearchEnabled)
	debugEnabled := req.DebugEnabled != nil && *req.DebugEnabled
	baseOpts := client.Options{
		Model:       model,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Thinking:    thinking,
		Tools:       toolDefs,
	}
	baseOpts.CollectUsage = true
	return &streamConfig{
		ProjectID: projectID,
		SessionID: req.SessionID,
		UserID:    userID,
		Provider:  provider,
		Cli:       cli,
		Store:     store,
		Sink:      sink,
		Tools:     toolMap,
		ToolDefs:  toolDefs,
		BaseOpts:       baseOpts,
		HopBudget:      MaxToolHops,
		PageContext:    req.PageContext,
		VisionEnabled:  false,
		DebugEnabled:   debugEnabled,
	}, nil
}

func assistantThinkingOverride(provider *providerRow, enabled *bool, effort string) *client.OpenAIThinkingConfig {
	if enabled == nil && strings.TrimSpace(effort) == "" {
		return nil
	}
	dialect := ""
	if provider != nil {
		switch strings.ToLower(strings.TrimSpace(provider.ProviderType)) {
		case ProviderTypeDeepSeek:
			dialect = "deepseek"
		case ProviderTypeXiaomi:
			dialect = "xiaomi"
		}
	}
	cfg := &client.OpenAIThinkingConfig{
		Enabled: dialect != "",
		Dialect: dialect,
		Effort:  strings.TrimSpace(effort),
	}
	if enabled != nil {
		cfg.Enabled = *enabled && dialect != ""
	}
	return cfg
}

// runStreamLoop pulls history, then iteratively calls the LLM, persisting
// every assistant / tool message and translating client-level stream
// events to assistant-level events on the way out.
func (s *Service) runStreamLoop(ctx context.Context, cfg *streamConfig) error {
	history, err := cfg.Store.ListMessages(ctx, cfg.SessionID)
	if err != nil {
		return s.streamFail(cfg.Sink, fmt.Errorf("加载会话历史失败: %w", err))
	}
	if err := s.applyModalityRouting(ctx, cfg, history); err != nil {
		return s.streamFail(cfg.Sink, err)
	}

	messages := buildClientMessages(history, cfg.PageContext, cfg.ProfileContext, cfg.VisionEnabled)
	initialHopBudget := cfg.HopBudget

	for cfg.HopBudget > 0 {
		cfg.HopBudget--
		hopIndex := initialHopBudget - cfg.HopBudget

		streamingCli, ok := cfg.Cli.(client.StreamingClient)
		if !ok {
			return s.streamFail(cfg.Sink, client.ErrStreamingUnsupported)
		}

		var (
			textBuf      strings.Builder
			reasoningBuf strings.Builder
			toolCalls    []client.ToolCall
			model        = cfg.Provider.DefaultModel
			finish       string
			start        = time.Now()
			hopUsage     client.TokenUsage
		)
		thinkingActive := false
		var thinkingStartedAt time.Time
		emitThinking := func(active bool) error {
			if active == thinkingActive {
				return nil
			}
			state := &AssistantThinkingState{Active: active}
			if active {
				thinkingStartedAt = time.Now()
			} else if !thinkingStartedAt.IsZero() {
				state.ElapsedMillis = time.Since(thinkingStartedAt).Milliseconds()
			}
			thinkingActive = active
			return cfg.Sink(AssistantStreamEvent{Kind: StreamEventThinking, Thinking: state})
		}

		// Compose options for this hop: the base options plus the running
		// conversation. We deliberately reuse cfg.BaseOpts (rather than
		// mutating it) so a future caller can run multiple hops with the
		// same config.
		opts := cfg.BaseOpts

		if err := streamingCli.ChatStream(ctx, messages, opts, func(ev client.StreamEvent) error {
			switch ev.Kind {
			case client.StreamEventText:
				if err := emitThinking(false); err != nil {
					return err
				}
				textBuf.WriteString(ev.Text)
				return cfg.Sink(AssistantStreamEvent{Kind: StreamEventText, Text: ev.Text})
			case client.StreamEventReasoning:
				reasoningBuf.WriteString(ev.ReasoningContent)
				return emitThinking(true)
			case client.StreamEventToolCall:
				if ev.ToolCall != nil {
					toolCalls = append(toolCalls, *ev.ToolCall)
				}
				return nil
			case client.StreamEventDone:
				if strings.TrimSpace(ev.Model) != "" {
					model = ev.Model
				}
				finish = ev.Finish
				if ev.Usage != nil {
					hopUsage = hopUsage.MergePreferNonZero(*ev.Usage)
				}
				return nil
			}
			return nil
		}); err != nil {
			if thinkingActive {
				_ = emitThinking(false)
			}
			return s.streamFail(cfg.Sink, err)
		}
		if thinkingActive {
			if err := emitThinking(false); err != nil {
				return err
			}
		}

		elapsed := int(time.Since(start).Milliseconds())
		usageRaw, usageDetail := cfg.buildHopUsage(hopIndex, model, elapsed, hopUsage)

		// Xiaomi web_search is executed by the provider; strip it before our loop.
		toolCalls = filterProviderNativeToolCalls(toolCalls)

		// 1) No tool calls -> final assistant message, end of stream.
		if len(toolCalls) == 0 {
			msg, err := cfg.Store.AppendMessage(ctx, cfg.SessionID, StoredMessageInput{
				Role:             "assistant",
				Content:          textBuf.String(),
				ReasoningContent: reasoningBuf.String(),
				Status:           storedStatusFinal,
				Model:            model,
				ElapsedMillis:    elapsed,
				UsageDetails:     usageRaw,
			})
			if err != nil {
				return s.streamFail(cfg.Sink, fmt.Errorf("持久化助理消息失败: %w", err))
			}
			if err := cfg.emitUsageIfNeeded(usageDetail); err != nil {
				return err
			}
			if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(msg)}); err != nil {
				return err
			}
			return cfg.Sink(AssistantStreamEvent{Kind: StreamEventDone, Model: model, Finish: finish})
		}

		// 2) Partition tool calls. Auto-run read-only and non-destructive
		// writes; suspend only when a delete_* (RequiresConfirm) tool is present.
		confirmCalls := pickRequiresConfirm(toolCalls, cfg.Tools)
		autoCalls := pickAutoExecute(toolCalls, cfg.Tools)

		if len(autoCalls) > 0 {
			storedCalls := toStoredCalls(toolCalls, cfg.Tools)
			payload, err := json.Marshal(storedCalls)
			if err != nil {
				return s.streamFail(cfg.Sink, fmt.Errorf("序列化工具调用失败: %w", err))
			}
			assistantMsg, err := cfg.Store.AppendMessage(ctx, cfg.SessionID, StoredMessageInput{
				Role:             "assistant",
				Content:          textBuf.String(),
				ReasoningContent: reasoningBuf.String(),
				ToolCalls:        payload,
				Status:           storedStatusFinal,
				Model:            model,
				ElapsedMillis:    elapsed,
				UsageDetails:     usageRaw,
			})
			if err != nil {
				return s.streamFail(cfg.Sink, fmt.Errorf("持久化助理消息失败: %w", err))
			}
			if err := cfg.emitUsageIfNeeded(usageDetail); err != nil {
				return err
			}
			if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(assistantMsg)}); err != nil {
				return err
			}
			messages = append(messages, client.Message{
				Role:             "assistant",
				Content:          textBuf.String(),
				ReasoningContent: reasoningBuf.String(),
				ToolCalls:        toolCalls,
			})
			for _, call := range autoCalls {
				ac := toAssistantCall(call, cfg.Tools)
				if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventToolCall, ToolCall: &ac}); err != nil {
					return err
				}
				content := executeToolCall(ctx, cfg.Tools, call)
				toolMsg, err := cfg.Store.AppendMessage(ctx, cfg.SessionID, StoredMessageInput{
					Role:       "tool",
					Content:    content,
					ToolCallID: call.ID,
					Status:     storedStatusFinal,
				})
				if err != nil {
					return s.streamFail(cfg.Sink, fmt.Errorf("持久化工具结果失败: %w", err))
				}
				if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(toolMsg)}); err != nil {
					return err
				}
				if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventToolResult, ToolResult: &AssistantToolResult{
					CallID:  call.ID,
					Name:    call.Name,
					Content: content,
				}}); err != nil {
					return err
				}
				messages = append(messages, client.Message{
					Role:       "tool",
					ToolCallID: call.ID,
					Content:    content,
				})
			}
		}

		if len(confirmCalls) > 0 {
			storedCalls := toStoredCalls(toolCalls, cfg.Tools)
			payload, err := json.Marshal(storedCalls)
			if err != nil {
				return s.streamFail(cfg.Sink, fmt.Errorf("序列化工具调用失败: %w", err))
			}
			status := storedStatusPendingConfirm
			if len(autoCalls) == 0 {
				// No auto tools ran yet; persist the assistant turn now.
				msg, err := cfg.Store.AppendMessage(ctx, cfg.SessionID, StoredMessageInput{
					Role:             "assistant",
					Content:          textBuf.String(),
					ReasoningContent: reasoningBuf.String(),
					ToolCalls:        payload,
					Status:           status,
					Model:            model,
					ElapsedMillis:    elapsed,
					UsageDetails:     usageRaw,
				})
				if err != nil {
					return s.streamFail(cfg.Sink, fmt.Errorf("持久化挂起助理消息失败: %w", err))
				}
				if err := cfg.emitUsageIfNeeded(usageDetail); err != nil {
					return err
				}
				if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(msg)}); err != nil {
					return err
				}
			} else {
				// Auto tools already persisted an assistant row; add a pending row for confirm-only calls.
				msg, err := cfg.Store.AppendMessage(ctx, cfg.SessionID, StoredMessageInput{
					Role:             "assistant",
					Content:          "",
					ReasoningContent: "",
					ToolCalls:        payload,
					Status:           status,
					Model:            model,
				})
				if err != nil {
					return s.streamFail(cfg.Sink, fmt.Errorf("持久化挂起助理消息失败: %w", err))
				}
				if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(msg)}); err != nil {
					return err
				}
			}
			for _, call := range confirmCalls {
				if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventPendingConfirm, ToolCall: call}); err != nil {
					return err
				}
			}
			return cfg.Sink(AssistantStreamEvent{Kind: StreamEventDone, Model: model, Finish: "pending_confirm"})
		}

		if len(autoCalls) > 0 {
			continue
		}

		// 3) Only readonly tools -> persist the assistant turn, execute
		// each tool, persist results, then loop.
		storedCalls := toStoredCalls(toolCalls, cfg.Tools)
		payload, err := json.Marshal(storedCalls)
		if err != nil {
			return s.streamFail(cfg.Sink, fmt.Errorf("序列化工具调用失败: %w", err))
		}
		assistantMsg, err := cfg.Store.AppendMessage(ctx, cfg.SessionID, StoredMessageInput{
			Role:             "assistant",
			Content:          textBuf.String(),
			ReasoningContent: reasoningBuf.String(),
			ToolCalls:        payload,
			Status:           storedStatusFinal,
			Model:            model,
			ElapsedMillis:    elapsed,
			UsageDetails:     usageRaw,
		})
		if err != nil {
			return s.streamFail(cfg.Sink, fmt.Errorf("持久化助理消息失败: %w", err))
		}
		if err := cfg.emitUsageIfNeeded(usageDetail); err != nil {
			return err
		}
		if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(assistantMsg)}); err != nil {
			return err
		}

		messages = append(messages, client.Message{
			Role:             "assistant",
			Content:          textBuf.String(),
			ReasoningContent: reasoningBuf.String(),
			ToolCalls:        toolCalls,
		})

		for _, call := range toolCalls {
			ac := toAssistantCall(call, cfg.Tools)
			if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventToolCall, ToolCall: &ac}); err != nil {
				return err
			}
			content := executeToolCall(ctx, cfg.Tools, call)
			toolMsg, err := cfg.Store.AppendMessage(ctx, cfg.SessionID, StoredMessageInput{
				Role:       "tool",
				Content:    content,
				ToolCallID: call.ID,
				Status:     storedStatusFinal,
			})
			if err != nil {
				return s.streamFail(cfg.Sink, fmt.Errorf("持久化工具结果失败: %w", err))
			}
			if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventMessage, Message: toPersisted(toolMsg)}); err != nil {
				return err
			}
			if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventToolResult, ToolResult: &AssistantToolResult{
				CallID:  call.ID,
				Name:    call.Name,
				Content: content,
			}}); err != nil {
				return err
			}
			messages = append(messages, client.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    content,
			})
		}
	}

	// Loop budget exhausted: surface a soft fail to the user and close.
	if err := cfg.Sink(AssistantStreamEvent{Kind: StreamEventText, Text: fmt.Sprintf("\n\n（AI 在 %d 轮工具调用后仍未给出结论，已停止。请缩小问题范围或在下一轮中给出更明确的指令。）", MaxToolHops)}); err != nil {
		return err
	}
	return cfg.Sink(AssistantStreamEvent{Kind: StreamEventDone, Finish: "hop_budget_exhausted"})
}

// pickProvider falls back to the platform default when callerID is zero.
func (s *Service) pickProvider(ctx context.Context, callerID uuid.UUID) (*providerRow, error) {
	if callerID != uuid.Nil {
		row, err := s.repo.Get(ctx, callerID)
		if err != nil {
			return nil, err
		}
		if !row.Enabled {
			return nil, ErrProviderDisabled
		}
		return row, nil
	}
	row, err := s.repo.GetDefaultProvider(ctx)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, ErrProviderDisabled
	}
	return row, nil
}

// applyDecision runs (or skips) a mutating tool based on the user's call.
// On success it returns the tool's JSON output; on failure it returns a
// JSON-encoded `{"error": "..."}` blob so the model always sees structured
// content.
func (s *Service) applyDecision(ctx context.Context, cfg *streamConfig, call StoredToolCall, decision AssistantConfirmDecision) (string, bool) {
	if !decision.Approve {
		reason := strings.TrimSpace(decision.Reason)
		if reason == "" {
			reason = "用户拒绝执行该工具调用。请在不调用该工具的前提下继续作答或提出替代方案。"
		}
		payload := map[string]string{"error": reason, "rejected": "true"}
		body, _ := json.Marshal(payload)
		return string(body), true
	}

	tool, ok := cfg.Tools[call.Name]
	if !ok {
		return encodeToolError(fmt.Sprintf("未知工具: %s", call.Name)), true
	}
	if !tool.RequiresConfirm {
		return encodeToolError(fmt.Sprintf("工具 %s 不需要人工确认，拒绝通过确认接口执行", call.Name)), true
	}
	value, err := tool.Run(ctx, call.Arguments)
	if err != nil {
		return encodeToolError(err.Error()), true
	}
	body, err := json.Marshal(value)
	if err != nil {
		return encodeToolError(fmt.Sprintf("序列化工具结果失败: %s", err)), true
	}
	return string(body), false
}

// streamFail emits a final error event and surfaces the same err.
func (s *Service) streamFail(sink StreamCallback, err error) error {
	if sink == nil || err == nil {
		return err
	}
	_ = sink(AssistantStreamEvent{Kind: StreamEventError, Error: err.Error()})
	_ = sink(AssistantStreamEvent{Kind: StreamEventDone, Finish: "error"})
	return err
}

// buildClientMessages walks the persisted history and produces the
// client-side message list. Pending messages are filtered:
//   - pending_confirm rows are mid-flight requests that have not yet been
//     turned into a final tool result; if we replayed them the model would
//     re-request the same call without context.
//
// The leading system prompt is always inserted from assistantChatSystem
// (which we look up by going through buildMessages). For convenience we
// take advantage of the existing rawSystem fallback.
//
// pageContext (optional) is injected as a second system message right
// after the assistant prompt, so the model knows what the user is
// currently looking at without us persisting state that's better kept
// transient.
func buildClientMessages(history []StoredMessage, pageContext json.RawMessage, profileContext string, visionEnabled bool) []client.Message {
	out := make([]client.Message, 0, len(history)+3)
	out = append(out, client.Message{Role: "system", Content: assistantChatSystem})
	if profileContext != "" {
		out = append(out, client.Message{Role: "system", Content: "## 项目测试画像\n" + profileContext})
	}
	if msg, ok := renderPageContextSystem(pageContext); ok {
		out = append(out, msg)
	}
	for _, m := range history {
		switch m.Status {
		case storedStatusPendingConfirm:
			continue
		}
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

// renderPageContextSystem turns the frontend's opaque page snapshot into
// a system message the LLM can act on. We:
//   - ignore null/whitespace/literal "{}" inputs (no useful context);
//   - validate it parses as JSON (untrusted input — the frontend owns
//     the shape but we still avoid handing the model malformed bytes);
//   - re-encode with indentation so the model reads it more reliably
//     than a minified blob.
//
// The wrapper prose tells the model what the field means and explicitly
// reminds it that page context is hints, not authority over tool args
// (which are still validated server-side by aitools.ResolveProjectID).
func renderPageContextSystem(pageContext json.RawMessage) (client.Message, bool) {
	raw := bytes.TrimSpace([]byte(pageContext))
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return client.Message{}, false
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return client.Message{}, false
	}
	pretty, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return client.Message{}, false
	}
	body := strings.Join([]string{
		"## 用户当前页面上下文",
		"以下 JSON 是用户在前端正在浏览的页面状态（路由、当前看的资源 ID 等）。",
		"如果用户没明说要操作哪个对象，可以优先采用此处的 caseId / scenarioId / serviceId；",
		"但调用工具时仍需以工具参数为准，平台会在工具层再次校验权限与项目归属。",
		"```json",
		string(pretty),
		"```",
	}, "\n")
	return client.Message{Role: "system", Content: body}, true
}

// pickRequiresConfirm returns tool calls that must be approved by the user.
func pickRequiresConfirm(calls []client.ToolCall, tools map[string]aitools.Tool) []*AssistantToolCall {
	out := []*AssistantToolCall{}
	for _, c := range calls {
		tool, ok := tools[c.Name]
		if !ok || !tool.RequiresConfirm {
			continue
		}
		out = append(out, toAssistantCallPtr(c, tool))
	}
	return out
}

// pickAutoExecute returns read-only tools and mutating tools that do not
// require confirmation (create/update).
func pickAutoExecute(calls []client.ToolCall, tools map[string]aitools.Tool) []client.ToolCall {
	out := make([]client.ToolCall, 0, len(calls))
	for _, c := range calls {
		tool, ok := tools[c.Name]
		if !ok {
			continue
		}
		if tool.RequiresConfirm {
			continue
		}
		out = append(out, c)
	}
	return out
}

func toAssistantCallPtr(c client.ToolCall, tool aitools.Tool) *AssistantToolCall {
	ac := AssistantToolCall{
		ID:              c.ID,
		Name:            c.Name,
		Arguments:       c.Arguments,
		Mutating:        tool.Mutating,
		RequiresConfirm: tool.RequiresConfirm,
	}
	return &ac
}

// toStoredCalls produces the JSON-serialisable shape persisted in
// ai_messages.tool_calls.
func toStoredCalls(calls []client.ToolCall, tools map[string]aitools.Tool) []StoredToolCall {
	out := make([]StoredToolCall, 0, len(calls))
	for _, c := range calls {
		stored := StoredToolCall{
			ID:        c.ID,
			Name:      c.Name,
			Arguments: c.Arguments,
		}
		if t, ok := tools[c.Name]; ok {
			stored.Mutating = t.Mutating
			stored.RequiresConfirm = t.RequiresConfirm
		}
		out = append(out, stored)
	}
	return out
}

// toAssistantCall is the small bridge to the SSE event shape.
func toAssistantCall(c client.ToolCall, tools map[string]aitools.Tool) AssistantToolCall {
	ac := AssistantToolCall{
		ID:        c.ID,
		Name:      c.Name,
		Arguments: c.Arguments,
	}
	if t, ok := tools[c.Name]; ok {
		ac.Mutating = t.Mutating
		ac.RequiresConfirm = t.RequiresConfirm
	}
	return ac
}

// removeCall returns the persisted tool_calls JSON with the given call id
// stripped. If decoding fails the original payload is returned unchanged.
func removeCall(raw json.RawMessage, callID string) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var calls []StoredToolCall
	if err := json.Unmarshal(raw, &calls); err != nil {
		return raw
	}
	out := make([]StoredToolCall, 0, len(calls))
	for _, c := range calls {
		if c.ID == callID {
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return json.RawMessage("[]")
	}
	body, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return body
}

// keepOnlyCall returns a tool_calls JSON payload containing only the decided
// call. Keeping that pair is required by OpenAI-compatible tool protocols when
// the next request includes the corresponding role=tool result.
func keepOnlyCall(raw json.RawMessage, callID string) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var calls []StoredToolCall
	if err := json.Unmarshal(raw, &calls); err != nil {
		return raw
	}
	out := make([]StoredToolCall, 0, 1)
	for _, c := range calls {
		if c.ID == callID {
			out = append(out, c)
			break
		}
	}
	if len(out) == 0 {
		return json.RawMessage("[]")
	}
	body, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return body
}

func (cfg *streamConfig) buildHopUsage(hop int, model string, elapsedMillis int, usage client.TokenUsage) (json.RawMessage, *AssistantUsageDetail) {
	providerType := ""
	if cfg.Provider != nil {
		providerType = cfg.Provider.ProviderType
	}
	detail := usageDetailFromClient(providerType, model, hop, elapsedMillis, usage)
	return marshalUsageDetail(detail), detail
}

func (cfg *streamConfig) emitUsageIfNeeded(detail *AssistantUsageDetail) error {
	if !cfg.DebugEnabled || detail == nil {
		return nil
	}
	return cfg.Sink(AssistantStreamEvent{Kind: StreamEventUsage, Usage: detail})
}

// toPersisted is the StoredMessage -> PersistedMessage adapter.
func toPersisted(m *StoredMessage) *PersistedMessage {
	if m == nil {
		return nil
	}
	return &PersistedMessage{
		ID:           m.ID,
		Seq:          m.Seq,
		Role:         m.Role,
		Content:      m.Content,
		Attachments:  m.Attachments,
		ToolCallID:   m.ToolCallID,
		ToolCalls:    m.ToolCalls,
		Status:       m.Status,
		Model:        m.Model,
		UsageDetails: m.UsageDetails,
		CreatedAt:    m.CreatedAt,
	}
}
