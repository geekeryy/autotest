package aiskill

import (
	"strings"
	"text/template"
	"bytes"
	"encoding/json"
)

// Registry manages skill matching and discovery.
type Registry struct {
	skills []*Skill
}

// NewRegistry creates a new skill registry.
func NewRegistry(skills []*Skill) *Registry {
	return &Registry{
		skills: skills,
	}
}

// Match finds skills that match the given input and page context.
func (r *Registry) Match(input string, pageContext json.RawMessage) []*Skill {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" && len(pageContext) == 0 {
		return nil
	}

	var matched []*Skill
	for _, skill := range r.skills {
		if !skill.Enabled {
			continue
		}
		if r.matchSkill(skill, input, pageContext) {
			matched = append(matched, skill)
		}
	}
	return matched
}

// matchSkill checks if a skill matches the input and page context.
func (r *Registry) matchSkill(skill *Skill, input string, pageContext json.RawMessage) bool {
	for _, trigger := range skill.Triggers {
		switch trigger.Type {
		case "keyword":
			if r.matchKeyword(trigger.Pattern, input) {
				return true
			}
		case "page":
			if r.matchPageContext(trigger.Pattern, pageContext) {
				return true
			}
		case "intent":
			if r.matchIntent(trigger.Pattern, input) {
				return true
			}
		}
	}
	return false
}

// matchKeyword checks if the input contains the keyword pattern.
func (r *Registry) matchKeyword(pattern, input string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	return strings.Contains(input, pattern)
}

// matchPageContext checks if the page context matches the pattern.
func (r *Registry) matchPageContext(pattern string, pageContext json.RawMessage) bool {
	if len(pageContext) == 0 {
		return false
	}
	// Simple string contains check on the JSON
	return strings.Contains(string(pageContext), pattern)
}

// matchIntent checks if the input matches an intent pattern.
func (r *Registry) matchIntent(pattern, input string) bool {
	// Simple keyword-based intent matching
	keywords := strings.Split(pattern, "|")
	for _, kw := range keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if strings.Contains(input, kw) {
			return true
		}
	}
	return false
}

// GetByName returns a skill by name.
func (r *Registry) GetByName(name string) *Skill {
	for _, skill := range r.skills {
		if skill.Name == name {
			return skill
		}
	}
	return nil
}

