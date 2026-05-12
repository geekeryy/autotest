package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"autotest/internal/aianalysis"
	"autotest/internal/aiprovider"
	"autotest/internal/apikey"
	"autotest/internal/auth"
	testcase "autotest/internal/case"
	"autotest/internal/generator"
	"autotest/internal/httpx"
	"autotest/internal/mockserver"
	"autotest/internal/mockset"
	"autotest/internal/paramsource"
	"autotest/internal/project"
	"autotest/internal/projectprompt"
	"autotest/internal/report"
	"autotest/internal/runner"
	"autotest/internal/scenario"
	"autotest/internal/scriptlibrary"
	"autotest/internal/spec"
	"autotest/internal/store"
	"autotest/internal/testdata"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
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
	mockSetRepo := mockset.NewRepository(repo)
	mockSetSvc := mockset.NewService(mockSetRepo)
	mockServerRepo := mockserver.NewRepository(repo)
	mockServerRuntime := mockserver.NewRuntime(mockServerRepo, mockSetSvc)
	mockServerSvc := mockserver.NewService(mockServerRepo, mockServerRuntime)
	if err := mockServerSvc.AutoStartAll(ctx); err != nil {
		log.Printf("auto-start mock servers: %v", err)
	}
	aiProviderRepo := aiprovider.NewRepository(repo)
	aiProviderSvc := aiprovider.NewService(aiProviderRepo).WithMockSets(mockSetSummaryAdapter{svc: mockSetSvc})
	projectPromptRepo := projectprompt.NewRepository(repo)
	projectPromptSvc := projectprompt.NewService(projectPromptRepo)
	testDataRepo := testdata.NewRepository(repo)
	testDataSvc := testdata.NewService(testDataRepo, aiProviderSvc, projectPromptSvc)
	apiKeyRepo := apikey.NewRepository(repo)
	apiKeySvc := apikey.NewService(apiKeyRepo, authRepo)
	authSvc.WithAPIKey(apiKeySvc)
	caseRunner := runner.New(nil, nil, reportRepo)
	runSvc := runner.NewService(caseSvc, projectSvc, reportRepo, caseRunner, paramSourceSvc, testDataSvc, mockSetSvc)

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
	specHandler := spec.NewHandler(specSvc)
	authHandler.RegisterPublic(api)

	// 主受保护路由组：JWT 与 API Key 共用入口，但默认通过 RejectAPIKey 拒绝 API Key，
	// 避免新增加的 API Key 来源意外触达未经审计的接口。
	api.Group(func(r chi.Router) {
		r.Use(authSvc.Authenticate)
		r.Use(authSvc.RejectAPIKey())
		authHandler.RegisterProtected(r)
		projectHandler.Register(r)
		testcase.NewHandler(caseSvc).Register(r)
		specHandler.Register(r)
		scenario.NewHandler(scenarioSvc).Register(r)
		paramsource.NewHandler(paramSourceSvc).Register(r)
		scriptlibrary.NewHandler(scriptLibrarySvc).Register(r)
		mockserver.NewHandler(mockServerSvc, projectHandler).Register(r)
		mockset.NewHandler(mockSetSvc, projectHandler).Register(r)
		projectprompt.NewHandler(projectPromptSvc, projectHandler).Register(r)
		aiprovider.NewHandler(aiProviderSvc, projectHandler, projectPromptSvc).Register(r)
		testdata.NewHandler(testDataSvc, projectHandler).Register(r)
		runner.NewHandler(runSvc, scenarioRepo).Register(r)
		apikey.NewHandler(apiKeySvc, authSvc.RequirePermission).Register(r)
		aianalysis.NewHandler(repo, aiProviderSvc, projectPromptSvc, reportRepo, projectHandler).Register(r)
	})

	// API Key 白名单组：当前仅 OpenAPI/Swagger 导入接口允许 API Key 调用，
	// 通过 AllowAPIKeyScope("specs:import") 校验 scope；JWT 来源不受影响。
	api.Group(func(r chi.Router) {
		r.Use(authSvc.Authenticate)
		r.Use(authSvc.AllowAPIKeyScope(apikey.ScopeSpecsImport))
		specHandler.RegisterImport(r)
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

// mockSetSummaryAdapter bridges `mockset.Service.SummariesForProject` (which
// returns `[]mockset.SetSummary`) into the `aiprovider.MockSetSummaryProvider`
// interface (which expects `[]aiprovider.MockSetSummary`). Keeping the
// conversion in main keeps the two packages independent.
type mockSetSummaryAdapter struct {
	svc *mockset.Service
}

func (a mockSetSummaryAdapter) SummariesForProject(ctx context.Context, projectID uuid.UUID) []aiprovider.MockSetSummary {
	rows := a.svc.SummariesForProject(ctx, projectID)
	if len(rows) == 0 {
		return nil
	}
	out := make([]aiprovider.MockSetSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, aiprovider.MockSetSummary{
			Key:            r.Key,
			Name:           r.Name,
			ValuesPreview:  r.ValuesPreview,
			HasWeights:     r.HasWeights,
			TotalValueSize: r.TotalValueSize,
		})
	}
	return out
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
