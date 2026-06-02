package ratelimit

import (
	"encoding/json"
	"net/http"

	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
)

// Handler exposes rate limit management endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register mounts rate limit routes.
func (h *Handler) Register(r chi.Router) {
	r.Route("/ai/rate-limits", func(r chi.Router) {
		r.Get("/rules", h.listRules)
		r.Post("/rules", h.createRule)
		r.Post("/cleanup", h.cleanup)
	})
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.ListRules(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rules)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var input RateLimitRule
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	rule, err := h.service.CreateRule(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rule)
}

func (h *Handler) cleanup(w http.ResponseWriter, r *http.Request) {
	count, err := h.service.Cleanup(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"deleted": count})
}
