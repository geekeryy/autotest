package authprovider

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes auth provider management API.
type Handler struct {
	service            *Service
	requireUsersManage func(http.Handler) http.Handler
}

// NewHandler constructs a Handler. requireUsersManage should enforce users:manage permission.
func NewHandler(service *Service, requireUsersManage func(http.Handler) http.Handler) *Handler {
	return &Handler{service: service, requireUsersManage: requireUsersManage}
}

// Register mounts admin routes (requires users:manage).
func (h *Handler) Register(r chi.Router) {
	r.Route("/", func(r chi.Router) {
		r.Use(h.requireUsersManage)
		r.Get("/auth-provider-types", h.listTypes)
		r.Route("/auth-providers", func(r chi.Router) {
			r.Get("/", h.list)
			r.Post("/", h.create)
			r.Put("/{providerID}", h.update)
			r.Delete("/{providerID}", h.delete)
			r.Post("/{providerID}/test", h.test)
		})
	})
}

func (h *Handler) listTypes(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, h.service.SupportedTypes())
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	created, err := h.service.Create(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	providerID, ok := parseProviderID(w, r)
	if !ok {
		return
	}
	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	updated, err := h.service.Update(r.Context(), providerID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	providerID, ok := parseProviderID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), providerID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	providerID, ok := parseProviderID(w, r)
	if !ok {
		return
	}
	result, err := h.service.Test(r.Context(), providerID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func parseProviderID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "providerID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return id, true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrProviderNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	case errors.Is(err, ErrProviderTypeInvalid),
		errors.Is(err, ErrEmptyClientSecret),
		errors.Is(err, ErrSlugInvalid),
		errors.Is(err, ErrSlugDuplicate),
		errors.Is(err, ErrCallbackURLInvalid),
		errors.Is(err, ErrFrontendOriginInvalid):
		httpx.Error(w, http.StatusBadRequest, err)
	case errors.Is(err, ErrProviderDisabled):
		httpx.Error(w, http.StatusConflict, err)
	default:
		if err != nil && strings.Contains(err.Error(), "required") {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
	}
}
