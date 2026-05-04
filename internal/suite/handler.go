package suite

import (
	"encoding/json"
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
	r.Get("/suites", h.listSuites)
	r.Post("/suites", h.createSuite)
	r.Get("/suites/{suiteID}/items", h.listItems)
	r.Post("/suites/{suiteID}/items", h.addItem)
}

func (h *Handler) listSuites(w http.ResponseWriter, r *http.Request) {
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

	suites, err := h.service.List(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, suites)
}

func (h *Handler) createSuite(w http.ResponseWriter, r *http.Request) {
	var input CreateSuiteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	suite, err := h.service.Create(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, suite)
}

func (h *Handler) addItem(w http.ResponseWriter, r *http.Request) {
	suiteID, err := uuid.Parse(chi.URLParam(r, "suiteID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input AddItemInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	item, err := h.service.AddItem(r.Context(), suiteID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, item)
}

func (h *Handler) listItems(w http.ResponseWriter, r *http.Request) {
	suiteID, err := uuid.Parse(chi.URLParam(r, "suiteID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	items, err := h.service.ListItems(r.Context(), suiteID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}
