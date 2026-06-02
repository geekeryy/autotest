package promptlayer

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes prompt layer management endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register mounts prompt layer routes.
func (h *Handler) Register(r chi.Router) {
	r.Route("/ai/prompt-layers", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/{layerID}", h.get)
		r.Put("/{layerID}/enable", h.enableVersion)
		r.Delete("/{layerID}", h.delete)
		r.Get("/domain/{domain}", h.domainPrompt)
	})
	r.Route("/ai/prompt-layers/by-name/{name}", func(r chi.Router) {
		r.Get("/latest", h.getLatest)
		r.Get("/versions/{version}", h.getByVersion)
		r.Put("/", h.update)
	})
	// A/B test experiment endpoints
	r.Route("/ai/prompt-experiments", func(r chi.Router) {
		r.Get("/", h.listExperiments)
		r.Post("/", h.createExperiment)
		r.Put("/{experimentID}/stop", h.stopExperiment)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	opts := ListLayersOptions{
		Domain: r.URL.Query().Get("domain"),
		Limit:  50,
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.Limit = n
		}
	}
	if v := r.URL.Query().Get("enabled"); v != "" {
		b := v == "true" || v == "1"
		opts.Enabled = &b
	}
	items, err := h.service.ListLayers(r.Context(), opts)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input CreateLayerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	layer, err := h.service.CreateLayer(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, layer)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "layerID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 layerId"))
		return
	}
	layer, err := h.service.GetLayer(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, layer)
}

func (h *Handler) getLatest(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	layer, err := h.service.GetLatestVersion(r.Context(), name)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, layer)
}

func (h *Handler) getByVersion(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	version, err := strconv.Atoi(chi.URLParam(r, "version"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 version"))
		return
	}
	layer, err := h.service.GetByVersion(r.Context(), name, version)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, layer)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var input UpdateLayerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	layer, err := h.service.UpdateLayer(r.Context(), name, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, layer)
}

func (h *Handler) enableVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "layerID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 layerId"))
		return
	}
	// Get the layer to find its name and version
	layer, err := h.service.GetLayer(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	if err := h.service.EnableVersion(r.Context(), layer.Name, layer.Version); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "layerID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 layerId"))
		return
	}
	if err := h.service.DeleteLayer(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) domainPrompt(w http.ResponseWriter, r *http.Request) {
	domain := chi.URLParam(r, "domain")
	prompt, err := h.service.BuildDomainPrompt(r.Context(), domain)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"prompt": prompt})
}

func (h *Handler) listExperiments(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	items, err := h.service.ListExperiments(r.Context(), status)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) createExperiment(w http.ResponseWriter, r *http.Request) {
	var input CreateExperimentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	exp, err := h.service.CreateExperiment(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, exp)
}

func (h *Handler) stopExperiment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "experimentID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 experimentId"))
		return
	}
	if err := h.service.StopExperiment(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}
