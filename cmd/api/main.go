package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"autotest/internal/aianalysis"
	"autotest/internal/aiassert"
	"autotest/internal/aiconfig"
	"autotest/internal/aifeedback"
	"autotest/internal/aimemory"
	ainladapter "autotest/internal/ainl/adapter"
	"autotest/internal/aiprovider"
	"autotest/internal/aisession"
	"autotest/internal/aiskill"
	"autotest/internal/aitools"
	"autotest/internal/aitools/builtin"
	"autotest/internal/apikey"
	"autotest/internal/auditlog"
	"autotest/internal/auth"
	"autotest/internal/authprovider"
	testcase "autotest/internal/case"
	"autotest/internal/config"
	"autotest/internal/evalagent"
	"autotest/internal/generator"
	"autotest/internal/genagent"
	"autotest/internal/httpx"
	"autotest/internal/logx"
	"autotest/internal/mockrecord"
	"autotest/internal/mockserver"
	"autotest/internal/mockset"
	"autotest/internal/notification"
	"autotest/internal/paramsource"
	"autotest/internal/project"
	"autotest/internal/projectprompt"
	"autotest/internal/promptlayer"
	"autotest/internal/ratelimit"
	"autotest/internal/report"
	"autotest/internal/runner"
	"autotest/internal/scenario"
	"autotest/internal/scenariogen"
	"autotest/internal/scriptlibrary"
	"autotest/internal/spec"
	"autotest/internal/store"
	"autotest/internal/testdata"
	"autotest/internal/testprofile"

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
	testProfileRepo := testprofile.NewPGRepository(repo)
	testProfileDS := testprofile.NewPGDataSource(repo)
	testProfileSvc := testprofile.NewService(testProfileRepo, testProfileDS)

	caseSvc := testcase.NewService(caseRepo, specRepo).WithProfileGetter(testProfileSvc)
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
	mockRecordRepo := mockrecord.NewRepository(repo)
	mockRecorder := mockrecord.NewRecorder(mockRecordRepo)
	mockPlayer := mockrecord.NewPlayer(mockRecordRepo)
	mockServerRuntime := mockserver.NewRuntime(mockServerRepo, mockSetSvc, &recordProxyAdapter{rec: mockRecorder}, &replayPlayerAdapter{player: mockPlayer})
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
	aiProviderSvc.WithProfileContext(testProfileSvc)
	aiProviderSvc.WithEndpointSchema(endpointSchemaAdapter{repo: specRepo})
	aiProviderSvc.WithConvention(conventionAdapter{svc: testProfileSvc})

	// Phase 2+ services: Memory, Skills, Prompt Layers, Rate Limits
	aiMemoryRepo := aimemory.NewRepository(repo)
	aiMemorySvc := aimemory.NewService(aiMemoryRepo)
	aiSkillRepo := aiskill.NewRepository(repo)
	aiSkillCandidateRepo := aiskill.NewCandidateRepository(repo)
	aiSkillSvc := aiskill.NewService(aiSkillRepo).WithCandidateRepo(aiSkillCandidateRepo)
	promptLayerRepo := promptlayer.NewRepository(repo)
	promptLayerSvc := promptlayer.NewService(promptLayerRepo)
	rateLimitRepo := ratelimit.NewRepository(repo)
	rateLimitSvc := ratelimit.NewService(rateLimitRepo)

	// Self-evolution services: Feedback, Eval Agent
	aiFeedbackRepo := aifeedback.NewRepository(repo)
	aiFeedbackSvc := aifeedback.NewService(aiFeedbackRepo)
	evalAgentRepo := evalagent.NewRepository(repo)
	evalAgentEvaluator := evalagent.NewEvaluator(evalAgentRepo)

	// Configure skill discovery with routing log and outcome queriers
	aiSkillSvc.WithDiscovery(&aiskill.DiscoveryDeps{
		RoutingLogQuerier:  &routingLogQuerierAdapter{repo: aiSessionRepo},
		ToolOutcomeQuerier: &toolOutcomeQuerierAdapter{repo: evalAgentRepo},
		CandidateRepo:      aiSkillCandidateRepo,
		SkillRepo:          aiSkillRepo,
	})
	caseRunner := runner.New(nil, nil, reportRepo)
	runSvc := runner.NewService(caseSvc, projectSvc, reportRepo, caseRunner, paramSourceSvc, testDataSvc, mockSetSvc)

	scenarioGen := scenariogen.NewGenerator(scenariogen.Deps{
		Cases:     caseSvc,
		Scenarios: scenarioSvc,
		Specs:     specRepo,
		Generator: generator.NewDefault(),
	})
	genAgentRepo := genagent.NewPGRepository(repo)
	genAgent := &genagent.Agent{
		Jobs:      genAgentRepo,
		Generator: scenarioGen,
		Runner: &genagent.RunnerAdapter{
			Svc:  runSvc,
			Repo: scenarioRepo,
		},
		Scenarios:    scenarioSvc,
		ScenarioRepo: scenarioRepo,
		Repairer: &genagent.Repairer{
			Scenarios: scenarioSvc,
			AI:        genagent.NewAIClassifier(aiProviderSvc),
		},
	}
	if aiconfig.ScenarioAutorunEnabled() {
		logx.Info("ai scenario autorun enabled")
	}

	aiSettingsStore := aiconfig.NewStore()
	aiconfig.SetGlobalStore(aiSettingsStore)
	aiConfigSvc := aiconfig.NewService(aiconfig.NewRepository(repo), aiSettingsStore)
	if err := aiConfigSvc.Load(ctx); err != nil {
		logx.Warn("load ai assistant settings", "err", err)
	}

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

		// 智能断言推断引擎：从 OpenAPI schema 和历史响应自动生成断言
		aiAssertEngine := aiassert.NewInferEngine(
			&aiAssertEndpointGetter{repo: specRepo},
			&aiAssertCaseGetter{repo: caseRepo},
			&aiAssertHistoryCollector{repo: reportRepo},
		)
		aiassert.NewHandler(aiassert.NewInferService(aiAssertEngine, &aiAssertCasePatcher{svc: caseSvc})).Register(r)
		paramsource.NewHandler(paramSourceSvc, authSvc).Register(r)
		scriptlibrary.NewHandler(scriptLibrarySvc).Register(r)
		mockserver.NewHandler(mockServerSvc, projectHandler).Register(r)
		mockrecord.NewHandler(mockPlayer, &routeModeAdapter{svc: mockServerSvc}, projectHandler).Register(r)
		mockset.NewHandler(mockSetSvc, projectHandler).Register(r)
		projectprompt.NewHandler(projectPromptSvc, authSvc).Register(r)
		aiconfig.NewHandler(aiConfigSvc, authSvc).Register(r)
		scenariogen.NewHandler(scenariogen.Deps{
			Cases:     caseSvc,
			Scenarios: scenarioSvc,
			Specs:     specRepo,
		}, projectSvc, projectHandler).Register(r)

		// 全局 AI 助理与智能分析共用一份内置工具配置：智能分析仅注入只读
		// 工具，浮窗会话再额外挂载写工具。delete_* 类工具（RequiresConfirm）
		// 会挂起 SSE 等待用户确认；create/update 类写工具在流内自动执行。
		toolDeps := builtin.Deps{
			Cases:        caseSvc,
			Scenarios:    scenarioSvc,
			Specs:        specRepo,
			Generator:    generator.NewDefault(),
			SpecImporter: specSvc,
			Projects:     projectSvc,
			ParamSource:  paramSourceSvc,
			TestData:     testDataSvc,
			MockServers:  mockServerSvc,
			MockSets:     mockSetSvc,
			Scripts:      scriptLibrarySvc,
			Runs:         runSvc,
			GenAgent:     genAgent,
			AI:           &ainladapter.ChatProviderAdapter{AI: aiProviderSvc},
		}
		aiReadOnly := builtin.ReadOnly(toolDeps)
		mutatingTools := builtin.Mutating(toolDeps)
		if genAgent != nil {
			mutatingTools = append(mutatingTools, builtin.GatedScenarioGenTools(toolDeps)...)
		}
		// 按需工具表面：Catalog 索引全部领域工具，find_tools/describe_tools
		// 元工具基于该 Catalog 构建，并加入执行用全量工具集（既进 catalog
		// 的 wire 执行 map，也作为 Router/Planner 的目录底座）。
		domainTools := append(aiReadOnly, mutatingTools...)
		aiCatalog := aitools.NewCatalog(domainTools)
		if embIdx, err := aitools.LoadEmbeddedToolEmbeddings(); err != nil {
			logx.Warn("load tool catalog embeddings", "err", err)
		} else if embIdx != nil {
			aiCatalog = aiCatalog.WithEmbeddings(embIdx)
			logx.Info("tool catalog embeddings loaded", "model", embIdx.Model, "tools", len(embIdx.Vectors), "dim", embIdx.Dimensions)
		}
		aiAllTools := append(domainTools,
			aitools.FindToolsTool(aiCatalog),
			aitools.DescribeToolsTool(aiCatalog),
		)
		aiRoutingMode := aiprovider.ParseRoutingMode(aiconfig.Get().ToolRoutingMode)
		aiProviderSvc.WithRouting(
			aiprovider.NewPlanner(aiCatalog),
			aiprovider.NewRouter(),
			aiCatalog,
			aiRoutingMode,
		)
		// Inject skill matcher so Router considers matched skills
		aiProviderSvc.WithSkillMatcher(aiskill.NewServiceAdapter(aiSkillSvc))
		logx.Info("ai tool routing", "mode", string(aiRoutingMode), "tools", len(aiAllTools))

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
		testprofile.NewHandler(testProfileSvc).Register(r)

		// Phase 2+ handlers
		aimemory.NewHandler(aiMemorySvc).Register(r)
		aiskill.NewHandler(aiSkillSvc).Register(r)
		promptlayer.NewHandler(promptLayerSvc).Register(r)
		ratelimit.NewHandler(rateLimitSvc).Register(r)

		// Self-evolution handlers
		aifeedback.NewHandler(aiFeedbackSvc, authSvc).Register(r)
		evalagent.NewHandler(evalAgentEvaluator, evalAgentRepo).Register(r)
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

	// Background jobs: skill discovery (daily) and self-evaluation (weekly)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			since := time.Now().Add(-7 * 24 * time.Hour)
			if _, err := aiSkillSvc.DiscoverCandidates(context.Background(), since); err != nil {
				logx.Warn("skill discovery failed", "err", err)
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(7 * 24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			since := time.Now().Add(-7 * 24 * time.Hour)
			if _, err := evalAgentEvaluator.RunEvaluation(context.Background(), since); err != nil {
				logx.Warn("self-evaluation failed", "err", err)
			}
		}
	}()

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

