package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"autotest/internal/aiprovider"
	"autotest/internal/auth"
	"autotest/internal/projectprompt"
	testcase "autotest/internal/case"
	"autotest/internal/generator"
	"autotest/internal/httpx"
	"autotest/internal/mockserver"
	"autotest/internal/paramsource"
	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/runner"
	"autotest/internal/scenario"
	"autotest/internal/scriptlibrary"
	"autotest/internal/spec"
	"autotest/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	ctx := context.Background()
	db, err := store.Open(ctx, store.ConfigFromEnv())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	repo := store.NewRepository(db)

	authRepo := auth.NewRepository(repo)
	authSvc := auth.NewService(authRepo)
	if err := authSvc.EnsureDefaultAdmin(ctx); err != nil {
		log.Fatalf("ensure default admin: %v", err)
	}

	projectRepo := project.NewRepository(repo)
	projectSvc := project.NewServiceLayer(projectRepo)

	caseRepo := testcase.NewRepository(repo)
	specRepo := spec.NewRepository(repo)

	caseSvc := testcase.NewService(caseRepo, specRepo)
	specSvc := spec.NewService(specRepo, caseRepo, spec.NewImporter(), generator.NewDefault())

	scenarioRepo := scenario.NewRepository(repo)
	scenarioSvc := scenario.NewService(scenarioRepo)

	reportRepo := report.NewRepository(repo)
	paramSourceRepo := paramsource.NewRepository(repo)
	paramSourceSvc := paramsource.NewService(paramSourceRepo, paramsource.NewExecutor())
	scriptLibraryRepo := scriptlibrary.NewRepository(repo)
	scriptLibrarySvc := scriptlibrary.NewService(scriptLibraryRepo)
	mockServerRepo := mockserver.NewRepository(repo)
	mockServerRuntime := mockserver.NewRuntime(mockServerRepo)
	mockServerSvc := mockserver.NewService(mockServerRepo, mockServerRuntime)
	aiProviderRepo := aiprovider.NewRepository(repo)
	aiProviderSvc := aiprovider.NewService(aiProviderRepo)
	projectPromptRepo := projectprompt.NewRepository(repo)
	projectPromptSvc := projectprompt.NewService(projectPromptRepo)
	caseRunner := runner.New(nil, nil, reportRepo)
	runSvc := runner.NewService(caseSvc, projectSvc, reportRepo, caseRunner, paramSourceSvc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(devCORS)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	api := chi.NewRouter()
	authHandler := auth.NewHandler(authSvc)
	projectHandler := project.NewHandler(projectSvc)
	authHandler.RegisterPublic(api)
	api.Group(func(r chi.Router) {
		r.Use(authSvc.Authenticate)
		authHandler.RegisterProtected(r)
		projectHandler.Register(r)
		testcase.NewHandler(caseSvc).Register(r)
		spec.NewHandler(specSvc).Register(r)
		scenario.NewHandler(scenarioSvc).Register(r)
		paramsource.NewHandler(paramSourceSvc).Register(r)
		scriptlibrary.NewHandler(scriptLibrarySvc).Register(r)
		mockserver.NewHandler(mockServerSvc, projectHandler).Register(r)
		projectprompt.NewHandler(projectPromptSvc, projectHandler).Register(r)
		aiprovider.NewHandler(aiProviderSvc, projectHandler, projectPromptSvc).Register(r)
		runner.NewHandler(runSvc, scenarioRepo).Register(r)
	})
	r.Mount("/api/v1", api)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("api listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func devCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
