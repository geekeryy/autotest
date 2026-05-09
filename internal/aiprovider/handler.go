package aiprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/aiprovider/client"
	"autotest/internal/httpx"
	"autotest/internal/project"
	"autotest/internal/projectprompt"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// promptService is a minimal interface to decouple the aiprovider handler from the projectprompt package.
type promptService interface {
	GetByAction(ctx context.Context, projectID uuid.UUID, action string) (*projectprompt.ProjectPrompt, error)
}

// Handler exposes the AI provider management API and the AI chat invocation endpoint.
type Handler struct {
	service        *Service
	projectHandler *project.Handler
	promptSvc      promptService
}

// NewHandler constructs a Handler.
func NewHandler(service *Service, projectHandler *project.Handler, promptSvc promptService) *Handler {
	return &Handler{service: service, projectHandler: projectHandler, promptSvc: promptSvc}
}

// Register mounts the routes under /projects/{projectID}/...
func (h *Handler) Register(r chi.Router) {
	r.Get("/ai-provider-types", h.listTypes)

	r.Route("/projects/{projectID}/ai-providers", func(r chi.Router) {
		r.Use(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer))

		r.Get("/", h.list)

		r.Group(func(r chi.Router) {
			r.Use(requireProjectRole(project.ProjectRoleDeveloper))
			r.Post("/", h.create)
			r.Put("/{providerID}", h.update)
			r.Delete("/{providerID}", h.delete)
			r.Post("/{providerID}/test", h.test)
		})
	})

	r.Route("/projects/{projectID}/ai", func(r chi.Router) {
		r.Use(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer))
		r.Post("/chat", h.chat)
	})
}

func (h *Handler) listTypes(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, h.service.SupportedTypes())
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
	projectID, providerID, ok := parseProviderIDs(w, r)
	if !ok {
		return
	}
	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	updated, err := h.service.Update(r.Context(), projectID, providerID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	projectID, providerID, ok := parseProviderIDs(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), projectID, providerID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	projectID, providerID, ok := parseProviderIDs(w, r)
	if !ok {
		return
	}
	resp, err := h.service.TestConnection(r.Context(), projectID, providerID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if h.promptSvc != nil {
		if cfg, err := h.promptSvc.GetByAction(r.Context(), projectID, req.Action); err == nil && cfg != nil {
			if cfg.Enabled && cfg.SystemPrompt != "" {
				req.SystemPromptOverride = cfg.SystemPrompt
			}
		}
	}
	resp, err := h.service.Chat(r.Context(), projectID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
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

func parseProviderIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	providerID, err := uuid.Parse(chi.URLParam(r, "providerID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, providerID, true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrProviderNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	case errors.Is(err, ErrProviderTypeInvalid),
		errors.Is(err, ErrProviderActionInvalid),
		errors.Is(err, ErrProviderEmptyKey),
		errors.Is(err, ErrAssertionIntentRequired):
		httpx.Error(w, http.StatusBadRequest, err)
	case errors.Is(err, ErrProviderDisabled):
		httpx.Error(w, http.StatusConflict, err)
	case errors.Is(err, client.ErrUnsupportedProvider):
		httpx.Error(w, http.StatusBadRequest, err)
	default:
		httpx.Error(w, http.StatusBadGateway, err)
	}
}
