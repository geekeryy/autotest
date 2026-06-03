package mockrecord

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/httpx"
	"autotest/internal/mockserver"
	"autotest/internal/project"
	"autotest/internal/urlx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RouteModeService 切换 Mock 路由的 record/replay 模式。
type RouteModeService interface {
	EnableRecordMode(ctx context.Context, projectID, serverID, routeID uuid.UUID, targetURL string) (RouteModeResult, error)
	EnableReplayMode(ctx context.Context, projectID, serverID, routeID uuid.UUID) (RouteModeResult, error)
}

// RouteModeResult 描述路由模式切换结果。
type RouteModeResult struct {
	RouteID         uuid.UUID `json:"routeId"`
	ResponseMode    string    `json:"responseMode"`
	RecordTargetURL string    `json:"recordTargetUrl,omitempty"`
}

// Handler registers recording/replay management APIs.
type Handler struct {
	player *Player
	routes RouteModeService
	proj   *project.Handler
}

// NewHandler creates a Handler.
func NewHandler(player *Player, routes RouteModeService, proj *project.Handler) *Handler {
	return &Handler{player: player, routes: routes, proj: proj}
}

// Register mounts recording/replay APIs on mock routes.
// 使用完整路径注册，避免 Route("/.../mock-servers/{serverID}") 抢占前缀导致
// mockserver 的 GET .../mock-servers/{serverID}/routes 返回 404。
func (h *Handler) Register(r chi.Router) {
	const routePrefix = "/projects/{projectID}/mock-servers/{serverID}/routes/{routeID}"
	const serverPrefix = "/projects/{projectID}/mock-servers/{serverID}"

	r.With(h.proj.RequireProjectRole(project.ProjectRoleViewer)).
		Get(routePrefix+"/recordings", h.listRecordings)

	r.Group(func(r chi.Router) {
		r.Use(h.proj.RequireProjectRole(project.ProjectRoleDeveloper))
		r.Post(routePrefix+"/record", h.startRecording)
		r.Post(routePrefix+"/replay", h.enableReplay)
		r.Delete(routePrefix+"/recordings", h.clearRecordings)
		r.Delete(serverPrefix+"/recordings/{recordingID}", h.deleteRecording)
	})
}

func (h *Handler) startRecording(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, routeID, ok := parseRouteIDs(w, r)
	if !ok {
		return
	}

	var input struct {
		TargetURL string `json:"targetUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := urlx.ValidateHTTPServiceURL(input.TargetURL); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	result, err := h.routes.EnableRecordMode(r.Context(), projectID, serverID, routeID, input.TargetURL)
	if err != nil {
		writeRouteModeError(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"message":      "recording mode enabled",
		"targetUrl":    result.RecordTargetURL,
		"routeId":      result.RouteID,
		"responseMode": result.ResponseMode,
	})
}

func (h *Handler) enableReplay(w http.ResponseWriter, r *http.Request) {
	projectID, serverID, routeID, ok := parseRouteIDs(w, r)
	if !ok {
		return
	}

	result, err := h.routes.EnableReplayMode(r.Context(), projectID, serverID, routeID)
	if err != nil {
		writeRouteModeError(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"message":      "replay mode enabled",
		"routeId":      result.RouteID,
		"responseMode": result.ResponseMode,
	})
}

func (h *Handler) listRecordings(w http.ResponseWriter, r *http.Request) {
	_, serverID, routeID, ok := parseRouteIDs(w, r)
	if !ok {
		return
	}

	recordings, err := h.player.ListRecordings(r.Context(), serverID, routeID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, recordings)
}

func (h *Handler) deleteRecording(w http.ResponseWriter, r *http.Request) {
	recordingID, err := uuid.Parse(chi.URLParam(r, "recordingID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if err := h.player.DeleteRecording(r.Context(), recordingID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) clearRecordings(w http.ResponseWriter, r *http.Request) {
	_, serverID, routeID, ok := parseRouteIDs(w, r)
	if !ok {
		return
	}

	if err := h.player.ClearRecordings(r.Context(), serverID, routeID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeRouteModeError(w http.ResponseWriter, err error) {
	if errors.Is(err, urlx.ErrInvalidTargetURL) {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if errors.Is(err, mockserver.ErrMockRouteNotFound) || errors.Is(err, mockserver.ErrMockServerNotFound) {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.Error(w, http.StatusInternalServerError, err)
}

func parseRouteIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	routeID, err := uuid.Parse(chi.URLParam(r, "routeID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return projectID, serverID, routeID, true
}
