package aisession

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/auth"
	"autotest/internal/httpx"
	"autotest/internal/project"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the AI assistant session CRUD endpoints. The chat stream
// and tool-call confirmation endpoints live in `internal/aiprovider`
// because they are tightly coupled to provider flow control.
type Handler struct {
	service        *Service
	projectHandler *project.Handler
}

// NewHandler constructs a Handler.
func NewHandler(service *Service, projectHandler *project.Handler) *Handler {
	return &Handler{service: service, projectHandler: projectHandler}
}

// Register mounts session routes under /projects/{projectID}/ai/sessions.
// All routes require project viewer role at minimum.
func (h *Handler) Register(r chi.Router) {
	r.Route("/projects/{projectID}/ai/sessions", func(r chi.Router) {
		r.Use(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer))
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{sessionID}", h.get)
		r.Patch("/{sessionID}", h.rename)
		r.Delete("/{sessionID}", h.delete)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	projectID, userID, ok := h.ids(w, r)
	if !ok {
		return
	}
	items, err := h.service.ListSessions(r.Context(), projectID, userID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	projectID, userID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var input CreateSessionInput
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
	}
	session, err := h.service.CreateSession(r.Context(), projectID, userID, input)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, session)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	projectID, userID, ok := h.ids(w, r)
	if !ok {
		return
	}
	sessionID, ok := parseSessionID(w, r)
	if !ok {
		return
	}
	session, messages, err := h.service.GetSession(r.Context(), projectID, userID, sessionID)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"session":  session,
		"messages": messages,
	})
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request) {
	projectID, userID, ok := h.ids(w, r)
	if !ok {
		return
	}
	sessionID, ok := parseSessionID(w, r)
	if !ok {
		return
	}
	var input UpdateSessionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	session, err := h.service.RenameSession(r.Context(), projectID, userID, sessionID, input)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, session)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	projectID, userID, ok := h.ids(w, r)
	if !ok {
		return
	}
	sessionID, ok := parseSessionID(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteSession(r.Context(), projectID, userID, sessionID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ids(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 projectId"))
		return uuid.Nil, uuid.Nil, false
	}
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, principal.UserID, true
}

func parseSessionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 sessionId"))
		return uuid.Nil, false
	}
	return id, true
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSessionNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	case errors.Is(err, ErrSessionForbidden):
		httpx.Error(w, http.StatusForbidden, err)
	case errors.Is(err, ErrPendingNotFound), errors.Is(err, ErrPendingDecided):
		httpx.Error(w, http.StatusConflict, err)
	default:
		httpx.Error(w, http.StatusInternalServerError, err)
	}
}
