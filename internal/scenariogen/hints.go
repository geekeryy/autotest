package scenariogen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	testcase "autotest/internal/case"
	"autotest/internal/project"
	"autotest/internal/spec"

	"github.com/google/uuid"
)

// LoginHintSource describes where a credential hint came from.
type LoginHintSource string

const (
	HintSourceSavedCase       LoginHintSource = "saved_case"
	HintSourceEnvironmentAuth LoginHintSource = "environment_auth"
	HintSourceEnvironmentVar  LoginHintSource = "environment_variable"
)

// LoginEndpointHint summarizes discoverable credentials for one login endpoint.
type LoginEndpointHint struct {
	EndpointID   uuid.UUID       `json:"endpointId,omitempty"`
	CaseID       uuid.UUID       `json:"caseId,omitempty"`
	Method       string          `json:"method"`
	Path         string          `json:"path"`
	Role         string          `json:"role"` // user | admin
	Source       LoginHintSource `json:"source"`
	Username     string              `json:"username,omitempty"`
	PasswordHint string              `json:"passwordHint,omitempty"`
	ProfileName  string              `json:"profileName,omitempty"`
	VariableName string              `json:"variableName,omitempty"`
	BodyFields   []LoginBodyFieldHint `json:"bodyFields,omitempty"`
}

// LoginHintsResponse is returned by list_scenario_login_hints and the REST hints API.
type LoginHintsResponse struct {
	Hints    []LoginEndpointHint `json:"hints"`
	Guidance string              `json:"guidance"`
}

// EnvironmentReader loads service environments for hint discovery.
type EnvironmentReader interface {
	GetServiceEnvironment(ctx context.Context, projectID, serviceID, environmentID uuid.UUID) (*project.Environment, error)
}

// CollectLoginHints scans saved cases and optional environment config for login credential candidates.
func CollectLoginHints(ctx context.Context, deps Deps, envReader EnvironmentReader, projectID, serviceID uuid.UUID, environmentID uuid.UUID) (*LoginHintsResponse, error) {
	if deps.Cases == nil || deps.Specs == nil {
		return nil, fmt.Errorf("scenariogen: 依赖未配置")
	}
	endpoints, err := deps.Specs.ListEndpoints(ctx, projectID, serviceID)
	if err != nil {
		return nil, err
	}
	cases, err := deps.Cases.List(ctx, testcase.ListFilter{ProjectID: projectID, ServiceID: serviceID})
	if err != nil {
		return nil, err
	}
	caseByEndpoint := map[uuid.UUID]*testcase.TestCase{}
	caseByKey := map[string]*testcase.TestCase{}
	for i := range cases {
		tc := &cases[i]
		if tc.EndpointID != nil {
			caseByEndpoint[*tc.EndpointID] = tc
		}
		key := strings.ToUpper(tc.Method) + " " + tc.Path
		if _, ok := caseByKey[key]; !ok {
			caseByKey[key] = tc
		}
	}

	var hints []LoginEndpointHint
	for _, ep := range endpoints {
		if !isLoginEndpoint(ep) {
			continue
		}
		role := "user"
		if strings.Contains(strings.ToLower(ep.Path), "/admin/") {
			role = "admin"
		}
		tc := caseByEndpoint[ep.ID]
		if tc == nil {
			tc = caseByKey[strings.ToUpper(ep.Method)+" "+ep.Path]
		}
		if tc != nil {
			if user, pass, ok := ExtractCredentialsFromRequest(tc.Request); ok {
				hints = append(hints, LoginEndpointHint{
					EndpointID:   ep.ID,
					CaseID:       tc.ID,
					Method:       ep.Method,
					Path:         ep.Path,
					Role:         role,
					Source:       HintSourceSavedCase,
					Username:     user,
					PasswordHint: maskPassword(pass),
					BodyFields:   loginBodyFieldHints(ep, tc.Request, nil),
				})
				continue
			}
			hints = append(hints, LoginEndpointHint{
				EndpointID: ep.ID,
				CaseID:     tc.ID,
				Method:     ep.Method,
				Path:       ep.Path,
				Role:       role,
				Source:     HintSourceSavedCase,
				BodyFields: loginBodyFieldHints(ep, tc.Request, nil),
			})
			continue
		}
		hints = append(hints, LoginEndpointHint{
			EndpointID: ep.ID,
			Method:     ep.Method,
			Path:       ep.Path,
			Role:       role,
			BodyFields: loginBodyFieldHints(ep, nil, nil),
		})
	}

	if envReader != nil && environmentID != uuid.Nil {
		env, err := envReader.GetServiceEnvironment(ctx, projectID, serviceID, environmentID)
		if err == nil && env != nil {
			hints = append(hints, authProfileHints(env.Auth)...)
			hints = append(hints, environmentVariableHints(env.Variables)...)
		}
	}

	return &LoginHintsResponse{
		Hints: hints,
		Guidance: "登录凭据不应写死在平台配置中。生成场景时**不要**在对话里逐条追问用户；" +
			"应直接调用 generate_coverage_scenarios / generate_and_verify_scenarios 挂起确认卡片，由用户在卡片内填写 loginCredentials（含 body 扩展字段如 code/union_id）。" +
			"优先复用 saved_case 中的已有模板；缺失字段在确认面板填写，或在生成后用 update_scenario_step 更新登录步 requestOverride。禁止 AI 自行编造 __FILL_* 占位符。",
	}, nil
}

