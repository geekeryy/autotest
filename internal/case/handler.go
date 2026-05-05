package testcase

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/httpx"

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
	r.Get("/cases", h.list)
	r.Get("/cases/{caseID}", h.get)
	r.Post("/cases", h.createManual)
	r.Patch("/cases/{caseID}", h.patch)
	r.Get("/cases/{caseID}/saved-cases", h.listSaved)
	r.Post("/cases/{caseID}/saved-cases", h.createSaved)
	r.Delete("/cases/{caseID}/saved-cases/{savedCaseID}", h.deleteSaved)
	r.Get("/cases/{caseID}/generate-params", h.generateParams)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	tc, err := h.service.Get(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, ErrTestCaseNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tc)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{}
	if projectID := r.URL.Query().Get("projectId"); projectID != "" {
		id, err := uuid.Parse(projectID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		filter.ProjectID = id
	}
	if serviceID := r.URL.Query().Get("serviceId"); serviceID != "" {
		id, err := uuid.Parse(serviceID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		filter.ServiceID = id
	}

	cases, err := h.service.List(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cases)
}

func (h *Handler) listSaved(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	cases, err := h.service.ListSaved(r.Context(), caseID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cases)
}

func (h *Handler) generateParams(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	params, err := h.service.GenerateParams(r.Context(), caseID)
	if err != nil {
		if errors.Is(err, ErrTestCaseNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusUnprocessableEntity, err)
		return
	}
	httpx.JSON(w, http.StatusOK, params)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input PatchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	tc, err := h.service.Patch(r.Context(), caseID, input)
	if err != nil {
		if errors.Is(err, ErrTestCaseNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tc)
}

func (h *Handler) createManual(w http.ResponseWriter, r *http.Request) {
	var input CreateManualInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	tc, err := h.service.CreateManual(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, tc)
}

func (h *Handler) createSaved(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input CreateSavedInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	tc, err := h.service.CreateSaved(r.Context(), caseID, input)
	if err != nil {
		if errors.Is(err, ErrTestCaseNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, tc)
}

func (h *Handler) deleteSaved(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	savedCaseID, err := uuid.Parse(chi.URLParam(r, "savedCaseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if err := h.service.DeleteSaved(r.Context(), caseID, savedCaseID); err != nil {
		if errors.Is(err, ErrTestCaseNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
