package aiconfig

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
)

// Handler exposes platform AI assistant settings API.
type Handler struct {
	svc     *Service
	authSvc *auth.Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

// Register mounts GET/PUT /ai-assistant-settings under platform permissions.
func (h *Handler) Register(r chi.Router) {
	r.Route("/ai-assistant-settings", func(r chi.Router) {
		r.Use(h.authSvc.RequirePermission(auth.PermissionProjectsRead))
		r.Get("/", h.get)
		r.Group(func(r chi.Router) {
			r.Use(h.authSvc.RequirePermission(auth.PermissionProjectsWrite))
			r.Put("/", h.update)
		})
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.GetView(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	view, err := h.svc.Update(r.Context(), input)
	if err != nil {
		writeUpdateError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

func writeUpdateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidMaxToolHops),
		errors.Is(err, ErrInvalidMaxRounds),
		errors.Is(err, ErrInvalidConfidence),
		errors.Is(err, ErrInvalidToolRoutingMode):
		httpx.Error(w, http.StatusBadRequest, err)
	default:
		httpx.Error(w, http.StatusInternalServerError, err)
	}
}
