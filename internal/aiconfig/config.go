package aiconfig

import (
	"os"
	"strconv"
	"strings"
)

const (
	DefaultMaxToolHops                 = 6
	DefaultScenarioGenMaxRounds        = 3
	DefaultRouterConfidenceThreshold   = 0.7
	DefaultToolEmbedModel              = "text-embedding-3-small"
	DefaultToolRoutingMode             = "shadow"
)

// Config holds platform-wide AI assistant runtime settings.
type Config struct {
	MaxToolHops                 int     `json:"maxToolHops"`
	ToolRoutingMode             string  `json:"toolRoutingMode"`
	ScenarioAutorunEnabled      bool    `json:"scenarioAutorunEnabled"`
	ScenarioGenDefaultMaxRounds int     `json:"scenarioGenDefaultMaxRounds"`
	RouterConfidenceThreshold   float64 `json:"routerConfidenceThreshold"`
	ToolEmbedBaseURL            string  `json:"toolEmbedBaseUrl"`
	ToolEmbedAPIKey             string  `json:"-"`
	ToolEmbedModel              string  `json:"toolEmbedModel"`
}

// FieldMeta describes one configurable parameter for the admin UI.
type FieldMeta struct {
	Key          string        `json:"key"`
	Label        string        `json:"label"`
	Description  string        `json:"description"`
	Type         string        `json:"type"`
	DefaultValue any           `json:"defaultValue,omitempty"`
	Options      []FieldOption `json:"options,omitempty"`
	Min          *float64      `json:"min,omitempty"`
	Max          *float64      `json:"max,omitempty"`
}

// FieldOption is an enum choice for select fields.
type FieldOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SettingsView is the API response for GET /ai-assistant-settings.
type SettingsView struct {
	Settings SettingsPayload `json:"settings"`
	Fields   []FieldMeta     `json:"fields"`
	UpdatedAt string         `json:"updatedAt,omitempty"`
}

// SettingsPayload is the editable subset exposed to clients (secrets masked).
type SettingsPayload struct {
	MaxToolHops                 int     `json:"maxToolHops"`
	ToolRoutingMode             string  `json:"toolRoutingMode"`
	ScenarioAutorunEnabled      bool    `json:"scenarioAutorunEnabled"`
	ScenarioGenDefaultMaxRounds int     `json:"scenarioGenDefaultMaxRounds"`
	RouterConfidenceThreshold   float64 `json:"routerConfidenceThreshold"`
	ToolEmbedBaseURL            string  `json:"toolEmbedBaseUrl"`
	ToolEmbedAPIKeyMasked       string  `json:"toolEmbedApiKeyMasked"`
	ToolEmbedAPIKeyPresent      bool    `json:"toolEmbedApiKeyPresent"`
	ToolEmbedModel              string  `json:"toolEmbedModel"`
}

// UpdateInput is the payload for PUT /ai-assistant-settings.
// Secret fields: empty string keeps the previous value; "__clear__" wipes it.
type UpdateInput struct {
	MaxToolHops                 *int     `json:"maxToolHops"`
	ToolRoutingMode             *string  `json:"toolRoutingMode"`
	ScenarioAutorunEnabled      *bool    `json:"scenarioAutorunEnabled"`
	ScenarioGenDefaultMaxRounds *int     `json:"scenarioGenDefaultMaxRounds"`
	RouterConfidenceThreshold   *float64 `json:"routerConfidenceThreshold"`
	ToolEmbedBaseURL            *string  `json:"toolEmbedBaseUrl"`
	ToolEmbedAPIKey             *string  `json:"toolEmbedApiKey"`
	ToolEmbedModel              *string  `json:"toolEmbedModel"`
}

// storedConfig is the JSON shape persisted in ai_assistant_settings.config.
type storedConfig struct {
	MaxToolHops                 *int     `json:"maxToolHops,omitempty"`
	ToolRoutingMode             *string  `json:"toolRoutingMode,omitempty"`
	ScenarioAutorunEnabled      *bool    `json:"scenarioAutorunEnabled,omitempty"`
	ScenarioGenDefaultMaxRounds *int     `json:"scenarioGenDefaultMaxRounds,omitempty"`
	RouterConfidenceThreshold   *float64 `json:"routerConfidenceThreshold,omitempty"`
	ToolEmbedBaseURL            *string  `json:"toolEmbedBaseUrl,omitempty"`
	ToolEmbedAPIKey             *string  `json:"toolEmbedApiKey,omitempty"`
	ToolEmbedModel              *string  `json:"toolEmbedModel,omitempty"`
}

// DefaultConfig returns code defaults without env or DB overrides.
func DefaultConfig() Config {
	return Config{
		MaxToolHops:                 DefaultMaxToolHops,
		ToolRoutingMode:             DefaultToolRoutingMode,
		ScenarioAutorunEnabled:      false,
		ScenarioGenDefaultMaxRounds: DefaultScenarioGenMaxRounds,
		RouterConfidenceThreshold:   DefaultRouterConfidenceThreshold,
		ToolEmbedModel:              DefaultToolEmbedModel,
	}
}

