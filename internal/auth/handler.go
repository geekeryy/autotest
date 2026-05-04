package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterPublic(r chi.Router) {
	r.Post("/auth/login", h.login)
}

func (h *Handler) RegisterProtected(r chi.Router) {
	r.Get("/auth/me", h.me)
	r.Post("/auth/logout", h.logout)

	r.Group(func(r chi.Router) {
		r.Use(h.service.RequirePermission(PermissionUsersManage))
		r.Get("/users", h.listUsers)
		r.Post("/users", h.createUser)
		r.Put("/users/{userID}", h.updateUser)
		r.Delete("/users/{userID}", h.deleteUser)
	})

	r.Group(func(r chi.Router) {
		r.Use(h.service.RequirePermission(PermissionRolesManage))
		r.Get("/roles", h.listRoles)
		r.Post("/roles", h.createRole)
		r.Put("/roles/{roleID}", h.updateRole)
		r.Delete("/roles/{roleID}", h.deleteRole)
		r.Put("/roles/{roleID}/permissions", h.setRolePermissions)
	})

	r.Group(func(r chi.Router) {
		r.Use(h.service.RequirePermission(PermissionPermissionsManage))
		r.Get("/permissions", h.listPermissions)
		r.Post("/permissions", h.createPermission)
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	response, err := h.service.Login(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	user, err := h.service.CurrentUser(r.Context(), principal.UserID)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, users)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var input CreateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	user, err := h.service.CreateUser(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, user)
}

func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	var input UpdateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	user, err := h.service.UpdateUser(r.Context(), id, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.service.DeleteUser(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, roles)
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	var input CreateRoleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	role, err := h.service.CreateRole(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, role)
}

func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "roleID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	var input UpdateRoleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	role, err := h.service.UpdateRole(r.Context(), id, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, role)
}

func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "roleID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.service.DeleteRole(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setRolePermissions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "roleID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	var input SetRolePermissionsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	role, err := h.service.SetRolePermissions(r.Context(), id, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, role)
}

func (h *Handler) listPermissions(w http.ResponseWriter, r *http.Request) {
	permissions, err := h.service.ListPermissions(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, permissions)
}

func (h *Handler) createPermission(w http.ResponseWriter, r *http.Request) {
	var input CreatePermissionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	permission, err := h.service.CreatePermission(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, permission)
}
