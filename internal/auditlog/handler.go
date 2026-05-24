package auditlog

import (
	"net/http"
	"strconv"

	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
)

// Handler 暴露审计日志 REST 接口。
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 在指定路由组下挂载审计日志接口。
func (h *Handler) Register(r chi.Router) {
	r.Get("/audit-logs", h.list)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{
		Action:        r.URL.Query().Get("action"),
		ActorUsername: r.URL.Query().Get("username"),
	}
	if raw := r.URL.Query().Get("success"); raw != "" {
		v := raw == "true" || raw == "1"
		filter.Success = &v
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			filter.Limit = n
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			filter.Offset = n
		}
	}

	resp, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