// MergeEnv overlays environment variables onto cfg (env wins over defaults).
func MergeEnv(cfg Config) Config {
	if v := envInt("AI_MAX_TOOL_HOPS"); v > 0 {
		cfg.MaxToolHops = v
	}
	if v := strings.TrimSpace(os.Getenv("AI_TOOL_ROUTING_MODE")); v != "" {
		cfg.ToolRoutingMode = normalizeRoutingMode(v)
	}
	if envBool("AI_SCENARIO_AUTORUN") {
		cfg.ScenarioAutorunEnabled = true
	}
	if v := envInt("AI_SCENARIO_GEN_MAX_ROUNDS"); v > 0 {
		cfg.ScenarioGenDefaultMaxRounds = v
	}
	if v := envFloat("AI_ROUTER_CONFIDENCE_THRESHOLD"); v > 0 {
		cfg.RouterConfidenceThreshold = v
	}
	if v := strings.TrimSpace(os.Getenv("AI_TOOL_EMBED_BASE_URL")); v != "" {
		cfg.ToolEmbedBaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("AI_TOOL_EMBED_API_KEY")); v != "" {
		cfg.ToolEmbedAPIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("AI_TOOL_EMBED_MODEL")); v != "" {
		cfg.ToolEmbedModel = v
	}
	return cfg
}

// ApplyStored overlays persisted JSON values onto cfg (DB wins over env/defaults).
func ApplyStored(cfg Config, raw storedConfig) Config {
	if raw.MaxToolHops != nil && *raw.MaxToolHops > 0 {
		cfg.MaxToolHops = *raw.MaxToolHops
	}
	if raw.ToolRoutingMode != nil {
		cfg.ToolRoutingMode = normalizeRoutingMode(*raw.ToolRoutingMode)
	}
	if raw.ScenarioAutorunEnabled != nil {
		cfg.ScenarioAutorunEnabled = *raw.ScenarioAutorunEnabled
	}
	if raw.ScenarioGenDefaultMaxRounds != nil && *raw.ScenarioGenDefaultMaxRounds > 0 {
		cfg.ScenarioGenDefaultMaxRounds = *raw.ScenarioGenDefaultMaxRounds
	}
	if raw.RouterConfidenceThreshold != nil && *raw.RouterConfidenceThreshold > 0 {
		cfg.RouterConfidenceThreshold = *raw.RouterConfidenceThreshold
	}
	if raw.ToolEmbedBaseURL != nil {
		cfg.ToolEmbedBaseURL = strings.TrimSpace(*raw.ToolEmbedBaseURL)
	}
	if raw.ToolEmbedAPIKey != nil {
		cfg.ToolEmbedAPIKey = strings.TrimSpace(*raw.ToolEmbedAPIKey)
	}
	if raw.ToolEmbedModel != nil && strings.TrimSpace(*raw.ToolEmbedModel) != "" {
		cfg.ToolEmbedModel = strings.TrimSpace(*raw.ToolEmbedModel)
	}
	return cfg
}

func (c Config) ToStored() storedConfig {
	out := storedConfig{
		MaxToolHops:                 intPtr(c.MaxToolHops),
		ToolRoutingMode:             strPtr(c.ToolRoutingMode),
		ScenarioAutorunEnabled:      boolPtr(c.ScenarioAutorunEnabled),
		ScenarioGenDefaultMaxRounds: intPtr(c.ScenarioGenDefaultMaxRounds),
		RouterConfidenceThreshold:   floatPtr(c.RouterConfidenceThreshold),
		ToolEmbedBaseURL:            strPtr(c.ToolEmbedBaseURL),
		ToolEmbedModel:              strPtr(c.ToolEmbedModel),
	}
	if c.ToolEmbedAPIKey != "" {
		out.ToolEmbedAPIKey = strPtr(c.ToolEmbedAPIKey)
	}
	return out
}

func (c Config) ToPayload() SettingsPayload {
	return SettingsPayload{
		MaxToolHops:                      c.MaxToolHops,
		ToolRoutingMode:                  c.ToolRoutingMode,
		ScenarioAutorunEnabled:           c.ScenarioAutorunEnabled,
		ScenarioGenDefaultMaxRounds:      c.ScenarioGenDefaultMaxRounds,
		RouterConfidenceThreshold:        c.RouterConfidenceThreshold,
		ToolEmbedBaseURL:                 c.ToolEmbedBaseURL,
		ToolEmbedAPIKeyMasked:            MaskSecret(c.ToolEmbedAPIKey),
		ToolEmbedAPIKeyPresent:           c.ToolEmbedAPIKey != "",
		ToolEmbedModel:                   c.ToolEmbedModel,
	}
}

