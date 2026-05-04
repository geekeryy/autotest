package spec

import (
	"io"
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
	r.Get("/projects/{projectID}/services/{serviceID}/specs", h.listSpecs)
	r.Post("/projects/{projectID}/services/{serviceID}/specs/import", h.importSpec)
	r.Get("/projects/{projectID}/services/{serviceID}/endpoints", h.listEndpoints)
}

func (h *Handler) listSpecs(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	serviceID, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	specs, err := h.service.ListSpecs(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, specs)
}

func (h *Handler) importSpec(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	serviceID, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 20<<20))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	summary, err := h.service.Import(r.Context(), projectID, serviceID, data)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, summary)
}

func (h *Handler) listEndpoints(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	serviceID, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	endpoints, err := h.service.ListEndpoints(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, endpoints)
}
