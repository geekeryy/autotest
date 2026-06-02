package aimemory

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes AI memory CRUD endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register mounts AI memory routes.
func (h *Handler) Register(r chi.Router) {
	r.Route("/ai/memories", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.save)
		r.Post("/recall", h.recall)
		r.Get("/{memoryID}", h.get)
		r.Put("/{memoryID}", h.update)
		r.Delete("/{memoryID}", h.delete)
	})
	r.Route("/projects/{projectID}/ai/memories", func(r chi.Router) {
		r.Get("/", h.listByProject)
		r.Post("/", h.saveToProject)
		r.Post("/recall", h.recallProject)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	opts := parseListOptions(r)
	memories, total, err := h.service.List(r.Context(), uuid.Nil, principal.UserID, opts)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": memories, "total": total})
}

func (h *Handler) listByProject(w http.ResponseWriter, r *http.Request) {
	projectID, userID, ok := projectIDs(w, r)
	if !ok {
		return
	}
	opts := parseListOptions(r)
	memories, total, err := h.service.List(r.Context(), projectID, userID, opts)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": memories, "total": total})
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	var input SaveMemoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	mem, err := h.service.Save(r.Context(), uuid.Nil, principal.UserID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, mem)
}

func (h *Handler) saveToProject(w http.ResponseWriter, r *http.Request) {
	projectID, userID, ok := projectIDs(w, r)
	if !ok {
		return
	}
	var input SaveMemoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	mem, err := h.service.Save(r.Context(), projectID, userID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, mem)
}

func (h *Handler) recall(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	var body struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	results, err := h.service.Recall(r.Context(), uuid.Nil, principal.UserID, body.Query, body.Limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, results)
}

func (h *Handler) recallProject(w http.ResponseWriter, r *http.Request) {
	projectID, userID, ok := projectIDs(w, r)
	if !ok {
		return
	}
	var body struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	results, err := h.service.Recall(r.Context(), projectID, userID, body.Query, body.Limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, results)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "memoryID")
	if !ok {
		return
	}
	mem, err := h.service.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, mem)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "memoryID")
	if !ok {
		return
	}
	var input UpdateMemoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	mem, err := h.service.Update(r.Context(), id, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, mem)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "memoryID")
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func projectIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
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

func parseUUID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 "+param))
		return uuid.Nil, false
	}
	return id, true
}

func parseListOptions(r *http.Request) ListOptions {
	opts := ListOptions{}
	if t := r.URL.Query().Get("type"); t != "" {
		opts.Type = t
	}
	if c := r.URL.Query().Get("category"); c != "" {
		opts.Category = c
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			opts.Offset = n
		}
	}
	if opts.Limit == 0 {
		opts.Limit = 50
	}
	return opts
}
