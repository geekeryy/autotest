package aiprovider

// This file mounts the SSE endpoints for the global assistant chat. The
// transport is intentionally narrow: one bidirectional protocol (POST +
// SSE response) for two stages — initial chat and confirmation-driven
// resume. Both stages share a single SSE event schema defined in
// assistant.go (AssistantStreamEvent).
//
// The handler stays lean — all conversation logic lives in
// service.ChatStreamWithTools / service.ContinueAfterConfirm. We are
// merely:
//
//   1. Parsing the incoming JSON body.
//   2. Setting up SSE writer plumbing (flush, headers, heartbeat).
//   3. Adapting the service-level StreamCallback to actual SSE frames.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"autotest/internal/aitools"
	"autotest/internal/auth"
	"autotest/internal/httpx"
	"autotest/internal/project"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// withAssistantCaller stamps a CallerContext onto ctx so individual
// tools can reject cross-project / unauthenticated calls without each
// reaching for *http.Request. We do this at the SSE boundary (where we
// already have both the project route param and the auth principal)
// rather than pushing tunneling parameters down to every tool's
// signature.
func withAssistantCaller(ctx context.Context, projectID uuid.UUID, principal *auth.Principal) context.Context {
	cc := aitools.CallerContext{ProjectID: projectID}
	if principal != nil {
		cc.UserID = principal.UserID
		cc.Username = principal.Username
	}
	return aitools.WithCaller(ctx, cc)
}

// WithAssistant equips the handler with conversational-chat plumbing.
// Calling this enables /ai/chat/stream and /ai/tool-calls/{callID}/confirm
// routes during Register.
func (h *Handler) WithAssistant(store SessionStore, tools []aitools.Tool) *Handler {
	h.sessionStore = store
	h.assistantTools = tools
	return h
}

// chatStream is the long-running SSE endpoint for the conversational AI.
// The HTTP request body is a JSON AssistantStreamRequest; the response is
// a sequence of `event:` / `data:` frames terminating with a `done` event.
func (h *Handler) chatStream(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	if h.sessionStore == nil {
		httpx.Error(w, http.StatusServiceUnavailable, errors.New("assistant 未启用"))
		return
	}

	var req AssistantStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.SessionID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("sessionId 不能为空"))
		return
	}

	flusher, err := startSSE(w)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	callerCtx := withAssistantCaller(r.Context(), projectID, principal)
	sink, stop := buildSSESink(w, flusher, callerCtx)
	defer stop()

	if err := h.service.ChatStreamWithTools(
		callerCtx, projectID, principal.UserID, req,
		h.sessionStore, h.assistantTools, sink,
	); err != nil {
		// 错误事件已经在 ChatStreamWithTools 内部通过 sink 发出；这里
		// 只是日志记录路径，HTTP 状态保持 200（SSE 已开始）。
		_ = err
	}
}

// confirmToolCall resumes a previously-suspended stream with the user's
// approve/reject decision. Body shape:
//
//	{
//	  "sessionId": "uuid",
//	  "approve": true,
//	  "reason": "...",     // optional, surfaced to the model when reject
//	  "providerId": "uuid" // optional; defaults to the project's default
//	}
//
// The response is the same SSE stream emitted by /chat/stream.
func (h *Handler) confirmToolCall(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	callID := strings.TrimSpace(chi.URLParam(r, "callID"))
	if callID == "" {
		httpx.Error(w, http.StatusBadRequest, errors.New("callId 不能为空"))
		return
	}
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	if h.sessionStore == nil {
		httpx.Error(w, http.StatusServiceUnavailable, errors.New("assistant 未启用"))
		return
	}

	var body struct {
		SessionID       uuid.UUID       `json:"sessionId"`
		Approve         bool            `json:"approve"`
		Reason          string          `json:"reason"`
		ProviderID      uuid.UUID       `json:"providerId"`
		Model           string          `json:"model,omitempty"`
		ThinkingEnabled  *bool           `json:"thinkingEnabled,omitempty"`
		ReasoningEffort  string          `json:"reasoningEffort,omitempty"`
		WebSearchEnabled *bool           `json:"webSearchEnabled,omitempty"`
		PageContext      json.RawMessage `json:"pageContext,omitempty"`
		DebugEnabled     *bool           `json:"debugEnabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if body.SessionID == uuid.Nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("sessionId 不能为空"))
		return
	}

	flusher, err := startSSE(w)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	callerCtx := withAssistantCaller(r.Context(), projectID, principal)
	sink, stop := buildSSESink(w, flusher, callerCtx)
	defer stop()

	decision := AssistantConfirmDecision{Approve: body.Approve, Reason: body.Reason}
	if err := h.service.ContinueAfterConfirm(
		callerCtx, projectID, principal.UserID,
		body.SessionID, callID, decision,
		body.ProviderID, body.Model, body.ThinkingEnabled, body.ReasoningEffort, body.WebSearchEnabled, body.PageContext, body.DebugEnabled,
		h.sessionStore, h.assistantTools, sink,
	); err != nil {
		_ = err
	}
}

// startSSE sets the headers required for an SSE response and grabs a
// flusher to push frames immediately. Returns an error if the underlying
// http.ResponseWriter does not support flushing (which would make
// streaming pointless).
func startSSE(w http.ResponseWriter) (http.Flusher, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming unsupported by current writer")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx 反向代理时关闭缓冲
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return flusher, nil
}

// buildSSESink wires an AssistantStreamEvent callback to the SSE writer.
// A background heartbeat keeps idle connections alive across proxies that
// drop quiet TCP streams (NGINX, AWS ALB, ...). The returned stop()
// closure must be invoked from the handler to terminate the heartbeat.
func buildSSESink(w http.ResponseWriter, flusher http.Flusher, ctx context.Context) (StreamCallback, func()) {
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				_, _ = fmt.Fprint(w, ": ping\n\n")
				flusher.Flush()
				mu.Unlock()
			}
		}
	}()

	stop := func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	sink := func(event AssistantStreamEvent) error {
		mu.Lock()
		defer mu.Unlock()
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal sse event: %w", err)
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", event.Kind); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	return sink, stop
}

// requireProjectViewer is used by Register; declared here to keep the
// assistant handler self-contained.
func (h *Handler) requireProjectViewer() func(http.Handler) http.Handler {
	return h.projectHandler.RequireProjectRole(project.ProjectRoleViewer)
}
