package aiskill

import (
	"encoding/json"
	"errors"
	"net/http"

	"autotest/internal/auth"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes AI skill CRUD endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register mounts AI skill routes.
func (h *Handler) Register(r chi.Router) {
	r.Route("/ai/skills", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Post("/match", h.match)
		r.Get("/candidates", h.listCandidates)
		r.Post("/candidates/{candidateID}/approve", h.approveCandidate)
		r.Post("/candidates/{candidateID}/reject", h.rejectCandidate)
		r.Get("/{skillID}", h.get)
		r.Put("/{skillID}", h.update)
		r.Delete("/{skillID}", h.delete)
	})
	r.Route("/projects/{projectID}/ai/skills", func(r chi.Router) {
		r.Get("/", h.listByProject)
		r.Post("/", h.createInProject)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	opts := ListSkillsOptions{
		Enabled: parseBoolPtr(r.URL.Query().Get("enabled")),
		Limit:   50,
	}
	items, err := h.service.ListSkills(r.Context(), opts)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) listByProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 projectId"))
		return
	}
	opts := ListSkillsOptions{
		ProjectID: &projectID,
		Enabled:   parseBoolPtr(r.URL.Query().Get("enabled")),
		Limit:     50,
	}
	items, err := h.service.ListSkills(r.Context(), opts)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	var input CreateSkillInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	skill, err := h.service.CreateSkill(r.Context(), nil, principal.UserID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, skill)
}

func (h *Handler) createInProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 projectId"))
		return
	}
	principal := auth.PrincipalFromContext(r.Context())
	if principal == nil {
		httpx.Error(w, http.StatusUnauthorized, errors.New("missing principal"))
		return
	}
	var input CreateSkillInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	skill, err := h.service.CreateSkill(r.Context(), &projectID, principal.UserID, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, skill)
}

func (h *Handler) match(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query       string `json:"query"`
		PageContext string `json:"pageContext"`
		ProjectID   string `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	var projectID *uuid.UUID
	if body.ProjectID != "" {
		id, err := uuid.Parse(body.ProjectID)
		if err == nil {
			projectID = &id
		}
	}
	matches, err := h.service.MatchSkills(r.Context(), projectID, body.Query, []byte(body.PageContext))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, matches)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "skillID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 skillId"))
		return
	}
	skill, err := h.service.GetSkill(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, skill)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "skillID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 skillId"))
		return
	}
	var input UpdateSkillInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	skill, err := h.service.UpdateSkill(r.Context(), id, input)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	httpx.JSON(w, http.StatusOK, skill)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "skillID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 skillId"))
		return
	}
	if err := h.service.DeleteSkill(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listCandidates(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	candidates, err := h.service.ListCandidates(r.Context(), status, 50)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, candidates)
}

func (h *Handler) approveCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "candidateID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 candidateId"))
		return
	}
	if err := h.service.ApproveCandidate(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *Handler) rejectCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "candidateID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, errors.New("无效的 candidateId"))
		return
	}
	if err := h.service.RejectCandidate(r.Context(), id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func parseBoolPtr(s string) *bool {
	if s == "" {
		return nil
	}
	b := s == "true" || s == "1"
	return &b
}
