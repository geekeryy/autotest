package testprofile

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes the test profile HTTP API.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the test profile routes under a chi router that already
// has projectID in its URL (e.g. /projects/{projectID}/test-profile).
func (h *Handler) Register(r chi.Router) {
	r.Route("/projects/{projectID}/test-profile", func(r chi.Router) {
		r.Get("/", h.get)
		r.Post("/build", h.build)
		r.Get("/prompt-context", h.promptContext)
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		http.Error(w, "invalid projectID", http.StatusBadRequest)
		return
	}

	profile, err := h.svc.Get(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if profile == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"profile":null,"message":"尚未构建项目测试画像，请调用 POST /build 生成"}`))
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *Handler) build(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		http.Error(w, "invalid projectID", http.StatusBadRequest)
		return
	}

	profile, err := h.svc.Build(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"profile": profile,
		"message": "项目测试画像已更新",
	})
}

func (h *Handler) promptContext(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		http.Error(w, "invalid projectID", http.StatusBadRequest)
		return
	}

	ctx, err := h.svc.GetPromptContext(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"context": ctx,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
