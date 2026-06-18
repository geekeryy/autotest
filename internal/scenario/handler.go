package scenario

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
	r.Get("/scenarios", h.list)
	r.Post("/scenarios", h.create)
	r.Get("/scenarios/{scenarioID}", h.get)
	r.Put("/scenarios/{scenarioID}", h.update)
	r.Delete("/scenarios/{scenarioID}", h.delete)
	r.Get("/scenarios/{scenarioID}/steps", h.listSteps)
	r.Put("/scenarios/{scenarioID}/steps/reorder", h.reorderSteps)
	r.Put("/scenarios/{scenarioID}/steps/{stepOrder}", h.upsertStep)
	r.Delete("/scenarios/{scenarioID}/steps/{stepID}", h.deleteStep)
}

// RegisterAPIKeyRead 挂载 API Key 场景只读路由（需 scenarios:read 作用域组守卫）。
func (h *Handler) RegisterAPIKeyRead(r chi.Router) {
	r.Get("/scenarios", h.list)
	r.Get("/scenarios/{scenarioID}", h.get)
	r.Get("/scenarios/{scenarioID}/steps", h.listSteps)
}

// RegisterAPIKeyWrite 挂载 API Key 场景写入路由（需 scenarios:write 作用域组守卫）。
func (h *Handler) RegisterAPIKeyWrite(r chi.Router) {
	r.Post("/scenarios", h.create)
	r.Put("/scenarios/{scenarioID}", h.update)
	r.Delete("/scenarios/{scenarioID}", h.delete)
	r.Put("/scenarios/{scenarioID}/steps/reorder", h.reorderSteps)
	r.Put("/scenarios/{scenarioID}/steps/{stepOrder}", h.upsertStep)
	r.Delete("/scenarios/{scenarioID}/steps/{stepID}", h.deleteStep)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	filter := ListFilter{}
	if projectID := r.URL.Query().Get("projectId"); projectID != "" {
		id, err := uuid.Parse(projectID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		filter.ProjectID = id
	}
	if serviceID := r.URL.Query().Get("serviceId"); serviceID != "" {
		id, err := uuid.Parse(serviceID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		filter.ServiceID = id
	}
	list, err := h.svc.List(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input CreateScenarioInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	sc, err := h.svc.Create(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, sc)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "scenarioID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	sc, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrScenarioNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sc)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "scenarioID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	var input UpdateScenarioInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	sc, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, ErrScenarioNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sc)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "scenarioID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrScenarioNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) reorderSteps(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := uuid.Parse(chi.URLParam(r, "scenarioID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	var input ReorderStepsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.ReorderSteps(r.Context(), scenarioID, input); err != nil {
		if errors.Is(err, ErrScenarioNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, ErrStepNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listSteps(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "scenarioID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	steps, err := h.svc.ListSteps(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, steps)
}

func (h *Handler) upsertStep(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := uuid.Parse(chi.URLParam(r, "scenarioID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	stepOrder, err := parsePositiveInt(chi.URLParam(r, "stepOrder"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input UpsertStepInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	input.StepOrder = stepOrder

	step, err := h.svc.UpsertStep(r.Context(), scenarioID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, step)
}

func (h *Handler) deleteStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := uuid.Parse(chi.URLParam(r, "stepID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.DeleteStep(r.Context(), stepID); err != nil {
		if errors.Is(err, ErrStepNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parsePositiveInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not a valid integer: " + s)
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return 0, errors.New("must be a positive integer")
	}
	return n, nil
}
