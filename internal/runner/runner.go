package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"autotest/internal/assertion"
	testcase "autotest/internal/case"
	"autotest/internal/project"
	"autotest/internal/report"
	"autotest/internal/templating"

	"github.com/google/uuid"
)

type ResultStore interface {
	SaveResult(ctx context.Context, result report.Result) (*report.Result, error)
}

type Runner struct {
	Client  *http.Client
	Engine  *assertion.Engine
	Results ResultStore
}

type RequestDefinition struct {
	Method    string                `json:"method"`
	Path      string                `json:"path"`
	URL       string                `json:"url"`
	Headers   map[string]string     `json:"headers"`
	Query     map[string]string     `json:"query"`
	Body      any                   `json:"body"`
	Variables map[string]any        `json:"variables"`
	Security  []map[string][]string `json:"security,omitempty"`
}

func New(client *http.Client, engine *assertion.Engine, results ResultStore) *Runner {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if engine == nil {
		engine = assertion.NewEngine()
	}
	return &Runner{Client: client, Engine: engine, Results: results}
}

// defaultRetryBackoff is the base delay used when a step opts into retries
// without specifying its own backoff. Successive attempts grow exponentially.
const defaultRetryBackoff = 200 * time.Millisecond

// execOptions carry per-step execution tuning. The zero value preserves the
// historical behaviour: no extra timeout (the http.Client default applies) and
// no retries.
type execOptions struct {
	timeout time.Duration
	retry   retryPolicy
}

// ExecOption configures a single case execution. Options only affect the
// scenario step path; the standalone ExecuteCase entrypoint ignores them.
type ExecOption func(*execOptions)

// WithStepTimeout bounds the whole attempt (including any retries) with a
// context deadline. A non-positive duration is ignored.
func WithStepTimeout(d time.Duration) ExecOption {
	return func(o *execOptions) {
		if d > 0 {
			o.timeout = d
		}
	}
}

// WithStepRetry enables retrying the outbound HTTP call. Retries fire on
// transport errors always, and on the listed status codes when provided.
// Non-idempotent methods (POST/PATCH) are retried only when allowNonIdempotent
// is true, so the default policy never risks double-submitting a mutation.
func WithStepRetry(maxRetries int, backoff time.Duration, retryOnStatus []int, allowNonIdempotent bool) ExecOption {
	return func(o *execOptions) {
		if maxRetries <= 0 {
			return
		}
		if backoff <= 0 {
			backoff = defaultRetryBackoff
		}
		statuses := make(map[int]bool, len(retryOnStatus))
		for _, c := range retryOnStatus {
			statuses[c] = true
		}
		o.retry = retryPolicy{
			maxRetries:         maxRetries,
			backoff:            backoff,
			retryOnStatus:      statuses,
			allowNonIdempotent: allowNonIdempotent,
		}
	}
}

// retryPolicy is the resolved, immutable retry configuration for one step.
type retryPolicy struct {
	maxRetries         int
	backoff            time.Duration
	retryOnStatus      map[int]bool
	allowNonIdempotent bool
}

func (p retryPolicy) enabled() bool { return p.maxRetries > 0 }

