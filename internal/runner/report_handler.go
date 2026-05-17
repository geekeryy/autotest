package runner

import (
	"errors"
	"net/http"
	"strconv"

	"autotest/internal/auth"
	"autotest/internal/httpx"
	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/scenario"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) listProjectRuns(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if !h.ensureProjectAccess(w, r, projectID, project.ProjectRoleViewer) {
		return
	}
	filter, err := parseRunListFilter(queryGetter{r}, projectID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	entries, total, limit, offset, err := h.service.ListProjectRuns(r.Context(), filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	pageNum := 1
	if limit > 0 {
		pageNum = offset/limit + 1
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"items":    entries,
		"total":    total,
		"page":     pageNum,
		"pageSize": limit,
	})
}

func (h *Handler) listScenarioRuns(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := uuid.Parse(chi.URLParam(r, "scenarioID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	projectID, err := uuid.Parse(r.URL.Query().Get("projectId"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("projectId is required"))
		return
	}
	if !h.ensureProjectAccess(w, r, projectID, project.ProjectRoleViewer) {
		return
	}
	filter, err := parseRunListFilter(queryGetter{r}, projectID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	page, err := h.service.ListScenarioRuns(r.Context(), h.scenarioRepo, scenarioID, filter)
	if err != nil {
		if errors.Is(err, scenario.ErrScenarioNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	pageNum := 1
	if page.Limit > 0 {
		pageNum = page.Offset/page.Limit + 1
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"items":    page.Items,
		"total":    page.Total,
		"page":     pageNum,
		"pageSize": page.Limit,
	})
}

func (h *Handler) scenarioRunStats(w http.ResponseWriter, r *http.Request) {
	scenarioID, err := uuid.Parse(chi.URLParam(r, "scenarioID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	projectID, err := uuid.Parse(r.URL.Query().Get("projectId"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("projectId is required"))
		return
	}
	if !h.ensureProjectAccess(w, r, projectID, project.ProjectRoleViewer) {
		return
	}
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			days = n
		}
	}
	stats, err := h.service.ScenarioRunStats(r.Context(), h.scenarioRepo, scenarioID, projectID, days)
	if err != nil {
		if errors.Is(err, scenario.ErrScenarioNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, stats)
}

func (h *Handler) getScenarioRunReport(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	rep, err := h.service.GetScenarioRunReport(r.Context(), runID)
	if err != nil {
		if errors.Is(err, report.ErrRunNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, report.ErrNotScenarioRun) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !h.ensureProjectAccess(w, r, rep.Run.ProjectID, project.ProjectRoleViewer) {
		return
	}
	httpx.JSON(w, http.StatusOK, rep)
}

func (h *Handler) exportScenarioRun(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	rep, err := h.service.GetScenarioRunReport(r.Context(), runID)
	if err != nil {
		if errors.Is(err, report.ErrRunNotFound) || errors.Is(err, report.ErrNotScenarioRun) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !h.ensureProjectAccess(w, r, rep.Run.ProjectID, project.ProjectRoleViewer) {
		return
	}
	data, contentType, filename, err := h.service.ExportScenarioRun(r.Context(), runID, format)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

type queryGetter struct{ r *http.Request }

func (q queryGetter) URLQueryGet(key string) string {
	return q.r.URL.Query().Get(key)
}

func (h *Handler) ensureProjectAccess(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, minRole project.ProjectRole) bool {
	if h.projectService == nil {
		httpx.Error(w, http.StatusInternalServerError, errors.New("project service unavailable"))
		return false
	}
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return false
	}
	if _, isAdmin := principal.Permissions[auth.PermissionUsersManage]; isAdmin {
		return true
	}
	membership, err := h.projectService.GetMembership(r.Context(), projectID, principal.UserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return false
	}
	if membership == nil {
		httpx.Error(w, http.StatusForbidden, errors.New("project access denied"))
		return false
	}
	if !project.ProjectRoleAtLeast(membership.Role, minRole) {
		httpx.Error(w, http.StatusForbidden, errors.New("insufficient project role"))
		return false
	}
	return true
}
