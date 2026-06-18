package project

import (
	"errors"
	"net/http"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) getServiceMcpIntegration(w http.ResponseWriter, r *http.Request) {
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

	svc, err := h.service.GetService(r.Context(), projectID, serviceID)
	if err != nil {
		if errors.Is(err, ErrServiceNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !svc.McpEnabled {
		httpx.Error(w, http.StatusBadRequest, errors.New("该服务未启用 MCP 自动化"))
		return
	}

	envs, err := h.service.ListServiceEnvironments(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if envs == nil {
		envs = []Environment{}
	}

	var apiKeyToken string
	var apiKeyMask string
	var apiKeyID string

	ensureKey := r.URL.Query().Get("ensureApiKey") != "0"
	regenerate := r.URL.Query().Get("regenerate") == "1"
	if ensureKey || regenerate {
		principal := auth.PrincipalFromContext(r.Context())
		if principal == nil {
			httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
			return
		}
		role, ok := ProjectRoleFromContext(r.Context())
		if !ok || !ProjectRoleAtLeast(role, ProjectRoleDeveloper) {
			httpx.Error(w, http.StatusForbidden, errors.New("需要项目 developer 权限以生成 MCP API Key"))
			return
		}
		if h.apiKeys == nil {
			httpx.Error(w, http.StatusInternalServerError, errors.New("api key service is not configured"))
			return
		}
		keyRes, updated, err := h.service.EnsureServiceMcpAPIKey(
			r.Context(), principal.UserID, projectID, serviceID, svc, h.apiKeys, regenerate,
		)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		svc = updated
		apiKeyToken = keyRes.Token
		apiKeyMask = keyRes.Mask
		if keyRes.KeyID != uuid.Nil {
			apiKeyID = keyRes.KeyID.String()
		}
	}

	guide := BuildMcpIntegrationGuide(h.mcpHTTP, h.listenAddr, r, *svc, envs, apiKeyToken)
	guide.ApiKeyToken = apiKeyToken
	guide.ApiKeyMask = apiKeyMask
	guide.ApiKeyID = apiKeyID
	if apiKeyToken != "" {
		guide.ServerEnvHint = "已自动生成 MCP 专用 API Key 并填入下方配置；明文仅本次可见，请尽快完成 Cursor 安装。"
	}

	if envID := r.URL.Query().Get("environmentId"); envID != "" {
		if parsed, err := uuid.Parse(envID); err == nil {
			guide = PatchMcpIntegrationEnvironment(guide, parsed)
		}
	}

	httpx.JSON(w, http.StatusOK, guide)
}
