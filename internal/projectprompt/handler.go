package projectprompt

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the platform AI prompt management API.
type Handler struct {
	service *Service
	authSvc *auth.Service
}

// NewHandler constructs a Handler.
func NewHandler(service *Service, authSvc *auth.Service) *Handler {
	return &Handler{service: service, authSvc: authSvc}
}

// Register mounts the routes under /ai-prompts.
func (h *Handler) Register(r chi.Router) {
	r.Route("/ai-prompts", func(r chi.Router) {
		r.Use(h.authSvc.RequirePermission(auth.PermissionProjectsRead))
		r.Get("/", h.list)

		r.Group(func(r chi.Router) {
			r.Use(h.authSvc.RequirePermission(auth.PermissionProjectsWrite))
			r.Post("/", h.create)
			r.Put("/{promptID}", h.update)
			r.Delete("/{promptID}", h.delete)
		})
	})
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
	promptID, ok := parsePromptID(w, r)
	if !ok {
		return
	}
	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	updated, err := h.service.Update(r.Context(), promptID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	promptID, ok := parsePromptID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), promptID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parsePromptID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	promptID, err := uuid.Parse(chi.URLParam(r, "promptID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return promptID, true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	default:
		httpx.Error(w, http.StatusBadRequest, err)
	}
}
