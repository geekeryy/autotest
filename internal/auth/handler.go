package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/authprovider"
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
	r.Get("/auth/login-providers", h.listLoginProviders)
	r.Get("/auth/oauth/{slug}/login", h.oauthLogin)
	r.Get("/auth/oauth/{slug}/callback", h.oauthCallback)
	r.Get("/auth/github/status", h.githubStatus)
	r.Get("/auth/github/login", h.githubLogin)
	r.Get("/auth/github/callback", h.githubCallback)
}

func (h *Handler) RegisterAuthenticated(r chi.Router) {
	r.Get("/auth/me", h.me)
	r.Post("/auth/logout", h.logout)
	r.Post("/auth/change-password", h.changePassword)
}

func (h *Handler) RegisterProtected(r chi.Router) {
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
	response, err := h.service.Login(r.Context(), input, RequestMetaFromHTTP(r))
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			httpx.Error(w, http.StatusUnauthorized, errors.New("用户名或密码错误"))
			return
		}
		httpx.Error(w, http.StatusUnauthorized, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) listLoginProviders(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListLoginProviders(r.Context(), RequestAPIOrigin(r))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) oauthLogin(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	frontendURL := r.URL.Query().Get("frontendUrl")
	url, err := h.service.OAuthLoginURL(r.Context(), slug, frontendURL)
	if err != nil {
		if errors.Is(err, ErrInvalidFrontendURL) {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		status := http.StatusServiceUnavailable
		if !errors.Is(err, ErrOAuthDisabled) && !errors.Is(err, authprovider.ErrProviderNotFound) && !errors.Is(err, authprovider.ErrProviderDisabled) {
			status = http.StatusInternalServerError
		}
		httpx.Error(w, status, err)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	redirectURL, frontendURL, err := h.service.HandleOAuthCallback(r.Context(), slug, r.URL.Query().Get("code"), r.URL.Query().Get("state"), RequestMetaFromHTTP(r))
	if err != nil {
		query := "error=" + OAuthCallbackErrorQuery(err)
		if frontendURL != "" {
			http.Redirect(w, r, h.service.FrontendLoginURL(frontendURL, query), http.StatusFound)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *Handler) githubStatus(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]bool{
		"configured": h.service.GitHubOAuthEnabled(),
	})
}

func (h *Handler) githubLogin(w http.ResponseWriter, r *http.Request) {
	frontendURL := r.URL.Query().Get("frontendUrl")
	url, err := h.service.GitHubLoginURL(r.Context(), frontendURL)
	if err != nil {
		if errors.Is(err, ErrInvalidFrontendURL) {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		status := http.StatusServiceUnavailable
		if !errors.Is(err, ErrOAuthDisabled) {
			status = http.StatusInternalServerError
		}
		httpx.Error(w, status, err)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) githubCallback(w http.ResponseWriter, r *http.Request) {
	redirectURL, frontendURL, err := h.service.HandleGitHubCallback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"), RequestMetaFromHTTP(r))
	if err != nil {
		query := "error=" + OAuthCallbackErrorQuery(err)
		if frontendURL != "" {
			http.Redirect(w, r, h.service.FrontendLoginURL(frontendURL, query), http.StatusFound)
			return
		}
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
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

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	principal := PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	var input ChangePasswordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.service.ChangePassword(r.Context(), principal.UserID, input); err != nil {
		switch {
		case errors.Is(err, ErrUnauthorized):
			httpx.Error(w, http.StatusUnauthorized, errors.New("当前密码错误"))
		case errors.Is(err, ErrPasswordUnchanged):
			httpx.Error(w, http.StatusBadRequest, errors.New("新密码不能与当前密码相同"))
		default:
			httpx.Error(w, http.StatusBadRequest, err)
		}
		return
	}
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
