package aiprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/aiprovider/client"
	"autotest/internal/aitools"
	"autotest/internal/auth"
	"autotest/internal/httpx"
	"autotest/internal/project"
	"autotest/internal/projectprompt"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// promptService is a minimal interface to decouple the aiprovider handler from the projectprompt package.
type promptService interface {
	GetByAction(ctx context.Context, action string) (*projectprompt.ProjectPrompt, error)
}

// Handler exposes the AI provider management API and the AI chat invocation endpoint.
type Handler struct {
	service        *Service
	projectHandler *project.Handler
	promptSvc      promptService
	authSvc        *auth.Service
	// Conversational assistant plumbing. Both fields are optional — when
	// nil the streaming routes refuse with 503.
	sessionStore   SessionStore
	assistantTools []aitools.Tool
}

// NewHandler constructs a Handler.
func NewHandler(service *Service, projectHandler *project.Handler, promptSvc promptService, authSvc *auth.Service) *Handler {
	return &Handler{service: service, projectHandler: projectHandler, promptSvc: promptSvc, authSvc: authSvc}
}

// Register mounts platform AI provider routes and project-scoped chat routes.
func (h *Handler) Register(r chi.Router) {
	r.Get("/ai-provider-types", h.listTypes)

	r.Route("/ai-providers", func(r chi.Router) {
		r.Use(h.authSvc.RequirePermission(auth.PermissionProjectsRead))
		r.Get("/", h.list)
		r.Get("/{providerID}/models", h.listModels)

		r.Group(func(r chi.Router) {
			r.Use(h.authSvc.RequirePermission(auth.PermissionProjectsWrite))
			r.Post("/models/discover", h.discoverModels)
			r.Post("/", h.create)
			r.Put("/{providerID}", h.update)
			r.Delete("/{providerID}", h.delete)
			r.Post("/{providerID}/test", h.test)
		})
	})

	r.Route("/projects/{projectID}/ai", func(r chi.Router) {
		r.Use(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer))
		r.Post("/chat", h.chat)
		r.Post("/chat/stream", h.chatStream)
		r.Post("/tool-calls/{callID}/confirm", h.confirmToolCall)
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

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	providerID, ok := parseProviderID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ListModels(r.Context(), providerID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) discoverModels(w http.ResponseWriter, r *http.Request) {
	var input DiscoverModelsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.service.DiscoverModels(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	providerID, ok := parseProviderID(w, r)
	if !ok {
		return
	}
	resp, err := h.service.TestConnection(r.Context(), providerID)
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
		if cfg, err := h.promptSvc.GetByAction(r.Context(), req.Action); err == nil && cfg != nil {
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

func parseProjectID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return projectID, true
}

func parseProviderID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	providerID, err := uuid.Parse(chi.URLParam(r, "providerID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return providerID, true
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