func loginBodyFieldHints(ep spec.Endpoint, savedCaseRaw json.RawMessage, creds *LoginCredentialBundle) []LoginBodyFieldHint {
	admin := strings.Contains(strings.ToLower(ep.Path), "/admin/")
	_, pending, _ := BuildLoginRequestBody(ep, savedCaseRaw, creds, admin)
	if len(pending) > 0 {
		return pending
	}
	fields := ParseLoginBodyFields(ep.RequestSchema)
	if len(fields) == 0 {
		return nil
	}
	body := extractRequestBody(savedCaseRaw)
	out := make([]LoginBodyFieldHint, 0, len(fields))
	for _, f := range fields {
		h := f
		if body != nil {
			if v, ok := body[f.Name]; ok && hasConcreteValue(v) {
				h.CurrentValue = v
				if f.Sensitive {
					h.CurrentValue = maskAnySecret(v)
				}
			}
		}
		out = append(out, h)
	}
	return out
}

func authProfileHints(raw json.RawMessage) []LoginEndpointHint {
	if len(raw) == 0 {
		return nil
	}
	var cfg struct {
		Profiles map[string]struct {
			Type     string `json:"type"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil || len(cfg.Profiles) == 0 {
		var legacy struct {
			Type     string `json:"type"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if json.Unmarshal(raw, &legacy) != nil {
			return nil
		}
		if legacy.Username == "" && legacy.Password == "" {
			return nil
		}
		role := "user"
		if strings.Contains(strings.ToLower(legacy.Type), "admin") {
			role = "admin"
		}
		return []LoginEndpointHint{{
			Role:         role,
			Source:       HintSourceEnvironmentAuth,
			Username:     strings.TrimSpace(legacy.Username),
			PasswordHint: maskPassword(legacy.Password),
		}}
	}
	var out []LoginEndpointHint
	for name, p := range cfg.Profiles {
		user := strings.TrimSpace(p.Username)
		pass := strings.TrimSpace(p.Password)
		if user == "" && pass == "" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(p.Type)) != "basic" && user == "" {
			continue
		}
		role := "user"
		lower := strings.ToLower(name + " " + user)
		if strings.Contains(lower, "admin") {
			role = "admin"
		}
		out = append(out, LoginEndpointHint{
			Role:         role,
			Source:       HintSourceEnvironmentAuth,
			ProfileName:  name,
			Username:     user,
			PasswordHint: maskPassword(pass),
		})
	}
	return out
}

func environmentVariableHints(raw json.RawMessage) []LoginEndpointHint {
	if len(raw) == 0 {
		return nil
	}
	var vars map[string]any
	if err := json.Unmarshal(raw, &vars); err != nil {
		return nil
	}
	var out []LoginEndpointHint
	for k, v := range vars {
		upper := strings.ToUpper(k)
		if !strings.Contains(upper, "USER") && !strings.Contains(upper, "LOGIN") && !strings.Contains(upper, "PASS") {
			continue
		}
		val := strings.TrimSpace(fmt.Sprint(v))
		if val == "" {
			continue
		}
		role := "user"
		if strings.Contains(upper, "ADMIN") {
			role = "admin"
		}
		h := LoginEndpointHint{
			Role:         role,
			Source:       HintSourceEnvironmentVar,
			VariableName: k,
		}
		if strings.Contains(upper, "PASS") {
			h.PasswordHint = maskPassword(val)
		} else {
			h.Username = val
		}
		out = append(out, h)
	}
	return out
}
