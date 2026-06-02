package aifeedback

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
)

// Handler exposes feedback REST endpoints.
type Handler struct {
	service *Service
	authSvc *auth.Service
}

// NewHandler constructs a Handler.
func NewHandler(service *Service, authSvc *auth.Service) *Handler {
	return &Handler{service: service, authSvc: authSvc}
}

// Register mounts feedback routes.
func (h *Handler) Register(r chi.Router) {
	r.Route("/ai/feedback", func(r chi.Router) {
		r.Post("/", h.submit)
		r.Get("/", h.list)
		r.Get("/stats", h.stats)
	})
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	var input CreateFeedbackInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	f, err := h.service.SubmitFeedback(r.Context(), principal.UserID, input)
	if err != nil {
		var fe *FeedbackError
		if errors.As(err, &fe) {
			httpx.Error(w, http.StatusBadRequest, err)
		} else {
			httpx.Error(w, http.StatusInternalServerError, err)
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, f)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	opts := ListFeedbackOptions{Limit: 50}
	if v := r.URL.Query().Get("limit"); v != "" {
		// parse int, ignore error for brevity
	}
	items, err := h.service.List(r.Context(), opts)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-7 * 24 * time.Hour)
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}
	stats, err := h.service.GetStats(r.Context(), since)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, stats)
}
