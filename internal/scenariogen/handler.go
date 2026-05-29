package scenariogen

import (
	"net/http"

	"autotest/internal/httpx"
	"autotest/internal/project"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler exposes scenariogen helper routes under a project.
type Handler struct {
	deps           Deps
	envReader      EnvironmentReader
	projectHandler *project.Handler
}

// NewHandler constructs a Handler.
func NewHandler(deps Deps, envReader EnvironmentReader, projectHandler *project.Handler) *Handler {
	return &Handler{deps: deps, envReader: envReader, projectHandler: projectHandler}
}

// Register mounts login hint discovery for the admin UI.
func (h *Handler) Register(r chi.Router) {
	// Use a flat path so chi does not mount a /projects/{projectID}/services/{serviceID}
	// prefix route that shadows project handler's /services/{serviceID}/environments routes.
	r.With(h.projectHandler.RequireProjectRole(project.ProjectRoleViewer)).
		Get("/projects/{projectID}/services/{serviceID}/scenario-login-hints", h.loginHints)
}

func (h *Handler) loginHints(w http.ResponseWriter, r *http.Request) {
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
	var environmentID uuid.UUID
	if raw := r.URL.Query().Get("environmentId"); raw != "" {
		environmentID, err = uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
	}
	out, err := CollectLoginHints(r.Context(), h.deps, h.envReader, projectID, serviceID, environmentID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
