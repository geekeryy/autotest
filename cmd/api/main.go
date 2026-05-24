package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"autotest/internal/aianalysis"
	"autotest/internal/aiprovider"
	"autotest/internal/aisession"
	"autotest/internal/aitools/builtin"
	"autotest/internal/apikey"
	"autotest/internal/auditlog"
	"autotest/internal/auth"
	"autotest/internal/authprovider"
	testcase "autotest/internal/case"
	"autotest/internal/config"
	"autotest/internal/generator"
	"autotest/internal/httpx"
	"autotest/internal/logx"
	"autotest/internal/mockserver"
	"autotest/internal/mockset"
	"autotest/internal/notification"
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
	cfg, err := config.Load()
	if err != nil {
		logx.InitWith("error", "text")
		logx.Error("load config", "err", err)
		os.Exit(1)
	}

	logx.InitWith(cfg.LogLevel, cfg.LogFormat)
	logx.Info("config loaded", "summary", cfg.String())

	ctx := context.Background()
	db, err := store.Open(ctx, store.ConfigFromURL(cfg.DatabaseURL))
	if err != nil {
		logx.Error("open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := store.NewRepository(db)

	authRepo := auth.NewRepository(repo)
	authSettings := auth.Settings{
		JWTSecret:     cfg.JWTSecret,
		IsDevelopment: cfg.IsDevelopment(),
	}
	authSvc := auth.NewService(authRepo, authSettings)
	authProviderRepo := authprovider.NewRepository(repo)
	authOAuthRegistry := authprovider.NewRegistry(nil)
	authProviderSvc := authprovider.NewService(authProviderRepo, authOAuthRegistry)
	authSvc.WithAuthProvider(authProviderSvc, authOAuthRegistry)
	auditLogRepo := auditlog.NewRepository(repo)
	auditLogSvc := auditlog.NewService(auditLogRepo)
	authSvc.WithLoginAuditor(auditLogSvc)
	if err := authSvc.EnsureDefaultAdmin(ctx); err != nil {
		logx.Error("ensure default admin", "err", err)
		os.Exit(1)
	}

	projectRepo := project.NewRepository(repo)
	projectSvc := project.NewServiceLayer(projectRepo)

	caseRepo := testcase.NewRepository(repo)
	specRepo := spec.NewRepository(repo)

	caseSvc := testcase.NewService(caseRepo, specRepo)
	specSvc := spec.NewService(specRepo, caseRepo, spec.NewImporter(), generator.NewDefault())
	notificationRepo := notification.NewRepository(repo)
	notificationSvc := notification.NewService(notificationRepo)

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
		logx.Warn("auto-start mock servers", "err", err)
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
	aiSessionRepo := aisession.NewRepository(repo)
	aiSessionSvc := aisession.NewService(aiSessionRepo)
	caseRunner := runner.New(nil, nil, reportRepo)
	runSvc := runner.NewService(caseSvc, projectSvc, reportRepo, caseRunner, paramSourceSvc, testDataSvc, mockSetSvc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logx.RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(timeoutExceptStream(60 * time.Second))
	if cfg.EnableCORS() {
		r.Use(auth.CORSMiddleware(cfg.CORSAllowedOrigins, cfg.EnableDevCORS()))
		if !cfg.EnableDevCORS() && len(cfg.CORSAllowedOrigins) == 0 {
			logx.Warn("CORS_ALLOWED_ORIGINS is empty; cross-origin browser API calls will be rejected (same-origin deployments are unaffected)")
		}
	}

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	api := chi.NewRouter()
	authHandler := auth.NewHandler(authSvc)
	projectHandler := project.NewHandler(projectSvc)
	specHandler := spec.NewHandler(specSvc, notificationSvc)
	authHandler.RegisterPublic(api)

	authSvc.WithPendingUserNotifier(func(ctx context.Context, user *auth.User) error {
		adminIDs, err := authRepo.ListUserIDsByPermission(ctx, auth.PermissionUsersManage)
		if err != nil {
			return err
		}
		return notificationSvc.CreateUserPendingNotifications(ctx, adminIDs, user.ID, user.Username, user.DisplayName, user.Email)
	})

	api.Group(func(r chi.Router) {
		r.Use(authSvc.Authenticate)
		authHandler.RegisterAuthenticated(r)
	})

	// 主受保护路由组：JWT 与 API Key 共用入口，但默认通过 RejectAPIKey 拒绝 API Key，
	// 避免新增加的 API Key 来源意外触达未经审计的接口。
	api.Group(func(r chi.Router) {
		r.Use(authSvc.Authenticate)
		r.Use(authSvc.RequireActiveUser())
		r.Use(authSvc.RequirePasswordChanged())
		r.Use(authSvc.RejectAPIKey())
		authHandler.RegisterProtected(r)
		authprovider.NewHandler(authProviderSvc, authSvc.RequirePermission(auth.PermissionUsersManage)).Register(r)
		projectHandler.Register(r)
		testcase.NewHandler(caseSvc).Register(r)
		specHandler.Register(r)
		scenario.NewHandler(scenarioSvc).Register(r)
		paramsource.NewHandler(paramSourceSvc, authSvc).Register(r)
		scriptlibrary.NewHandler(scriptLibrarySvc).Register(r)
		mockserver.NewHandler(mockServerSvc, projectHandler).Register(r)
		mockset.NewHandler(mockSetSvc, projectHandler).Register(r)
		projectprompt.NewHandler(projectPromptSvc, authSvc).Register(r)

		// 全局 AI 助理与智能分析共用一份内置工具配置：智能分析仅注入只读
		// 工具，浮窗会话再额外挂载受控写工具。任何写工具调用都会被 SSE 流
		// 程挂起，等待用户在前端确认后才真正执行。
		toolDeps := builtin.Deps{
			Cases:     caseSvc,
			Scenarios: scenarioSvc,
			Specs:     specRepo,
			Projects:  projectSvc,
		}
		aiReadOnly := builtin.ReadOnly(toolDeps)
		aiAllTools := builtin.All(toolDeps)

		aiprovider.NewHandler(aiProviderSvc, projectHandler, projectPromptSvc, authSvc).
			WithAssistant(aisession.NewStoreAdapter(aiSessionSvc), aiAllTools).
			Register(r)
		aisession.NewHandler(aiSessionSvc, projectHandler).Register(r)

		testdata.NewHandler(testDataSvc, projectHandler).Register(r)
		runner.NewHandler(runSvc, scenarioRepo, projectSvc).Register(r)
		apikey.NewHandler(apiKeySvc, authSvc.RequirePermission).Register(r)
		notification.NewHandler(notificationSvc).Register(r)
		auditlog.NewHandler(auditLogSvc).Register(r.Group(func(r chi.Router) {
			r.Use(authSvc.RequirePermission(auth.PermissionAuditRead))
		}))

		aianalysis.NewHandler(repo, aiProviderSvc, projectPromptSvc, reportRepo, projectHandler).
			WithTools(aiReadOnly).
			Register(r)
	})

	// API Key 白名单组：当前仅 OpenAPI/Swagger 导入接口允许 API Key 调用，
	// 通过 AllowAPIKeyScope("specs:import") 校验 scope；JWT 来源不受影响。
	api.Group(func(r chi.Router) {
		r.Use(authSvc.Authenticate)
		r.Use(authSvc.RequireActiveUser())
		r.Use(authSvc.RequirePasswordChanged())
		r.Use(authSvc.AllowAPIKeyScope(apikey.ScopeSpecsImport))
		specHandler.RegisterImport(r)
	})

	r.Mount("/api/v1", api)
	registerAdminUI(r)

	logx.Info("api listening", "addr", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, r); err != nil {
		logx.Error("listen", "err", err)
		os.Exit(1)
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

// timeoutExceptStream wraps the standard chi Timeout middleware but lets
// long-lived SSE endpoints through untouched. Without this bypass the
// global 60s timeout would cut every AI assistant stream short, since
// http.TimeoutHandler buffers the response writer (defeating the
// purpose of streaming) and force-closes the connection on deadline.
func timeoutExceptStream(d time.Duration) func(http.Handler) http.Handler {
	timeout := middleware.Timeout(d)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isStreamPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			timeout(next).ServeHTTP(w, r)
		})
	}
}

// isStreamPath identifies SSE endpoints. Keep this list narrow: the
// timeout middleware is a real safety net for the rest of the API.
func isStreamPath(path string) bool {
	if strings.HasSuffix(path, "/chat/stream") {
		return true
	}
	if strings.HasSuffix(path, "/notifications/stream") {
		return true
	}
	if strings.Contains(path, "/tool-calls/") && strings.HasSuffix(path, "/confirm") {
		return true
	}
	return false
}