// allowsMethod reports whether the policy permits retrying the given method.
// Idempotent methods are always eligible; mutating ones require an explicit
// opt-in so a retried POST cannot create duplicate records in the system
// under test.
func (p retryPolicy) allowsMethod(method string) bool {
	if p.allowNonIdempotent {
		return true
	}
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func (p retryPolicy) shouldRetryStatus(code int) bool { return p.retryOnStatus[code] }

// backoffFor returns the delay before the given attempt index (1-based for the
// first retry), growing exponentially from the base backoff.
func (p retryPolicy) backoffFor(attempt int) time.Duration {
	d := p.backoff
	for i := 1; i < attempt; i++ {
		d *= 2
	}
	return d
}

// doWithRetry issues req, retrying transport failures and configured retryable
// status codes per policy. The request body is replayed via GetBody between
// attempts. When policy is disabled or the method is not eligible, it performs
// a single Do.
func (r *Runner) doWithRetry(ctx context.Context, req *http.Request, policy retryPolicy) (*http.Response, error) {
	attempts := 1
	if policy.enabled() && policy.allowsMethod(req.Method) {
		attempts = policy.maxRetries + 1
	}

	var resp *http.Response
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if req.GetBody != nil {
				body, gerr := req.GetBody()
				if gerr != nil {
					return nil, gerr
				}
				req.Body = body
			}
			select {
			case <-time.After(policy.backoffFor(attempt)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err = r.Client.Do(req)
		last := attempt == attempts-1
		if err != nil {
			if last {
				return nil, err
			}
			continue // transport error: retry
		}
		if !last && policy.shouldRetryStatus(resp.StatusCode) {
			resp.Body.Close()
			continue
		}
		return resp, nil
	}
	return resp, err
}

// ExecuteCase runs a historical test_cases row. In product terms, SourceAuto
// and SourceManual rows are request templates, while SourceDerived rows are
// saved test case snapshots.
//
// mockCfg is optional: when non-nil it enables `{{$mock.set.*}}` expansion
// against the project-scoped value sets and the run-shared cursor map. Pass
// nil to use the standard mockdata helpers only.
func (r *Runner) ExecuteCase(ctx context.Context, runID uuid.UUID, tc testcase.TestCase, env project.Environment, vars map[string]string, parameterSourceSnapshots json.RawMessage, mockCfg *templating.MockExpanderConfig) (*report.Result, error) {
	return r.executeCase(ctx, runID, nil, tc, env, vars, parameterSourceSnapshots, mockCfg)
}

// ExecuteCaseWithStepID is ExecuteCase with an attached scenario step result.
// Optional ExecOptions apply per-step timeout/retry tuning.
func (r *Runner) ExecuteCaseWithStepID(ctx context.Context, runID uuid.UUID, stepID uuid.UUID, tc testcase.TestCase, env project.Environment, vars map[string]string, parameterSourceSnapshots json.RawMessage, mockCfg *templating.MockExpanderConfig, opts ...ExecOption) (*report.Result, error) {
	return r.executeCase(ctx, runID, &stepID, tc, env, vars, parameterSourceSnapshots, mockCfg, opts...)
}

func (r *Runner) executeCase(ctx context.Context, runID uuid.UUID, stepID *uuid.UUID, tc testcase.TestCase, env project.Environment, vars map[string]string, parameterSourceSnapshots json.RawMessage, mockCfg *templating.MockExpanderConfig, opts ...ExecOption) (*report.Result, error) {
	start := time.Now()

	var cfg execOptions
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}
	reqDef, err := decodeRequest(tc.Request)
	if err != nil {
		return r.finish(ctx, report.Result{
			RunID:                    runID,
			TestCaseID:               tc.ID,
			StepID:                   stepID,
			Status:                   report.ResultError,
			DurationMillis:           time.Since(start).Milliseconds(),
			ParameterSourceSnapshots: parameterSourceSnapshots,
			Error:                    err.Error(),
		})
	}
	if reqDef.Method == "" {
		reqDef.Method = tc.Method
	}
	if reqDef.Path == "" {
		reqDef.Path = tc.Path
	}

	httpReq, requestSnapshot, err := buildHTTPRequest(ctx, reqDef, env, vars, mockCfg)
	if err != nil {
		return r.finish(ctx, report.Result{
			RunID:                    runID,
			TestCaseID:               tc.ID,
			StepID:                   stepID,
			Status:                   report.ResultError,
			DurationMillis:           time.Since(start).Milliseconds(),
			RequestSnapshot:          requestSnapshot,
			ParameterSourceSnapshots: parameterSourceSnapshots,
			Error:                    err.Error(),
		})
	}

	resp, err := r.doWithRetry(ctx, httpReq, cfg.retry)
	if err != nil {
		return r.finish(ctx, report.Result{
			RunID:                    runID,
			TestCaseID:               tc.ID,
			StepID:                   stepID,
			Status:                   report.ResultError,
			DurationMillis:           time.Since(start).Milliseconds(),
			RequestSnapshot:          requestSnapshot,
			ParameterSourceSnapshots: parameterSourceSnapshots,
			Error:                    err.Error(),
		})
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	durationMillis := time.Since(start).Milliseconds()
	responseSnapshot := snapshotResponse(resp, body)
	if readErr != nil {
		return r.finish(ctx, report.Result{
			RunID:                    runID,
			TestCaseID:               tc.ID,
			StepID:                   stepID,
			Status:                   report.ResultError,
			DurationMillis:           durationMillis,
			RequestSnapshot:          requestSnapshot,
			ResponseSnapshot:         responseSnapshot,
			ParameterSourceSnapshots: parameterSourceSnapshots,
			Error:                    readErr.Error(),
		})
	}

	assertionResults := r.Engine.Evaluate(ctx, tc.Assertions, assertion.Input{
		StatusCode:     resp.StatusCode,
		Headers:        resp.Header,
		Body:           body,
		DurationMillis: durationMillis,
		Variables:      vars,
	})
	rawAssertions, _ := json.Marshal(assertionResults)

	status := report.ResultPassed
	if !assertion.AllPassed(assertionResults) {
		status = report.ResultFailed
	}

	return r.finish(ctx, report.Result{
		RunID:                    runID,
		TestCaseID:               tc.ID,
		StepID:                   stepID,
		Status:                   status,
		DurationMillis:           durationMillis,
		RequestSnapshot:          requestSnapshot,
		ResponseSnapshot:         responseSnapshot,
		Assertions:               rawAssertions,
		ParameterSourceSnapshots: parameterSourceSnapshots,
	})
}

func (r *Runner) finish(ctx context.Context, result report.Result) (*report.Result, error) {
	if r.Results == nil {
		return &result, nil
	}
	return r.Results.SaveResult(ctx, result)
}

func decodeRequest(raw json.RawMessage) (RequestDefinition, error) {
	var req RequestDefinition
	if len(raw) == 0 {
		return req, nil
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return req, fmt.Errorf("decode request definition: %w", err)
	}
	return req, nil
}

func buildHTTPRequest(ctx context.Context, def RequestDefinition, env project.Environment, vars map[string]string, mockCfg *templating.MockExpanderConfig) (*http.Request, json.RawMessage, error) {
	effectiveVars := mergeRequestVariables(def.Variables, vars)
	target := renderVariables(def.URL, effectiveVars, mockCfg)
	if target == "" {
		base := strings.TrimRight(env.BaseURL, "/")
		path := "/" + strings.TrimLeft(renderPathVariables(def.Path, effectiveVars, mockCfg), "/")
		target = base + path
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return nil, nil, fmt.Errorf("parse request url: %w", err)
	}
	query := parsed.Query()
	for key, value := range renderMap(def.Query, effectiveVars, mockCfg) {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()

	renderedBody := renderAny(def.Body, effectiveVars, mockCfg)
	body, err := json.Marshal(renderedBody)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request body: %w", err)
	}
	if def.Body == nil {
		body = nil
	}

	method := strings.ToUpper(renderVariables(def.Method, effectiveVars, mockCfg))
	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	for key, value := range renderMap(def.Headers, effectiveVars, mockCfg) {
		req.Header.Set(key, value)
	}
	if err := applyEnvironmentAuth(req, env.Auth, def, effectiveVars, mockCfg); err != nil {
		return nil, nil, err
	}

	snapshot := map[string]any{
		"method":  method,
		"url":     req.URL.String(),
		"headers": req.Header,
	}
	if body != nil {
		snapshot["body"] = json.RawMessage(body)
	}
	rawSnapshot, _ := json.Marshal(snapshot)
	return req, rawSnapshot, nil
}

func mergeRequestVariables(defaults map[string]any, overrides map[string]string) map[string]string {
	if len(defaults) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]string, len(defaults)+len(overrides))
	for key, value := range defaults {
		merged[key] = variableString(value)
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

func hasSecurityRequirements(security []map[string][]string) bool {
	for _, requirement := range security {
		if len(requirement) > 0 {
			return true
		}
	}
	return false
}

type authDefinition struct {
	Type     string            `json:"type"`
	Token    string            `json:"token"`
	Username string            `json:"username"`
	Password string            `json:"password"`
	Name     string            `json:"name"`
	Value    string            `json:"value"`
	In       string            `json:"in"`
	Headers  map[string]string `json:"headers"`
	Query    map[string]string `json:"query"`
}

type environmentAuthConfig struct {
	DefaultProfile  string                    `json:"defaultProfile"`
	Profiles        map[string]authDefinition `json:"profiles"`
	SecuritySchemes map[string]string         `json:"securitySchemes"`
	PathRules       []authPathRule            `json:"pathRules"`
}

type authPathRule struct {
	Prefix  string `json:"prefix"`
	Path    string `json:"path"`
	Profile string `json:"profile"`
}

func applyEnvironmentAuth(req *http.Request, rawAuth json.RawMessage, def RequestDefinition, vars map[string]string, mockCfg *templating.MockExpanderConfig) error {
	if len(rawAuth) == 0 {
		return nil
	}

	auth, err := selectEnvironmentAuth(rawAuth, def, req.URL.Path, vars, mockCfg)
	if err != nil {
		return err
	}
	if auth == nil {
		return nil
	}
	return applyAuthDefinition(req, *auth, vars, mockCfg)
}

func selectEnvironmentAuth(rawAuth json.RawMessage, def RequestDefinition, requestPath string, vars map[string]string, mockCfg *templating.MockExpanderConfig) (*authDefinition, error) {
	var cfg environmentAuthConfig
	if err := json.Unmarshal(rawAuth, &cfg); err != nil {
		return nil, fmt.Errorf("decode environment auth: %w", err)
	}
	if len(cfg.Profiles) == 0 {
		if !hasSecurityRequirements(def.Security) {
			return nil, nil
		}
		var legacy authDefinition
		if err := json.Unmarshal(rawAuth, &legacy); err != nil {
			return nil, fmt.Errorf("decode environment auth: %w", err)
		}
		return &legacy, nil
	}

	if hasSecurityRequirements(def.Security) {
		if profileName, ok, err := profileNameForSecurity(cfg, def.Security); ok || err != nil {
			if err != nil {
				return nil, err
			}
			return authProfile(cfg, profileName)
		}
	}

	pathCandidates := []string{requestPath, renderPathVariables(def.Path, vars, mockCfg)}
	if profileName := profileNameForPathRules(cfg, pathCandidates); profileName != "" {
		return authProfile(cfg, profileName)
	}

	// Fallback to the configured default profile. We always try the default
	// profile when one is configured, even if the request definition has no
	// `security` field; that way a request whose `security` was lost in
	// transit (e.g. by an older client or a saved snapshot) still picks up
	// the operator's intended default credentials. Profiles that resolve to
	// type "none" or empty tokens still no-op inside applyAuthDefinition, so
	// public endpoints remain unaffected.
	if profileName := strings.TrimSpace(cfg.DefaultProfile); profileName != "" {
		return authProfile(cfg, profileName)
	}
	return nil, nil
}

func profileNameForSecurity(cfg environmentAuthConfig, security []map[string][]string) (string, bool, error) {
	for _, requirement := range security {
		for scheme := range requirement {
			scheme = strings.TrimSpace(scheme)
			if scheme == "" {
				continue
			}
			if profileName := strings.TrimSpace(cfg.SecuritySchemes[scheme]); profileName != "" {
				if _, ok := cfg.Profiles[profileName]; !ok {
					return "", true, fmt.Errorf("environment auth profile %q for security scheme %q not found", profileName, scheme)
				}
				return profileName, true, nil
			}
			if _, ok := cfg.Profiles[scheme]; ok {
				return scheme, true, nil
			}
		}
	}
	return "", false, nil
}

func profileNameForPathRules(cfg environmentAuthConfig, paths []string) string {
	bestProfile := ""
	bestLen := -1
	for _, rule := range cfg.PathRules {
		profile := strings.TrimSpace(rule.Profile)
		if profile == "" {
			continue
		}
		for _, prefix := range []string{rule.Prefix, rule.Path} {
			prefix = normalizeAuthPath(prefix)
			if prefix == "" {
				continue
			}
			for _, path := range paths {
				path = normalizeAuthPath(path)
				if pathMatchesAuthPrefix(path, prefix) && len(prefix) > bestLen {
					bestProfile = profile
					bestLen = len(prefix)
				}
			}
		}
	}
	return bestProfile
}

func normalizeAuthPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func pathMatchesAuthPrefix(path string, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, strings.TrimRight(prefix, "/")+"/")
}

func authProfile(cfg environmentAuthConfig, profileName string) (*authDefinition, error) {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return nil, nil
	}
	auth, ok := cfg.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("environment auth profile %q not found", profileName)
	}
	return &auth, nil
}

