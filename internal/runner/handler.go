package runner

import (
	"encoding/json"
	"errors"
	"net/http"

	testcase "autotest/internal/case"
	"autotest/internal/httpx"
	"autotest/internal/project"
	"autotest/internal/report"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(r chi.Router) {
	r.Post("/cases/{caseID}/run", h.runCase)
	r.Get("/runs/{runID}/result", h.getRunResult)
}

func (h *Handler) getRunResult(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	output, err := h.service.GetRunResult(r.Context(), runID)
	if err != nil {
		if errors.Is(err, report.ErrRunNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, output)
}

func (h *Handler) runCase(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input RunCaseInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	output, err := h.service.RunCase(r.Context(), caseID, input)
	if err != nil {
		switch {
		case errors.Is(err, testcase.ErrTestCaseNotFound), errors.Is(err, project.ErrEnvironmentNotFound):
			httpx.Error(w, http.StatusNotFound, err)
		case errors.Is(err, project.ErrDatabaseUnavailable):
			httpx.Error(w, http.StatusInternalServerError, err)
		default:
			httpx.Error(w, http.StatusBadRequest, err)
		}
		return
	}
	httpx.JSON(w, http.StatusOK, output)
}
