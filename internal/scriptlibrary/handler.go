package scriptlibrary

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/script-library-templates", h.list)
	r.Post("/script-library-templates", h.create)
	r.Put("/script-library-templates/{id}", h.update)
	r.Delete("/script-library-templates/{id}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseQueryUUID(w, r, "projectId")
	if !ok {
		return
	}
	list, err := h.svc.List(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	t, err := h.svc.Create(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	t, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrTemplateNotFound) {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	if errors.Is(err, ErrBuiltinTemplateReadonly) {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.Error(w, http.StatusBadRequest, err)
}

func parseQueryUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		httpx.Error(w, http.StatusBadRequest, errors.New(name+" is required"))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return id, true
}