func applyAuthDefinition(req *http.Request, auth authDefinition, vars map[string]string, mockCfg *templating.MockExpanderConfig) error {
	for key, value := range renderMap(auth.Headers, vars, mockCfg) {
		setHeaderIfAbsent(req, key, value)
	}
	for key, value := range renderMap(auth.Query, vars, mockCfg) {
		setQueryIfAbsent(req, key, value)
	}

	switch strings.ToLower(strings.TrimSpace(auth.Type)) {
	case "", "none":
		return nil
	case "bearer", "jwt", "oauth2":
		tokenTemplate := auth.Token
		if tokenTemplate == "" {
			tokenTemplate = auth.Value
		}
		token := strings.TrimSpace(renderVariables(tokenTemplate, vars, mockCfg))
		if token == "" {
			return nil
		}
		if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
			token = "Bearer " + token
		}
		setHeaderIfAbsent(req, "Authorization", token)
	case "basic":
		if req.Header.Get("Authorization") != "" {
			return nil
		}
		req.SetBasicAuth(renderVariables(auth.Username, vars, mockCfg), renderVariables(auth.Password, vars, mockCfg))
	case "api_key", "apikey", "api-key":
		name := strings.TrimSpace(renderVariables(auth.Name, vars, mockCfg))
		valueTemplate := auth.Value
		if valueTemplate == "" {
			valueTemplate = auth.Token
		}
		value := renderVariables(valueTemplate, vars, mockCfg)
		if name == "" {
			return nil
		}
		if strings.EqualFold(auth.In, "query") {
			setQueryIfAbsent(req, name, value)
		} else {
			setHeaderIfAbsent(req, name, value)
		}
	case "header":
		name := strings.TrimSpace(renderVariables(auth.Name, vars, mockCfg))
		if name != "" {
			setHeaderIfAbsent(req, name, renderVariables(auth.Value, vars, mockCfg))
		}
	case "query":
		name := strings.TrimSpace(renderVariables(auth.Name, vars, mockCfg))
		if name != "" {
			setQueryIfAbsent(req, name, renderVariables(auth.Value, vars, mockCfg))
		}
	default:
		return fmt.Errorf("unsupported environment auth type %q", auth.Type)
	}
	return nil
}

func setHeaderIfAbsent(req *http.Request, key string, value string) {
	key = strings.TrimSpace(key)
	if key == "" || req.Header.Get(key) != "" {
		return
	}
	req.Header.Set(key, value)
}

func setQueryIfAbsent(req *http.Request, key string, value string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	query := req.URL.Query()
	if query.Has(key) {
		return
	}
	query.Set(key, value)
	req.URL.RawQuery = query.Encode()
}

func snapshotResponse(resp *http.Response, body []byte) json.RawMessage {
	snapshot := map[string]any{
		"statusCode": resp.StatusCode,
		"headers":    resp.Header,
		"body":       string(body),
	}
	raw, _ := json.Marshal(snapshot)
	return raw
}
