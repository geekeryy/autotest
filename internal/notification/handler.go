package notification

import (
	"errors"
	"net/http"
	"strconv"
	"sync"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler 暴露用户级站内通知 REST 接口；由 cmd/api/main.go 注册在 RejectAPIKey 守卫之后。
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 在指定路由组下挂载通知接口；仅 JWT 用户可访问。
func (h *Handler) Register(r chi.Router) {
	r.Get("/notifications", h.list)
	r.Get("/notifications/stream", h.stream)
	r.Patch("/notifications/{id}/read", h.markRead)
	r.Post("/notifications/read-all", h.markAllRead)
	r.Post("/notifications/clear-all", h.clearAll)
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}

	flusher, err := startNotificationSSE(w)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	var mu sync.Mutex
	done := make(chan struct{})
	defer close(done)
	runNotificationHeartbeat(r.Context(), w, flusher, &mu, done)

	limit := defaultListLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	snapshot, err := h.svc.StreamSnapshot(r.Context(), principal.UserID, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if err := writeStreamEvent(w, flusher, &mu, StreamEvent{
		Kind:        "snapshot",
		UnreadCount: snapshot.UnreadCount,
		Items:       snapshot.Items,
	}); err != nil {
		return
	}

	events, unsubscribe := h.svc.Subscribe(principal.UserID)
	defer unsubscribe()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := writeStreamEvent(w, flusher, &mu, event); err != nil {
				return
			}
		}
	}
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}

	limit := defaultListLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}

	resp, err := h.svc.List(r.Context(), principal.UserID, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.MarkRead(r.Context(), principal.UserID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	if err := h.svc.MarkAllRead(r.Context(), principal.UserID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) clearAll(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	if err := h.svc.ClearAll(r.Context(), principal.UserID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
