package spec

import (
	"context"
	"io"
	"net/http"

	"autotest/internal/auth"
	"autotest/internal/httpx"
	"autotest/internal/logx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ImportNotifier 在 API Key 成功导入 OpenAPI/Swagger 后写入站内通知；由 internal/notification 实现。
type ImportNotifier interface {
	CreateSpecImportNotification(ctx context.Context, userID, projectID, serviceID uuid.UUID, summary *ImportSummary) error
}

type Handler struct {
	service  *Service
	notifier ImportNotifier
}

func NewHandler(service *Service, notifier ImportNotifier) *Handler {
	return &Handler{service: service, notifier: notifier}
}

// Register 挂载只读接口（list specs / list endpoints）。
// 导入接口由 RegisterImport 单独挂载，以便外层在不同的认证守卫（如允许 API Key）下注册。
func (h *Handler) Register(r chi.Router) {
	r.Get("/projects/{projectID}/services/{serviceID}/specs", h.listSpecs)
	r.Get("/projects/{projectID}/services/{serviceID}/endpoints", h.listEndpoints)
}

// RegisterImport 单独挂载 OpenAPI/Swagger 导入路由。
// 调用方负责在路由组级别添加 specs:import 作用域或权限校验。
func (h *Handler) RegisterImport(r chi.Router) {
	r.Post("/projects/{projectID}/services/{serviceID}/specs/import", h.importSpec)
}

// RegisterAPIKeyRead 挂载 API Key 可读路由（spec 列表与端点列表），需 cases:read 或 specs:import 作用域组守卫。
func (h *Handler) RegisterAPIKeyRead(r chi.Router) {
	h.Register(r)
}

func (h *Handler) listSpecs(w http.ResponseWriter, r *http.Request) {
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

	specs, err := h.service.ListSpecs(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, specs)
}

func (h *Handler) importSpec(w http.ResponseWriter, r *http.Request) {
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

	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 20<<20))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	summary, err := h.service.Import(r.Context(), projectID, serviceID, data, ParseSyncMode(r.URL.Query().Get("sync")))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	h.notifyImportIfAPIKey(r, projectID, serviceID, summary)
	httpx.JSON(w, http.StatusCreated, summary)
}

func (h *Handler) notifyImportIfAPIKey(r *http.Request, projectID, serviceID uuid.UUID, summary *ImportSummary) {
	if h.notifier == nil || summary == nil {
		return
	}
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil || principal.Source != auth.SourceAPIKey {
		return
	}
	if err := h.notifier.CreateSpecImportNotification(r.Context(), principal.UserID, projectID, serviceID, summary); err != nil {
		logx.Warn("create spec import notification", "err", err, "userId", principal.UserID, "projectId", projectID, "serviceId", serviceID)
	}
}

func (h *Handler) listEndpoints(w http.ResponseWriter, r *http.Request) {
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

	endpoints, err := h.service.ListEndpoints(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, endpoints)
}
