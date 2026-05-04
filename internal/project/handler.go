package project

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *ServiceLayer
}

func NewHandler(service *ServiceLayer) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/projects", h.listProjects)
	r.Post("/projects", h.createProject)
	r.Delete("/projects/{projectID}", h.deleteProject)
	r.Get("/projects/{projectID}/services", h.listServices)
	r.Post("/projects/{projectID}/services", h.createService)
	r.Put("/projects/{projectID}/services/{serviceID}", h.updateService)
	r.Delete("/projects/{projectID}/services/{serviceID}", h.deleteService)
	r.Get("/projects/{projectID}/services/{serviceID}/environments", h.listServiceEnvironments)
	r.Post("/projects/{projectID}/services/{serviceID}/environments", h.createServiceEnvironment)
	r.Put("/projects/{projectID}/services/{serviceID}/environments/{environmentID}", h.updateServiceEnvironment)
	r.Delete("/projects/{projectID}/services/{serviceID}/environments/{environmentID}", h.deleteServiceEnvironment)
	r.Get("/projects/{projectID}/environments", h.listEnvironments)
	r.Post("/projects/{projectID}/environments", h.createEnvironment)
	r.Put("/projects/{projectID}/environments/{environmentID}", h.updateEnvironment)
	r.Delete("/projects/{projectID}/environments/{environmentID}", h.deleteEnvironment)
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	var input CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	project, err := h.service.CreateProject(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, project)
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.service.ListProjects(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, projects)
}

func (h *Handler) deleteProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if err := h.service.DeleteProject(r.Context(), projectID); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createService(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input CreateServiceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	service, err := h.service.CreateService(r.Context(), projectID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, service)
}

func (h *Handler) listServices(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	services, err := h.service.ListServices(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, services)
}

func (h *Handler) updateService(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	serviceID, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input UpdateServiceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	service, err := h.service.UpdateService(r.Context(), projectID, serviceID, input)
	if err != nil {
		if errors.Is(err, ErrServiceNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, service)
}

func (h *Handler) deleteService(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	serviceID, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if err := h.service.DeleteService(r.Context(), projectID, serviceID); err != nil {
		if errors.Is(err, ErrServiceNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createServiceEnvironment(w http.ResponseWriter, r *http.Request) {
	projectID, serviceID, ok := serviceRouteIDs(w, r)
	if !ok {
		return
	}

	var input CreateEnvironmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	env, err := h.service.CreateServiceEnvironment(r.Context(), projectID, serviceID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, env)
}

func (h *Handler) listServiceEnvironments(w http.ResponseWriter, r *http.Request) {
	projectID, serviceID, ok := serviceRouteIDs(w, r)
	if !ok {
		return
	}

	environments, err := h.service.ListServiceEnvironments(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, environments)
}

func (h *Handler) updateServiceEnvironment(w http.ResponseWriter, r *http.Request) {
	projectID, serviceID, ok := serviceRouteIDs(w, r)
	if !ok {
		return
	}
	environmentID, err := uuid.Parse(chi.URLParam(r, "environmentID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input UpdateEnvironmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	env, err := h.service.UpdateServiceEnvironment(r.Context(), projectID, serviceID, environmentID, input)
	if err != nil {
		if errors.Is(err, ErrEnvironmentNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, env)
}

func (h *Handler) deleteServiceEnvironment(w http.ResponseWriter, r *http.Request) {
	projectID, serviceID, ok := serviceRouteIDs(w, r)
	if !ok {
		return
	}
	environmentID, err := uuid.Parse(chi.URLParam(r, "environmentID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if err := h.service.DeleteServiceEnvironment(r.Context(), projectID, serviceID, environmentID); err != nil {
		if errors.Is(err, ErrEnvironmentNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func serviceRouteIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, false
	}
	serviceID, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, serviceID, true
}

func (h *Handler) createEnvironment(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input CreateEnvironmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	env, err := h.service.CreateEnvironment(r.Context(), projectID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, env)
}

func (h *Handler) listEnvironments(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	environments, err := h.service.ListEnvironments(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, environments)
}

func (h *Handler) updateEnvironment(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	environmentID, err := uuid.Parse(chi.URLParam(r, "environmentID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input UpdateEnvironmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	env, err := h.service.UpdateEnvironment(r.Context(), projectID, environmentID, input)
	if err != nil {
		if errors.Is(err, ErrEnvironmentNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, env)
}

func (h *Handler) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	environmentID, err := uuid.Parse(chi.URLParam(r, "environmentID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if err := h.service.DeleteEnvironment(r.Context(), projectID, environmentID); err != nil {
		if errors.Is(err, ErrEnvironmentNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
