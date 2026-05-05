package paramsource

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

func (h *Handler) Register(r chi.Router) {
	r.Get("/data-sources", h.listDataSources)
	r.Post("/data-sources", h.createDataSource)
	r.Put("/data-sources/{id}", h.updateDataSource)
	r.Delete("/data-sources/{id}", h.deleteDataSource)
	r.Post("/data-sources/{id}/test", h.testDataSource)
	r.Get("/data-sources/{id}/schema", h.getDataSourceSchema)
	r.Get("/sql-parameter-sources", h.listSQLParameterSources)
	r.Post("/sql-parameter-sources/preview", h.previewSQLParameterSourceDraft)
	r.Post("/sql-parameter-sources", h.createSQLParameterSource)
	r.Put("/sql-parameter-sources/{id}", h.updateSQLParameterSource)
	r.Delete("/sql-parameter-sources/{id}", h.deleteSQLParameterSource)
	r.Post("/sql-parameter-sources/{id}/preview", h.previewSQLParameterSource)
}

func (h *Handler) listDataSources(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseQueryUUID(w, r, "projectId")
	if !ok {
		return
	}
	sources, err := h.service.ListDataSources(r.Context(), projectID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sources)
}

func (h *Handler) createDataSource(w http.ResponseWriter, r *http.Request) {
	var input DataSourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	source, err := h.service.CreateDataSource(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, source)
}

func (h *Handler) updateDataSource(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}
	var input DataSourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	source, err := h.service.UpdateDataSource(r.Context(), id, input)
	if err != nil {
		writeNotFoundOrBadRequest(w, err, ErrDataSourceNotFound)
		return
	}
	httpx.JSON(w, http.StatusOK, source)
}

func (h *Handler) deleteDataSource(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteDataSource(r.Context(), id); err != nil {
		writeNotFoundOrBadRequest(w, err, ErrDataSourceNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getDataSourceSchema(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}
	tables, err := h.service.GetDataSourceSchema(r.Context(), id)
	if err != nil {
		writeNotFoundOrBadRequest(w, err, ErrDataSourceNotFound)
		return
	}
	httpx.JSON(w, http.StatusOK, tables)
}

func (h *Handler) testDataSource(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.service.TestDataSource(r.Context(), id); err != nil {
		writeNotFoundOrBadRequest(w, err, ErrDataSourceNotFound)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listSQLParameterSources(w http.ResponseWriter, r *http.Request) {
	projectID, ok := parseQueryUUID(w, r, "projectId")
	if !ok {
		return
	}
	serviceID, ok := parseQueryUUID(w, r, "serviceId")
	if !ok {
		return
	}
	sources, err := h.service.ListSQLParameterSources(r.Context(), projectID, serviceID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sources)
}

func (h *Handler) createSQLParameterSource(w http.ResponseWriter, r *http.Request) {
	var input SQLParameterSourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	source, err := h.service.CreateSQLParameterSource(r.Context(), input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, source)
}

func (h *Handler) updateSQLParameterSource(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}
	var input SQLParameterSourceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	source, err := h.service.UpdateSQLParameterSource(r.Context(), id, input)
	if err != nil {
		writeNotFoundOrBadRequest(w, err, ErrSQLParameterSourceNotFound)
		return
	}
	httpx.JSON(w, http.StatusOK, source)
}

func (h *Handler) deleteSQLParameterSource(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteSQLParameterSource(r.Context(), id); err != nil {
		writeNotFoundOrBadRequest(w, err, ErrSQLParameterSourceNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) previewSQLParameterSourceDraft(w http.ResponseWriter, r *http.Request) {
	var input PreviewDraftInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	output, err := h.service.PreviewSQLParameterSourceDraft(r.Context(), input)
	if err != nil {
		writeNotFoundOrBadRequest(w, err, ErrDataSourceNotFound)
		return
	}
	httpx.JSON(w, http.StatusOK, output)
}

func (h *Handler) previewSQLParameterSource(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathUUID(w, r, "id")
	if !ok {
		return
	}
	var input PreviewInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	output, err := h.service.PreviewSQLParameterSource(r.Context(), id, input)
	if err != nil {
		writeNotFoundOrBadRequest(w, err, ErrSQLParameterSourceNotFound)
		return
	}
	httpx.JSON(w, http.StatusOK, output)
}

func parseQueryUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		httpx.Error(w, http.StatusBadRequest, errors.New(name+" is required"))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return id, true
}

func parsePathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return uuid.Nil, false
	}
	return id, true
}

func writeNotFoundOrBadRequest(w http.ResponseWriter, err error, notFound error) {
	if errors.Is(err, notFound) {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.Error(w, http.StatusBadRequest, err)
}