// FieldDefinitions returns metadata for the admin settings form.
func FieldDefinitions() []FieldMeta {
	minHops, maxHops := 1.0, 20.0
	minRounds, maxRounds := 1.0, 10.0
	minConf, maxConf := 0.1, 1.0
	return []FieldMeta{
		{
			Key: "maxToolHops", Label: "工具调用最大轮次", Type: "int",
			Description:  "单次 AI 对话中，模型与内置工具之间最多往返几轮。达到上限后停止工具调用并提示用户缩小问题范围。分析类入口同样受此限制。",
			DefaultValue: DefaultMaxToolHops, Min: &minHops, Max: &maxHops,
		},
		{
			Key: "toolRoutingMode", Label: "按需工具路由模式", Type: "enum",
			Description: "控制 AI 助理是否收窄每轮发给大模型的工具列表。无论取值如何，Planner/Router 都会计算并写入路由日志，便于离线对比。",
			DefaultValue: DefaultToolRoutingMode,
			Options: []FieldOption{
				{Value: "shadow", Label: "shadow（默认）— 始终发全量工具，仅影子记录"},
				{Value: "dynamic_fallback", Label: "dynamic_fallback — 高置信度时收窄，否则回退全量"},
				{Value: "dynamic", Label: "dynamic — 默认收窄，无可用域工具时回退全量"},
				{Value: "meta_only", Label: "meta_only — 仅核心发现 + meta 工具，其余靠 find/describe"},
			},
		},
		{
			Key: "routerConfidenceThreshold", Label: "路由置信度阈值", Type: "float",
			Description:  "Router 置信度低于此值时，在 dynamic_fallback 模式下回退为全量工具，避免误收窄导致模型找不到工具。",
			DefaultValue: DefaultRouterConfidenceThreshold, Min: &minConf, Max: &maxConf,
		},
		{
			Key: "scenarioAutorunEnabled", Label: "场景生成真实环境验证", Type: "bool",
			Description: "开启后，AI 助理可调用 generate_and_verify_scenarios，在指定真实环境上异步执行「生成→验证→（可选）自动修复」闭环。关闭时该工具在运行时拒绝执行。",
			DefaultValue: false,
		},
		{
			Key: "scenarioGenDefaultMaxRounds", Label: "场景闭环默认最大轮次", Type: "int",
			Description:  "generate_and_verify_scenarios 未指定 maxRounds 时使用的默认执行-修复轮次上限。",
			DefaultValue: DefaultScenarioGenMaxRounds, Min: &minRounds, Max: &maxRounds,
		},
		{
			Key: "toolEmbedBaseUrl", Label: "工具向量 Base URL", Type: "string",
			Description: "find_tools 查询向量的 OpenAI 兼容 /embeddings 端点 Base URL（…/v1）。留空时尝试复用对话 Provider；Anthropic 对话需单独配置。",
		},
		{
			Key: "toolEmbedApiKey", Label: "工具向量 API Key", Type: "password",
			Description: "与工具向量 Base URL 配套的 API Key。留空保存则保持原值不变。",
		},
		{
			Key: "toolEmbedModel", Label: "工具向量模型", Type: "string",
			Description:  "工具目录与查询 embedding 模型，须与 make gen-tool-embeddings 生成文件中的 model 一致。",
			DefaultValue: DefaultToolEmbedModel,
		},
	}
}

func normalizeRoutingMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "dynamic_fallback", "dynamic", "meta_only", "shadow", "":
		if strings.TrimSpace(s) == "" {
			return DefaultToolRoutingMode
		}
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return DefaultToolRoutingMode
	}
}

func validRoutingMode(s string) bool {
	switch s {
	case "shadow", "dynamic_fallback", "dynamic", "meta_only", "":
		return true
	default:
		return false
	}
}

// MaxToolHops returns the effective tool-calling hop limit.
func MaxToolHops() int {
	v := Get().MaxToolHops
	if v <= 0 {
		return DefaultMaxToolHops
	}
	return v
}

// RouterConfidenceThreshold returns the effective router confidence cutoff.
func RouterConfidenceThreshold() float64 {
	v := Get().RouterConfidenceThreshold
	if v <= 0 {
		return DefaultRouterConfidenceThreshold
	}
	return v
}

// ScenarioAutorunEnabled reports whether real-environment scenario verify is allowed.
func ScenarioAutorunEnabled() bool {
	return Get().ScenarioAutorunEnabled
}

// ScenarioGenDefaultMaxRounds returns the default max rounds for genagent jobs.
func ScenarioGenDefaultMaxRounds() int {
	v := Get().ScenarioGenDefaultMaxRounds
	if v <= 0 {
		return DefaultScenarioGenMaxRounds
	}
	return v
}

// ToolEmbedSettings returns embedding endpoint configuration.
func ToolEmbedSettings() (baseURL, apiKey, model string) {
	cfg := Get()
	return cfg.ToolEmbedBaseURL, cfg.ToolEmbedAPIKey, cfg.ToolEmbedModel
}

func MaskSecret(v string) string {
	if v == "" {
		return ""
	}
	runes := []rune(v)
	if len(runes) <= 4 {
		return "••••"
	}
	if len(runes) <= 10 {
		return "••••" + string(runes[len(runes)-2:])
	}
	return string(runes[:3]) + "••••" + string(runes[len(runes)-4:])
}

func envBool(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return v == "on" || v == "1" || v == "true"
}

func envInt(key string) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func envFloat(key string) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool        { return &v }
func strPtr(v string) *string     { return &v }
