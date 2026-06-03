package mockserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"autotest/internal/httpx"
	"autotest/internal/project"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler registers project-scoped mock server management APIs.
type Handler struct {
	service        *Service
	projectHandler *project.Handler
}

// NewHandler creates a Handler.
func NewHandler(service *Service, projectHandler *project.Handler) *Handler {
	return &Handler{service: service, projectHandler: projectHandler}
}

// Register mounts mock server APIs under /projects/{projectID}/mock-servers.
func (h *Handler) Register(r chi.Router) {
	r.Route("/projects/{projectID}/mock-servers", func(r chi.Router) {
		r.Use(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer))

		r.Get("/", h.listServers)
		r.Get("/{serverID}/status", h.getStatus)
		r.Get("/{serverID}/routes", h.listRoutes)
		r.Get("/{serverID}/access-logs", h.listAccessLogs)

		r.Group(func(r chi.Router) {
			r.Use(requireProjectRole(project.ProjectRoleDeveloper))
			r.Post("/", h.createServer)
			r.Put("/{serverID}", h.updateServer)
			r.Delete("/{serverID}", h.deleteServer)
			r.Post("/{serverID}/start", h.startServer)
			r.Post("/{serverID}/stop", h.stopServer)
			r.Post("/{serverID}/routes", h.createRoute)
			r.Put("/{serverID}/routes/{routeID}", h.updateRoute)
			r.Delete("/{serverID}/routes/{routeID}", h.deleteRoute)
			r.Delete("/{serverID}/access-logs", h.clearAccessLogs)
			r.Post("/sync-from-spec", h.syncFromSpec)
			r.Put("/{serverID}/routes/{routeID}/ai-pool", h.updateAIPool)
		})
	})
}

func (h *Handler) listServers(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	servers, err := h.service.ListServers(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, servers)
}

func (h *Handler) createServer(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	var input MockServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	server, err := h.service.CreateServer(r.Context(), projectID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, server)
}

func (h *Handler) updateServer(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	var input MockServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	server, err := h.service.UpdateServer(r.Context(), projectID, serverID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, server)
}

func (h *Handler) deleteServer(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteServer(r.Context(), projectID, serverID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) startServer(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	status, err := h.service.StartServer(r.Context(), projectID, serverID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

func (h *Handler) stopServer(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	status, err := h.service.StopServer(r.Context(), projectID, serverID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	status, err := h.service.GetStatus(r.Context(), projectID, serverID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

func (h *Handler) listRoutes(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	routes, err := h.service.ListRoutes(r.Context(), projectID, serverID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, routes)
}

func (h *Handler) listAccessLogs(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	filter, err := parseAccessLogsFilter(r.URL.Query().Get)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	page, err := h.service.ListAccessLogs(r.Context(), projectID, serverID, filter)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, page)
}

func (h *Handler) clearAccessLogs(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	if err := h.service.ClearAccessLogs(r.Context(), projectID, serverID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createRoute(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return
	}
	var input MockRouteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	route, err := h.service.CreateRoute(r.Context(), projectID, serverID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, route)
}

func (h *Handler) updateRoute(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, routeID, ok := parseRouteIDs(w, r)
	if !ok {
		return
	}
	var input MockRouteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	route, err := h.service.UpdateRoute(r.Context(), projectID, serverID, routeID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, route)
}

func (h *Handler) deleteRoute(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, routeID, ok := parseRouteIDs(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteRoute(r.Context(), projectID, serverID, routeID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requireProjectRole(minRole project.ProjectRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := project.ProjectRoleFromContext(r.Context())
			if !ok || !project.ProjectRoleAtLeast(role, minRole) {
				httpx.Error(w, http.StatusForbidden, errors.New("insufficient project role"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseProjectID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return projectID, true
}

func parseServerIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, serverID, true
}

func parseRouteIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	projectID, serverID, ok := parseServerIDs(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	routeID, err := uuid.Parse(chi.URLParam(r, "routeID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return projectID, serverID, routeID, true
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrMockServerNotFound), errors.Is(err, ErrMockRouteNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	case errors.Is(err, ErrMockServerRunning), errors.Is(err, ErrMockServerNotRunning):
		httpx.Error(w, http.StatusConflict, err)
	default:
		httpx.Error(w, http.StatusBadRequest, err)
	}
}

// syncFromSpec 接收 endpoints 列表并同步为 Schema 模式的 MockRoutes。
// 请求体格式：{"endpoints": [{"method": "GET", "path": "/users", "responseSchema": {...}}, ...]}
func (h *Handler) syncFromSpec(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}

	var input struct {
		Endpoints []EndpointInfo `json:"endpoints"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	created, err := h.service.SyncFromEndpoints(r.Context(), projectID, input.Endpoints)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"created": created,
		"total":   len(input.Endpoints),
	})
}

// updateAIPool 更新路由的 AI 预生成数据池。
// 请求体格式：{"pool": [{...}, {...}, ...]}
func (h *Handler) updateAIPool(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, routeID, ok := parseRouteIDs(w, r)
	if !ok {
		return
	}

	var input struct {
		Pool json.RawMessage `json:"pool"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if len(input.Pool) == 0 {
		httpx.Error(w, http.StatusBadRequest, nil)
		return
	}

	// 获取现有路由
	route, err := h.service.GetRoute(r.Context(), projectID, serverID, routeID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	// 更新 AI 池
	now := time.Now()
	routeUpdate := MockRouteInput{
		Method:           route.Method,
		Path:             route.Path,
		Priority:         route.Priority,
		Enabled:          &route.Enabled,
		RequestMatch:     route.RequestMatch,
		ResponseMode:     route.ResponseMode,
		ResponseStatus:   route.ResponseStatus,
		ResponseHeaders:  route.ResponseHeaders,
		RedirectLocation: route.RedirectLocation,
		ResponseBody:     route.ResponseBody,
		ResponseBodyType: route.ResponseBodyType,
		ResponseSchema:   route.ResponseSchema,
		AIGeneratedPool:  input.Pool,
		RecordTargetURL:  route.RecordTargetURL,
		DelayMillis:      route.DelayMillis,
	}

	updated, err := h.service.UpdateRoute(r.Context(), projectID, serverID, routeID, routeUpdate)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"message":       "AI pool updated",
		"poolSize":      len(input.Pool),
		"aiGeneratedAt": now,
		"route":         updated,
	})
}
