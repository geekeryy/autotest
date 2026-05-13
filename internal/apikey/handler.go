package apikey

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler 暴露 /api-keys 资源 REST 接口；由 cmd/api/main.go 注册在 RejectAPIKey 守卫之后，
// 因此 apikey 来源的请求无法访问这些管理接口。
type Handler struct {
	svc       *Service
	permGuard func(code string) func(http.Handler) http.Handler
}

// NewHandler 构造 Handler；permGuard 通常传入 auth.Service.RequirePermission。
func NewHandler(svc *Service, permGuard func(code string) func(http.Handler) http.Handler) *Handler {
	return &Handler{svc: svc, permGuard: permGuard}
}

// Register 在指定路由组下挂载 API Key 管理接口；所有接口要求 apikeys:manage 权限。
func (h *Handler) Register(r chi.Router) {
	r.Group(func(r chi.Router) {
		if h.permGuard != nil {
			r.Use(h.permGuard(auth.PermissionAPIKeysManage))
		}
		r.Get("/api-keys", h.list)
		r.Post("/api-keys", h.create)
		r.Patch("/api-keys/{id}", h.update)
		r.Post("/api-keys/{id}/rotate", h.rotate)
		r.Delete("/api-keys/{id}", h.delete)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	keys, err := h.svc.List(r.Context(), principal.UserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, keys)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.svc.Create(r.Context(), principal.UserID, input)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
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
	key, err := h.svc.Update(r.Context(), principal.UserID, id, input)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, key)
}

func (h *Handler) rotate(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	key, token, err := h.svc.Rotate(r.Context(), principal.UserID, id)
	if err != nil {
		writeServiceErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, &CreateResult{APIKey: *key, Token: token})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.svc.Delete(r.Context(), principal.UserID, id); err != nil {
		writeServiceErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeServiceErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	case errors.Is(err, ErrScopeForbidden):
		httpx.Error(w, http.StatusBadRequest, err)
	default:
		httpx.Error(w, http.StatusBadRequest, err)
	}
}
