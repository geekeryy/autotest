package testdata

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/httpx"
	"autotest/internal/mockdata"
	"autotest/internal/project"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the test data table management API.
type Handler struct {
	service        *Service
	projectHandler *project.Handler
}

// NewHandler constructs a Handler.
func NewHandler(service *Service, projectHandler *project.Handler) *Handler {
	return &Handler{service: service, projectHandler: projectHandler}
}

// Register mounts the routes. The mock-helpers route is exposed at the API
// root because it does not require a project scope; the table CRUD lives under
// `/projects/{projectID}/test-data-tables` and reuses the project role
// middleware.
func (h *Handler) Register(r chi.Router) {
	r.Get("/test-data/mock-helpers", h.listMockHelpers)

	r.Route("/projects/{projectID}/test-data-tables", func(r chi.Router) {
		r.Use(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer))

		r.Get("/", h.listTables)
		r.Get("/{tableID}", h.getTable)
		r.Get("/{tableID}/rows", h.listRows)

		r.Group(func(r chi.Router) {
			r.Use(requireProjectRole(project.ProjectRoleDeveloper))
			r.Post("/", h.createTable)
			r.Put("/{tableID}", h.updateTable)
			r.Delete("/{tableID}", h.deleteTable)
			r.Put("/{tableID}/rows", h.replaceRows)
			r.Post("/{tableID}/rows/generate", h.generateRows)
			r.Post("/{tableID}/rows/{rowID}/regenerate", h.regenerateCells)
		})
	})
}

func (h *Handler) listMockHelpers(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, mockdata.ListHelpers())
}

func (h *Handler) listTables(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	tables, err := h.service.ListTables(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tables)
}

func (h *Handler) getTable(w http.ResponseWriter, r *http.Request) {
	projectID, tableID, ok := parseTableIDs(w, r)
	if !ok {
		return
	}
	table, err := h.service.GetTable(r.Context(), projectID, tableID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, table)
}

func (h *Handler) createTable(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return
	}
	var input TableInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	table, err := h.service.CreateTable(r.Context(), projectID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, table)
}

func (h *Handler) updateTable(w http.ResponseWriter, r *http.Request) {
	projectID, tableID, ok := parseTableIDs(w, r)
	if !ok {
		return
	}
	var input TableInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	table, err := h.service.UpdateTable(r.Context(), projectID, tableID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, table)
}

func (h *Handler) deleteTable(w http.ResponseWriter, r *http.Request) {
	projectID, tableID, ok := parseTableIDs(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteTable(r.Context(), projectID, tableID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRows(w http.ResponseWriter, r *http.Request) {
	projectID, tableID, ok := parseTableIDs(w, r)
	if !ok {
		return
	}
	rows, err := h.service.ListRows(r.Context(), projectID, tableID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rows)
}

func (h *Handler) replaceRows(w http.ResponseWriter, r *http.Request) {
	projectID, tableID, ok := parseTableIDs(w, r)
	if !ok {
		return
	}
	var input RowsReplaceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	rows, err := h.service.ReplaceRows(r.Context(), projectID, tableID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rows)
}

func (h *Handler) generateRows(w http.ResponseWriter, r *http.Request) {
	projectID, tableID, ok := parseTableIDs(w, r)
	if !ok {
		return
	}
	var input GenerateRowsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	output, err := h.service.GenerateRows(r.Context(), projectID, tableID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, output)
}

func (h *Handler) regenerateCells(w http.ResponseWriter, r *http.Request) {
	projectID, tableID, ok := parseTableIDs(w, r)
	if !ok {
		return
	}
	rowID, err := uuid.Parse(chi.URLParam(r, "rowID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	var input RegenerateCellsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	row, err := h.service.RegenerateCells(r.Context(), projectID, tableID, rowID, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, row)
}

func parseProjectID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return projectID, true
}

func parseTableIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, ok := parseProjectID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	tableID, err := uuid.Parse(chi.URLParam(r, "tableID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, tableID, true
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

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrTableNotFound), errors.Is(err, ErrRowNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	case errors.Is(err, ErrTableKeyConflict):
		httpx.Error(w, http.StatusConflict, err)
	case errors.Is(err, ErrInvalidColumn),
		errors.Is(err, ErrInvalidGenerator),
		errors.Is(err, ErrUnknownMockHelper),
		errors.Is(err, ErrAIProviderUnavailable),
		errors.Is(err, ErrAIProviderUnconfigured):
		httpx.Error(w, http.StatusBadRequest, err)
	default:
		httpx.Error(w, http.StatusBadRequest, err)
	}
}