// endpointSchemaAdapter bridges spec.Repository.GetEndpointByID to
// aiprovider.EndpointSchemaProvider. It converts spec.Endpoint to
// aiprovider.EndpointBrief without creating an import cycle.
type endpointSchemaAdapter struct {
	repo *spec.Repository
}

func (a endpointSchemaAdapter) GetEndpointByID(ctx context.Context, endpointID uuid.UUID) (*aiprovider.EndpointBrief, error) {
	ep, err := a.repo.GetEndpointByID(ctx, endpointID)
	if err != nil {
		return nil, err
	}
	return &aiprovider.EndpointBrief{
		ID:             ep.ID,
		Method:         ep.Method,
		Path:           ep.Path,
		OperationID:    ep.OperationID,
		Summary:        ep.Summary,
		Tags:           ep.Tags,
		RequestSchema:  ep.RequestSchema,
		ResponseSchema: ep.ResponseSchema,
	}, nil
}

// conventionAdapter bridges testprofile.Service.GetConvention to
// aiprovider.ProfileConventionProvider.
type conventionAdapter struct {
	svc *testprofile.Service
}

func (a conventionAdapter) GetConvention(ctx context.Context, projectID uuid.UUID) (*aiprovider.ConventionBrief, error) {
	brief, err := a.svc.GetConvention(ctx, projectID)
	if err != nil || brief == nil {
		return nil, err
	}
	return &aiprovider.ConventionBrief{
		Convention: brief.Convention,
		Enums:      brief.Enums,
	}, nil
}

