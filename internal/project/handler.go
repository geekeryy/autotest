package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type contextKey string

const projectRoleKey contextKey = "projectRole"

// ProjectRoleFromContext returns the project-scoped role injected by RequireProjectRole.
// The second return value is false when no role has been injected (e.g. admin bypass with owner role is still injected).
func ProjectRoleFromContext(ctx context.Context) (ProjectRole, bool) {
	role, ok := ctx.Value(projectRoleKey).(ProjectRole)
	return role, ok
}

type Handler struct {
	service *ServiceLayer
}

func NewHandler(service *ServiceLayer) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(r chi.Router) {
	r.Get("/projects", h.listProjects)
	r.Post("/projects", h.createProject)

	r.Route("/projects/{projectID}", func(r chi.Router) {
		r.Use(h.RequireProjectRole(ProjectRoleViewer))

		// viewer+ read routes
		r.Get("/services", h.listServices)
		r.Get("/services/{serviceID}/environments", h.listServiceEnvironments)
		r.Get("/environments", h.listEnvironments)
		r.Get("/members", h.listMembers)

		// developer+ write routes
		r.Group(func(r chi.Router) {
			r.Use(requireContextRole(ProjectRoleDeveloper))
			r.Post("/services", h.createService)
			r.Put("/services/{serviceID}", h.updateService)
			r.Delete("/services/{serviceID}", h.deleteService)
			r.Post("/services/{serviceID}/environments", h.createServiceEnvironment)
			r.Put("/services/{serviceID}/environments/{environmentID}", h.updateServiceEnvironment)
			r.Delete("/services/{serviceID}/environments/{environmentID}", h.deleteServiceEnvironment)
			r.Post("/environments", h.createEnvironment)
			r.Put("/environments/{environmentID}", h.updateEnvironment)
			r.Delete("/environments/{environmentID}", h.deleteEnvironment)
		})

		// owner-only routes
		r.Group(func(r chi.Router) {
			r.Use(requireContextRole(ProjectRoleOwner))
			r.Delete("/", h.deleteProject)
			r.Post("/members", h.addMember)
			r.Put("/members/{userID}", h.updateMember)
			r.Delete("/members/{userID}", h.removeMember)
		})
	})
}

// RequireProjectRole middleware checks the user's project membership.
// Super admins (users:manage) are granted owner-level access without a DB lookup.
func (h *Handler) RequireProjectRole(minRole ProjectRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := auth.PrincipalFromContext(r.Context())
			if principal == nil {
				httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
				return
			}

			// Super admins bypass project membership checks
			if _, isAdmin := principal.Permissions[auth.PermissionUsersManage]; isAdmin {
				ctx := context.WithValue(r.Context(), projectRoleKey, ProjectRoleOwner)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
			if err != nil {
				httpx.Error(w, http.StatusBadRequest, errors.New("invalid projectID"))
				return
			}

			membership, err := h.service.GetMembership(r.Context(), projectID, principal.UserID)
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, err)
				return
			}
			if membership == nil {
				httpx.Error(w, http.StatusForbidden, errors.New("project access denied"))
				return
			}
			if !ProjectRoleAtLeast(membership.Role, minRole) {
				httpx.Error(w, http.StatusForbidden, errors.New("insufficient project role"))
				return
			}

			ctx := context.WithValue(r.Context(), projectRoleKey, membership.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requireContextRole returns a middleware that checks the role already injected into context.
// It must be used inside a group that already ran RequireProjectRole.
func requireContextRole(minRole ProjectRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := ProjectRoleFromContext(r.Context())
			if !ok || !ProjectRoleAtLeast(role, minRole) {
				httpx.Error(w, http.StatusForbidden, errors.New("insufficient project role"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}

	var input CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	project, err := h.service.CreateProject(r.Context(), input, principal.UserID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, project)
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}

	var (
		projects []Project
		err      error
	)
	if _, isAdmin := principal.Permissions[auth.PermissionUsersManage]; isAdmin {
		projects, err = h.service.ListProjects(r.Context())
	} else {
		projects, err = h.service.ListProjectsForUser(r.Context(), principal.UserID)
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if projects == nil {
		projects = []Project{}
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
	if services == nil {
		services = []Service{}
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

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	members, err := h.service.ListMembers(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if members == nil {
		members = []ProjectMember{}
	}
	httpx.JSON(w, http.StatusOK, members)
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input AddMemberInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	member, err := h.service.AddMember(r.Context(), projectID, input)
	if err != nil {
		if errors.Is(err, ErrMemberAlreadyExists) {
			httpx.Error(w, http.StatusConflict, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, member)
}

func (h *Handler) updateMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var input UpdateMemberInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	member, err := h.service.UpdateMember(r.Context(), projectID, userID, input)
	if err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, member)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if err := h.service.RemoveMember(r.Context(), projectID, userID); err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
