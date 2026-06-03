package mockrecord

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func registerLegacyMockrecordRoutes(r chi.Router) {
	// 旧写法：在 /mock-servers/{serverID} 下挂子路由，会挡住同前缀的 GET .../routes
	r.Route("/projects/{projectID}/mock-servers/{serverID}", func(r chi.Router) {
		r.Get("/routes/{routeID}/recordings", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})
	})
}

func registerMockserverListRoutes(r chi.Router) {
	r.Route("/projects/{projectID}/mock-servers", func(r chi.Router) {
		r.Get("/{serverID}/routes", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})
}

// 旧版 Route("/.../mock-servers/{serverID}") 会挡住 mockserver 的 listRoutes（回归用例）。
func TestLegacyNestedRouteShadowsMockServerListRoutes(t *testing.T) {
	r := chi.NewRouter()
	registerMockserverListRoutes(r)
	registerLegacyMockrecordRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/projects/00000000-0000-0000-0000-000000000001/mock-servers/00000000-0000-0000-0000-000000000002/routes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy nested route status = %d, want 404 shadowing", rec.Code)
	}
}

func TestRegisterFullPathsDoesNotShadowMockServerListRoutes(t *testing.T) {
	r := chi.NewRouter()
	registerMockserverListRoutes(r)
	// 与 Register 相同的路径布局，不挂 project 中间件
	r.Get("/projects/{projectID}/mock-servers/{serverID}/routes/{routeID}/recordings", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodGet, "/projects/00000000-0000-0000-0000-000000000001/mock-servers/00000000-0000-0000-0000-000000000002/routes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list routes status = %d, want 200", rec.Code)
	}
}
