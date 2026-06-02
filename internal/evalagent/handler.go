package evalagent

import (
	"net/http"
	"strconv"
	"time"

	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes evaluation endpoints.
type Handler struct {
	evaluator *Evaluator
	repo      *Repository
}

// NewHandler constructs a Handler.
func NewHandler(evaluator *Evaluator, repo *Repository) *Handler {
	return &Handler{evaluator: evaluator, repo: repo}
}

// Register mounts evaluation routes.
func (h *Handler) Register(r chi.Router) {
	r.Route("/ai/eval", func(r chi.Router) {
		r.Post("/run", h.runEvaluation)
		r.Get("/reports", h.listReports)
		r.Get("/reports/{reportID}", h.getReport)
	})
}

func (h *Handler) runEvaluation(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-7 * 24 * time.Hour)
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}
	report, err := h.evaluator.RunEvaluation(r.Context(), since)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, report)
}

func (h *Handler) listReports(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	reports, err := h.repo.List(r.Context(), limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, reports)
}

func (h *Handler) getReport(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "reportID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	report, err := h.repo.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, report)
}
