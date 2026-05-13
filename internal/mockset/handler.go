package mockset

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/httpx"
	"autotest/internal/project"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the project-scoped mock value sets API.
type Handler struct {
	service        *Service
	projectHandler *project.Handler
}

// NewHandler constructs a Handler.
func NewHandler(service *Service, projectHandler *project.Handler) *Handler {
	return &Handler{service: service, projectHandler: projectHandler}
}

// Register mounts the routes under /projects/{projectID}/mock-value-sets.
//
// Read operations require viewer+; write operations require developer+ —
// matching SQL parameter sources and Mock Server.
func (h *Handler) Register(r chi.Router) {
	r.Route("/projects/{projectID}/mock-value-sets", func(r chi.Router) {
		r.Use(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer))

		r.Get("/", h.list)

		r.Group(func(r chi.Router) {
			r.Use(requireProjectRole(project.ProjectRoleDeveloper))
			r.Post("/", h.create)
			r.Put("/{setID}", h.update)
			r.Delete("/{setID}", h.delete)
		})
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	items, err := h.service.List(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	created, err := h.service.Create(r.Context(), projectID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	projectID, setID, ok := parseSetIDs(w, r)
	if !ok {
		return
	}
	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	updated, err := h.service.Update(r.Context(), projectID, setID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	projectID, setID, ok := parseSetIDs(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), projectID, setID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requireProjectRole(minRole project.ProjectRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := project.ProjectRoleFromContext(r.Context())
			if !ok || !project.ProjectRoleAtLeast(role, minRole) {
				httpx.Error(w, http.StatusForbidden, errors.New("insufficient project role"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseProjectID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return projectID, true
}

func parseSetIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	setID, err := uuid.Parse(chi.URLParam(r, "setID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, setID, true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	case errors.Is(err, ErrKeyConflict):
		httpx.Error(w, http.StatusConflict, err)
	case errors.Is(err, ErrInvalidKey),
		errors.Is(err, ErrInvalidName),
		errors.Is(err, ErrEmptyValues),
		errors.Is(err, ErrEmptyValueEntry),
		errors.Is(err, ErrValueNotValidJSON),
		errors.Is(err, ErrValueExtraJSON),
		errors.Is(err, ErrValueDepthExceeded),
		errors.Is(err, ErrValueTooManyNodes),
		errors.Is(err, ErrWeightsLengthMismatch),
		errors.Is(err, ErrNegativeWeight):
		httpx.Error(w, http.StatusBadRequest, err)
	default:
		httpx.Error(w, http.StatusInternalServerError, err)
	}
}
