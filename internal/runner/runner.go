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
	Method   string                `json:"method"`
	Path     string                `json:"path"`
	URL      string                `json:"url"`
	Headers  map[string]string     `json:"headers"`
	Query    map[string]string     `json:"query"`
	Body     any                   `json:"body"`
	Security []map[string][]string `json:"security,omitempty"`
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

func (r *Runner) ExecuteCase(ctx context.Context, runID uuid.UUID, tc testcase.TestCase, env project.Environment, vars map[string]string) (*report.Result, error) {
	start := time.Now()
	reqDef, err := decodeRequest(tc.Request)
	if err != nil {
		return r.finish(ctx, report.Result{
			RunID:          runID,
			TestCaseID:     tc.ID,
			Status:         report.ResultError,
			DurationMillis: time.Since(start).Milliseconds(),
			Error:          err.Error(),
		})
	}
	if reqDef.Method == "" {
		reqDef.Method = tc.Method
	}
	if reqDef.Path == "" {
		reqDef.Path = tc.Path
	}

	httpReq, requestSnapshot, err := buildHTTPRequest(ctx, reqDef, env, vars)
	if err != nil {
		return r.finish(ctx, report.Result{
			RunID:           runID,
			TestCaseID:      tc.ID,
			Status:          report.ResultError,
			DurationMillis:  time.Since(start).Milliseconds(),
			RequestSnapshot: requestSnapshot,
			Error:           err.Error(),
		})
	}

	resp, err := r.Client.Do(httpReq)
	if err != nil {
		return r.finish(ctx, report.Result{
			RunID:           runID,
			TestCaseID:      tc.ID,
			Status:          report.ResultError,
			DurationMillis:  time.Since(start).Milliseconds(),
			RequestSnapshot: requestSnapshot,
			Error:           err.Error(),
		})
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	responseSnapshot := snapshotResponse(resp, body)
	if readErr != nil {
		return r.finish(ctx, report.Result{
			RunID:            runID,
			TestCaseID:       tc.ID,
			Status:           report.ResultError,
			DurationMillis:   time.Since(start).Milliseconds(),
			RequestSnapshot:  requestSnapshot,
			ResponseSnapshot: responseSnapshot,
			Error:            readErr.Error(),
		})
	}

	assertionResults := r.Engine.Evaluate(ctx, tc.Assertions, assertion.Input{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	})
	rawAssertions, _ := json.Marshal(assertionResults)

	status := report.ResultPassed
	if !assertion.AllPassed(assertionResults) {
		status = report.ResultFailed
	}

	return r.finish(ctx, report.Result{
		RunID:            runID,
		TestCaseID:       tc.ID,
		Status:           status,
		DurationMillis:   time.Since(start).Milliseconds(),
		RequestSnapshot:  requestSnapshot,
		ResponseSnapshot: responseSnapshot,
		Assertions:       rawAssertions,
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

func buildHTTPRequest(ctx context.Context, def RequestDefinition, env project.Environment, vars map[string]string) (*http.Request, json.RawMessage, error) {
	target := RenderVariables(def.URL, vars)
	if target == "" {
		base := strings.TrimRight(env.BaseURL, "/")
		path := "/" + strings.TrimLeft(RenderVariables(def.Path, vars), "/")
		target = base + path
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return nil, nil, fmt.Errorf("parse request url: %w", err)
	}
	query := parsed.Query()
	for key, value := range RenderMap(def.Query, vars) {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()

	body, err := json.Marshal(def.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request body: %w", err)
	}
	if def.Body == nil {
		body = nil
	}

	method := strings.ToUpper(RenderVariables(def.Method, vars))
	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}
	for key, value := range RenderMap(def.Headers, vars) {
		req.Header.Set(key, value)
	}
	if err := applyEnvironmentAuth(req, env.Auth, def, vars); err != nil {
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

func applyEnvironmentAuth(req *http.Request, rawAuth json.RawMessage, def RequestDefinition, vars map[string]string) error {
	if len(rawAuth) == 0 {
		return nil
	}

	auth, err := selectEnvironmentAuth(rawAuth, def, req.URL.Path, vars)
	if err != nil {
		return err
	}
	if auth == nil {
		return nil
	}
	return applyAuthDefinition(req, *auth, vars)
}

func selectEnvironmentAuth(rawAuth json.RawMessage, def RequestDefinition, requestPath string, vars map[string]string) (*authDefinition, error) {
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

	pathCandidates := []string{requestPath, RenderVariables(def.Path, vars)}
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

func applyAuthDefinition(req *http.Request, auth authDefinition, vars map[string]string) error {
	for key, value := range RenderMap(auth.Headers, vars) {
		setHeaderIfAbsent(req, key, value)
	}
	for key, value := range RenderMap(auth.Query, vars) {
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
		token := strings.TrimSpace(RenderVariables(tokenTemplate, vars))
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
		req.SetBasicAuth(RenderVariables(auth.Username, vars), RenderVariables(auth.Password, vars))
	case "api_key", "apikey", "api-key":
		name := strings.TrimSpace(RenderVariables(auth.Name, vars))
		valueTemplate := auth.Value
		if valueTemplate == "" {
			valueTemplate = auth.Token
		}
		value := RenderVariables(valueTemplate, vars)
		if name == "" {
			return nil
		}
		if strings.EqualFold(auth.In, "query") {
			setQueryIfAbsent(req, name, value)
		} else {
			setHeaderIfAbsent(req, name, value)
		}
	case "header":
		name := strings.TrimSpace(RenderVariables(auth.Name, vars))
		if name != "" {
			setHeaderIfAbsent(req, name, RenderVariables(auth.Value, vars))
		}
	case "query":
		name := strings.TrimSpace(RenderVariables(auth.Name, vars))
		if name != "" {
			setQueryIfAbsent(req, name, RenderVariables(auth.Value, vars))
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