// routingLogQuerierAdapter bridges aisession.Repository to aiskill.RoutingLogQuerier.
type routingLogQuerierAdapter struct {
	repo *aisession.Repository
}

func (a *routingLogQuerierAdapter) AggregateToolSequences(ctx context.Context, since time.Time) ([]aiskill.ToolSequenceAgg, error) {
	// Placeholder: aggregate from ai_routing_logs
	return nil, nil
}

// toolOutcomeQuerierAdapter bridges evalagent.Repository to aiskill.ToolOutcomeQuerier.
type toolOutcomeQuerierAdapter struct {
	repo *evalagent.Repository
}

func (a *toolOutcomeQuerierAdapter) AggregateSuccessByToolSequence(ctx context.Context, since time.Time) ([]aiskill.ToolSequenceSuccess, error) {
	// Placeholder: aggregate from ai_tool_outcomes
	return nil, nil
}

// aiAssertHistoryCollector bridges report.Repository to aiassert.HistoryCollector.
type aiAssertHistoryCollector struct {
	repo *report.Repository
}

func (a *aiAssertHistoryCollector) ListPassedResponsesByTestCase(ctx context.Context, testCaseID uuid.UUID, limit int) ([]aiassert.HistoryResult, error) {
	results, err := a.repo.ListPassedResponsesByTestCase(ctx, testCaseID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]aiassert.HistoryResult, 0, len(results))
	for _, r := range results {
		out = append(out, aiassert.HistoryResult{
			ResponseSnapshot: r.ResponseSnapshot,
			Status:           string(r.Status),
		})
	}
	return out, nil
}