// ExpandPrompt expands a skill's prompt template with the given variables.
func ExpandPrompt(skill *Skill, vars map[string]any) (string, error) {
	t, err := template.New("skill").Parse(skill.PromptTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// PresetSkills returns the built-in skill definitions.
func PresetSkills() []PresetSkill {
	return []PresetSkill{
		{
			Name:        "api_coverage_gen",
			Label:       "API 覆盖测试生成",
			Description: "基于 OpenAPI spec 自动生成覆盖全部 endpoint 的测试场景",
			PromptTemplate: `请为当前服务的所有 API endpoint 生成完整的测试场景覆盖。
要求:
1. 每个 endpoint 至少一个正向用例
2. POST/PUT/PATCH 额外生成参数校验反向用例
3. 有依赖关系的 endpoint 按依赖顺序编排
{{if .EnvironmentID}}在环境 {{.EnvironmentID}} 上验证。{{end}}`,
			ToolDomains: []string{"meta", "cases", "scenarios", "spec"},
			Triggers: []SkillTrigger{
				{Type: "keyword", Pattern: "覆盖"},
				{Type: "keyword", Pattern: "全部"},
				{Type: "intent", Pattern: "生成测试|覆盖测试|全量测试"},
			},
		},
		{
			Name:        "failure_diagnosis",
			Label:       "测试失败诊断",
			Description: "分析最近一次运行失败的根因并给出修复建议",
			PromptTemplate: `请分析最近一次运行 (ID: {{.RunID}}) 的失败原因。
重点关注:
1. 断言失败的具体字段
2. HTTP 状态码异常
3. 依赖步骤的数据传递问题`,
			ToolDomains: []string{"meta", "cases", "scenarios", "runs"},
			Triggers: []SkillTrigger{
				{Type: "keyword", Pattern: "失败"},
				{Type: "keyword", Pattern: "诊断"},
				{Type: "intent", Pattern: "失败分析|错误分析|问题排查"},
			},
		},
		{
			Name:        "mock_data_gen",
			Label:       "Mock 数据生成",
			Description: "为接口生成 Mock 数据和规则",
			PromptTemplate: `请为以下接口生成 Mock 数据:
- 接口: {{.Method}} {{.Path}}
- 描述: {{.Description}}

要求:
1. 生成符合接口 schema 的 Mock 响应数据
2. 包含常见的边界值和异常场景
3. 支持动态数据生成（使用 {{$mock.*}} 模板）`,
			ToolDomains: []string{"mock", "mockset", "cases"},
			Triggers: []SkillTrigger{
				{Type: "keyword", Pattern: "mock"},
				{Type: "keyword", Pattern: "模拟数据"},
				{Type: "intent", Pattern: "生成mock|创建mock|模拟数据"},
			},
		},
		{
			Name:        "assertion_optimize",
			Label:       "断言优化建议",
			Description: "分析现有断言并提供优化建议",
			PromptTemplate: `请分析以下接口模板的断言并提供优化建议:
- 模板 ID: {{.CaseID}}
- 当前断言数量: {{.AssertionCount}}

重点关注:
1. 断言覆盖度（是否遗漏关键字段）
2. 断言精确度（是否过于宽松或严格）
3. 性能影响（是否有不必要的断言）`,
			ToolDomains: []string{"cases", "meta"},
			Triggers: []SkillTrigger{
				{Type: "keyword", Pattern: "断言"},
				{Type: "keyword", Pattern: "优化"},
				{Type: "intent", Pattern: "断言优化|优化断言|改进断言"},
			},
		},
		{
			Name:        "spec_diff_analysis",
			Label:       "Spec 变更分析",
			Description: "分析 OpenAPI spec 变更对现有测试资产的影响",
			PromptTemplate: `请分析以下 OpenAPI spec 变更的影响:
- 服务: {{.ServiceName}}
- 变更类型: {{.ChangeType}}

重点分析:
1. 新增/删除/修改的 endpoint
2. 对现有接口模板的影响
3. 对现有场景步骤的影响
4. 建议的修复动作`,
			ToolDomains: []string{"spec", "cases", "scenarios", "meta"},
			Triggers: []SkillTrigger{
				{Type: "keyword", Pattern: "spec"},
				{Type: "keyword", Pattern: "变更"},
				{Type: "intent", Pattern: "spec分析|变更分析|接口变更"},
			},
		},
		{
			Name:        "smart_scenario_gen",
			Label:       "智能场景生成",
			Description: "基于 AI 语义分析接口业务关系，识别隐式依赖，按业务流程编排测试场景",
			PromptTemplate: `请对当前服务的所有 API endpoint 进行语义分析，并生成按业务流程编排的测试场景。

要求：
1. 使用 list_endpoints 获取全部接口清单
2. 对 summary 语义不明确的接口调用 get_endpoint 获取完整 schema
3. 语义分析每个接口的角色（鉴权/创建/读取/更新/删除/查询/业务操作）
4. 识别接口间的隐式依赖（从 summary、tags、path 结构、schema 字段语义推断）
5. 按业务流程分组场景，步骤按依赖拓扑排序
6. 使用 create_scenario_with_steps 创建场景，用 {{$steps[N]}} 模板变量引用前置步骤响应
{{if .ServiceID}}目标服务：{{.ServiceID}}{{end}}`,
			ToolDomains: []string{"meta", "cases", "scenarios", "spec", "paramsource"},
			Triggers: []SkillTrigger{
				{Type: "keyword", Pattern: "智能场景"},
				{Type: "keyword", Pattern: "语义分析"},
				{Type: "intent", Pattern: "智能生成|语义分析场景|隐式依赖|业务流程场景"},
			},
		},
	}
}