// aiAssertEndpointGetter bridges spec.Repository to aiassert.EndpointGetter.
type aiAssertEndpointGetter struct {
	repo *spec.Repository
}

func (a *aiAssertEndpointGetter) GetEndpointByID(ctx context.Context, endpointID uuid.UUID) (*aiassert.EndpointInfo, error) {
	ep, err := a.repo.GetEndpointByID(ctx, endpointID)
	if err != nil {
		return nil, err
	}
	return &aiassert.EndpointInfo{
		ID:             ep.ID,
		Method:         ep.Method,
		Path:           ep.Path,
		OperationID:    ep.OperationID,
		Summary:        ep.Summary,
		Tags:           ep.Tags,
		ResponseSchema: ep.ResponseSchema,
	}, nil
}

// aiAssertCaseGetter bridges testcase.Repository to aiassert.CaseGetter.
type aiAssertCaseGetter struct {
	repo *testcase.Repository
}

func (a *aiAssertCaseGetter) Get(ctx context.Context, id uuid.UUID) (*aiassert.TestCaseInfo, error) {
	tc, err := a.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &aiassert.TestCaseInfo{
		ID:                   tc.ID,
		ProjectID:            tc.ProjectID,
		ServiceID:            tc.ServiceID,
		EndpointID:           tc.EndpointID,
		Method:               tc.Method,
		Path:                 tc.Path,
		Assertions:           tc.Assertions,
		LastResponseSnapshot: tc.LastResponseSnapshot,
	}, nil
}

// aiAssertCasePatcher bridges testcase.Service to aiassert.CasePatcher.
type aiAssertCasePatcher struct {
	svc *testcase.Service
}

func (a *aiAssertCasePatcher) PatchAssertions(ctx context.Context, caseID uuid.UUID, assertions json.RawMessage) error {
	_, err := a.svc.Patch(ctx, caseID, testcase.PatchInput{Assertions: assertions})
	return err
}

type recordProxyAdapter struct {
	rec *mockrecord.Recorder
}

func (a *recordProxyAdapter) RecordAndProxy(ctx context.Context, projectID, serverID, routeID uuid.UUID, targetURL string, originalReq *http.Request, reqBody []byte) (*http.Response, error) {
	_, resp, err := a.rec.RecordAndProxy(ctx, projectID, serverID, routeID, targetURL, originalReq, reqBody)
	return resp, err
}

type replayPlayerAdapter struct {
	player *mockrecord.Player
}

func (a *replayPlayerAdapter) Replay(ctx context.Context, serverID, routeID uuid.UUID, r *http.Request, reqBody []byte) (*http.Response, error) {
	resp, _, err := a.player.Replay(ctx, serverID, routeID, r, reqBody)
	return resp, err
}

type routeModeAdapter struct {
	svc *mockserver.Service
}

func (a *routeModeAdapter) EnableRecordMode(ctx context.Context, projectID, serverID, routeID uuid.UUID, targetURL string) (mockrecord.RouteModeResult, error) {
	route, err := a.svc.GetRoute(ctx, projectID, serverID, routeID)
	if err != nil {
		return mockrecord.RouteModeResult{}, err
	}
	update := mockserver.RouteInputFromRoute(*route)
	update.ResponseMode = mockserver.ResponseModeRecord
	update.RecordTargetURL = strings.TrimRight(strings.TrimSpace(targetURL), "/")
	updated, err := a.svc.UpdateRoute(ctx, projectID, serverID, routeID, update)
	if err != nil {
		return mockrecord.RouteModeResult{}, err
	}
	return mockrecord.RouteModeResult{
		RouteID:         updated.ID,
		ResponseMode:    updated.ResponseMode,
		RecordTargetURL: updated.RecordTargetURL,
	}, nil
}

func (a *routeModeAdapter) EnableReplayMode(ctx context.Context, projectID, serverID, routeID uuid.UUID) (mockrecord.RouteModeResult, error) {
	route, err := a.svc.GetRoute(ctx, projectID, serverID, routeID)
	if err != nil {
		return mockrecord.RouteModeResult{}, err
	}
	update := mockserver.RouteInputFromRoute(*route)
	update.ResponseMode = mockserver.ResponseModeReplay
	updated, err := a.svc.UpdateRoute(ctx, projectID, serverID, routeID, update)
	if err != nil {
		return mockrecord.RouteModeResult{}, err
	}
	return mockrecord.RouteModeResult{
		RouteID:      updated.ID,
		ResponseMode: updated.ResponseMode,
	}, nil
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
