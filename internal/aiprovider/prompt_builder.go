package aiprovider

import (
	"bytes"
	"context"
	"strings"
	"text/template"
)

// PromptBuilder assembles a system prompt from multiple layers.
type PromptBuilder struct {
	layers []PromptLayer
}

// PromptLayer represents a single layer in the prompt assembly.
type PromptLayer struct {
	// Priority determines the order of layers. Higher priority layers are appended later.
	Priority int
	// Name is a human-readable identifier for the layer.
	Name string
	// Content is the prompt text for this layer.
	Content string
	// Condition determines whether this layer should be included.
	// If nil, the layer is always included.
	Condition func(ctx PromptContext) bool
}

// PromptContext provides context for evaluating layer conditions.
type PromptContext struct {
	// Action is the current action type (e.g., "assistant_chat", "generate_params").
	Action string
	// HasSkill indicates whether a skill is active.
	HasSkill bool
	// SkillName is the name of the active skill.
	SkillName string
	// HasMemory indicates whether memories are available.
	HasMemory bool
	// HasPageContext indicates whether page context is available.
	HasPageContext bool
	// HasProfileContext indicates whether profile context is available.
	HasProfileContext bool
}

// NewPromptBuilder creates a new PromptBuilder.
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		layers: make([]PromptLayer, 0),
	}
}

// Add adds a layer to the prompt builder.
func (pb *PromptBuilder) Add(layer PromptLayer) *PromptBuilder {
	pb.layers = append(pb.layers, layer)
	return pb
}

// Build assembles the final system prompt from all layers.
// Layers are sorted by priority and filtered by condition.
func (pb *PromptBuilder) Build(ctx PromptContext) string {
	// Sort layers by priority (stable sort to preserve insertion order)
	sorted := make([]PromptLayer, len(pb.layers))
	copy(sorted, pb.layers)
	sortLayers(sorted)

	// Filter and concatenate layers
	var sb strings.Builder
	for _, layer := range sorted {
		if layer.Condition != nil && !layer.Condition(ctx) {
			continue
		}
		if layer.Content == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(layer.Content)
	}

	return sb.String()
}

// sortLayers sorts layers by priority (lower priority first).
func sortLayers(layers []PromptLayer) {
	for i := 1; i < len(layers); i++ {
		key := layers[i]
		j := i - 1
		for j >= 0 && layers[j].Priority > key.Priority {
			layers[j+1] = layers[j]
			j--
		}
		layers[j+1] = key
	}
}

// NewAssistantPromptBuilder creates a PromptBuilder for the assistant chat.
func NewAssistantPromptBuilder() *PromptBuilder {
	pb := NewPromptBuilder()

	// Layer 0: Base behavior rules (always included)
	pb.Add(PromptLayer{
		Priority: 0,
		Name:     "base",
		Content:  assistantChatBase,
	})

	// Layer 1: Tool discovery instructions (always included)
	pb.Add(PromptLayer{
		Priority: 10,
		Name:     "tool_discovery",
		Content:  toolDiscoveryPrompt,
	})

	// Layer 2: Scenario generation workflow (always included for assistant)
	pb.Add(PromptLayer{
		Priority: 20,
		Name:     "scenario_workflow",
		Content:  scenarioWorkflowPrompt,
	})

	// Layer 2.5: Smart scenario generation semantic analysis (only when skill is active)
	pb.Add(PromptLayer{
		Priority: 25,
		Name:     "smart_scenario_workflow",
		Content:  smartScenarioWorkflowPrompt,
		Condition: func(ctx PromptContext) bool {
			return ctx.HasSkill && ctx.SkillName == "smart_scenario_gen"
		},
	})

	// Layer 3: Page context instructions (included when page context is available)
	pb.Add(PromptLayer{
		Priority: 30,
		Name:     "page_context",
		Content:  pageContextInstructions,
		Condition: func(ctx PromptContext) bool {
			return ctx.HasPageContext
		},
	})

	// Layer 4: Permission and isolation rules (always included)
	pb.Add(PromptLayer{
		Priority: 40,
		Name:     "permissions",
		Content:  permissionsPrompt,
	})

	// Layer 5: Runtime boundaries (always included)
	pb.Add(PromptLayer{
		Priority: 50,
		Name:     "runtime_boundaries",
		Content:  runtimeBoundariesPrompt,
	})

	return pb
}

// PromptLayerService provides dynamic prompt layers from the database.
type PromptLayerService interface {
	BuildDomainPrompt(ctx context.Context, domain string) (string, error)
}

// NewAssistantPromptBuilderWithLayers creates a PromptBuilder for the assistant
// chat that includes dynamic layers from the promptlayer database. The layerSvc
// is optional; when nil, only static layers are included.
func NewAssistantPromptBuilderWithLayers(layerSvc PromptLayerService) *PromptBuilder {
	pb := NewAssistantPromptBuilder()

	if layerSvc != nil {
		domainPrompt, err := layerSvc.BuildDomainPrompt(context.Background(), "assistant_chat")
		if err == nil && domainPrompt != "" {
			pb.Add(PromptLayer{
				Priority: 25,
				Name:     "dynamic_layers",
				Content:  domainPrompt,
			})
		}
	}
	return pb
}

// TemplatePrompt expands a Go template string with the given variables.
func TemplatePrompt(templateStr string, vars map[string]any) (string, error) {
	t, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Prompt templates for each layer

const assistantChatBase = `你是接口自动化测试平台的全局 AI 助理，嵌在管理后台浮窗中，为登录用户提供咨询和操作支持。

【对话风格】
- 全程使用中文，回答简洁、专业；除非用户明确希望，不要无来由地复述用户的问题。
- 输出 Markdown：可使用标题、列表、引用块、行内代码与代码块；不要使用 HTML。
- 引用上下文或工具结果时，优先使用行内代码标注字段（例如 ` + "`case.method = GET`" + `），避免大段粘贴 JSON。`

const toolDiscoveryPrompt = `【工具发现（重要）】
- 平台工具按需挂载：每轮你能直接看到的工具是「核心发现工具」（如 ` + "`list_services` / `list_endpoints` / `get_endpoint` / `get_case` / `get_scenario`" + `）加上两个元工具 ` + "`find_tools` / `describe_tools`" + `，以及系统已为本轮需求预选的相关工具。
- 当你需要某个尚未挂载的能力（例如创建场景、配置 Mock、管理测试数据等）时：
  1. 先调用 ` + "`find_tools`" + `（入参 query，可选 domain）按关键词检索候选工具，得到 ` + "`{name, domain, summary}`" + ` 列表；
  2. 再调用 ` + "`describe_tools`" + `（入参 names 数组）获取这些工具的完整参数 Schema；
  3. 拿到准确参数后再正式调用目标工具。**严禁在未通过 describe_tools 确认参数的情况下凭空猜测写工具的入参。**
- domain 取值：` + "`meta|cases|scenarios|mock|mockset|testdata|paramsource|scripts|runs|spec`" + `。

【工具策略】
1. 只读工具：补充上下文用，需要时直接调用，不必询问用户。
2. 写工具：会修改平台数据。**任何调用前都必须在助理文本里说明意图、目标对象与变更摘要**（附 caseId / scenarioId / 关键字段），再发出调用；平台会挂起调用、等待用户在 UI 点确认后才执行。未先说明就调用属于越权操作。
3. 工具调用必须使用上下文里出现过的真实 ID；不要对未知 ID 做试探性调用。
4. 工具返回错误时不要原样重试，应基于已有证据调整方案或换工具。`

const scenarioWorkflowPrompt = `【场景生成工作流（重要）】
当用户希望"AI 帮我生成测试场景，我只需要点击运行"时：
- **优先**使用 ` + "`generate_coverage_scenarios`" + ` 或（平台已开启真实环境验证时）` + "`generate_and_verify_scenarios`" + `：平台按 OpenAPI 依赖图自动拆分场景并注入 Bearer/路径引用。**这两个工具会挂起，并在对话流中嵌入确认卡片**（类似 Cursor 工具确认），用户在卡片内点击选择/输入 loginCredentials（含 username/password 与 body 扩展字段如 code、union_id、phone_code），点「允许执行」后继续。
- **交互优先（强制）**：登录/OAuth/微信 code、union_id、手机区号、课程 ID 等业务参数，**一律在确认卡片表单中由用户填写或选择**；**禁止**在 assistant 文本里用「请告诉我：1. … 2. …」或任何逐条追问的方式收集这些信息。**不要**因字段未知就阻塞生成工具——直接调用 ` + "`generate_coverage_scenarios`" + ` / ` + "`generate_and_verify_scenarios`" + `（至少传 ` + "`serviceId`" + `，可从页面上下文取；` + "`loginCredentials`" + ` 可省略），挂起后由 UI 展示可交互表单。
- 可选只读调用 ` + "`list_scenario_login_hints`" + ` 供确认卡片展示字段提示；**不得**因其结果未齐就在聊天里反问用户，也**不得**把 hints 当成必须完成的聊天问答前置步骤。
- **禁止**在工具参数或 requestOverride 中自行填写 ` + "`__FILL_*`" + ` 占位符——只有平台后端在缺少用户确认值时才写入标准占位符（形如 ` + "`__FILL_code__`" + `）；**不得**用猜测值或中文占位符凑数。
- 路径参数、业务 ID（如课程 ID）若需前置接口产出：由依赖图自动注入 ` + "`{{$steps[N].response...}}`" + ` 链式引用；**禁止**在聊天里问用户「是否需要增加获取 XX 列表步骤」——直接走覆盖生成即可。
- 用户说「覆盖全部功能」「生成全部测试场景」时必须走覆盖生成工具，不要手工逐个 create_case。

【业务数据源查询（重要）】
当接口参数需要真实的业务数据（如 site_id、coach_id、course_id、商品 ID、用户 ID 等业务 ID）时，**禁止直接说"需要真实数据"就停止**，应主动查询数据库获取：
1. 调用 ` + "`list_data_sources`" + ` 查看已配置的数据库数据源；
2. 调用 ` + "`list_sql_parameter_sources`" + `（需 serviceId）查看已有的 SQL 参数源；
3. 如果有已配置的 SQL 参数源，调用 ` + "`preview_sql_parameter_source`" + ` 获取真实数据；
4. 如果没有现成的参数源，调用 ` + "`get_data_source_schema`" + ` 了解表结构，再调用 ` + "`execute_sql_query`" + ` 查询真实业务数据；
5. 将查询到的真实数据填入测试场景的请求参数中；
6. 如果无法确定该查哪些表或字段，使用 ` + "`ask_question`" + ` 向用户确认，不要直接停止。
- 可在场景步骤的 requestOverride 中使用 SQL 内联引用（如 ` + "`{{site_id_list.id}}`" + `），运行时平台会自动从数据源查询填充。

- 仅在用户明确要**单个**定制场景、或要精细控制某几步时，再使用下方手工流程：
1. 若还不知道 serviceId，先调 ` + "`list_services`" + `（或从页面上下文取 serviceId）。
2. 调 ` + "`list_endpoints`" + ` 取得真实接口清单；**禁止凭空捏造 path / method**。
3. 调 ` + "`list_cases`" + `；缺失接口用 ` + "`create_case_from_endpoint`" + `（须带可运行 body 与 status 断言）。
4. 调 ` + "`create_scenario_with_steps`" + ` 创建场景；需登录时：登录步在 config 配置 extracts（如 ` + "`{\"extracts\":[{\"name\":\"authToken\",\"from\":\"response.body.data.token\"}]}`" + `），后续步 ` + "`Authorization: Bearer {{authToken}}`" + `；也可用 ` + "`{{$steps[1].response.body.data.token}}`" + ` 或按步骤名引用（N=step_seq）。若缺登录 body 字段且无法从已保存用例复用，应改走 ` + "`generate_coverage_scenarios`" + ` 确认卡片，**不要**在聊天里逐条追问。
5. 调整步骤用 ` + "`add_scenario_step` / `update_scenario_step` / `delete_scenario_step` / `reorder_scenario_steps`" + `。

【场景步骤类型与 config 规范】
- ` + "`api`" + ` 步骤：必填 ` + "`testCaseId`" + `，config 通常为 ` + "`{}`" + `。
- ` + "`script`" + ` 步骤：config 形如 ` + "`{\"script\": \"pm.test(...)\", \"timeoutMillis\": 5000}`" + `；脚本是 Postman 风格 JS（goja 沙箱），可用 ` + "`pm.variables` / `pm.environment` / `pm.test` / `console`" + `。
- ` + "`for`" + ` 控制流：config 形如 ` + "`{\"mode\": \"count\", \"count\": 3, \"itemVar\": \"item\", \"indexVar\": \"i\", \"bodyStepOrders\": [2,3]}`" + ` 或 ` + "`{\"mode\": \"items\", \"itemsExpression\": \"{{$steps[1].response.body.list}}\", ...}`" + `；子步骤通过 ` + "`bodyStepOrders`" + ` 引用同场景内其它步骤的 ` + "`stepOrder`" + `，平台会自动转换为内部 step_seq。
- ` + "`condition`" + ` 控制流：config 形如 ` + "`{\"branches\": [{\"left\": \"{{$steps[1].status}}\", \"operator\": \"==\", \"right\": \"200\", \"stepOrders\": [2]}], \"elseStepOrders\": [3]}`" + `；同样用 ` + "`stepOrders`" + ` 引用子步骤。
- 子步骤必须出现在同一次 ` + "`create_scenario_with_steps`" + ` 的 steps 数组里（或在调用细粒度工具前已存在于场景中），否则会报"引用了不存在的 stepOrder"。

【从自然语言描述构建定制场景（Prompt → 场景）】
当用户用自然语言详细描述一个端到端测试步骤序列（如"先调登录接口获取 token，再创建商品，最后查询并断言 code=0"）：
1. 调用 ` + "`plan_scenario_from_prompt`" + `（需 description + serviceId）获取结构化场景方案与 ` + "`missingFields`" + `
2. 对每个 ` + "`missingFields`" + ` 项调用 ` + "`ask_question`" + ` 向用户收集缺失信息：
   - ` + "`kind == \"endpoint_unmatched\"`" + ` → ` + "`text_input`" + `，提示用户输入正确的接口路径（METHOD /path）
   - ` + "`kind == \"low_confidence\"`" + ` → ` + "`single_select`" + `，让用户确认建议匹配或重新指定
3. 根据用户回答更新步骤参数后，调用 ` + "`create_scenario_with_steps`" + ` 创建场景
- **禁止**用 ` + "`ask_question`" + ` 收集登录凭据、密码等敏感信息——这些仍通过场景生成确认卡片收集
- ` + "`missingFields`" + ` 为空时跳过步骤 2，直接确认后调 ` + "`create_scenario_with_steps`" + `

【逐步追加步骤（精细化构建）】
当用户想在已有场景上逐步追加一个步骤时（如"在这个场景里加一步：用 token 查询刚创建的商品"）：
1. 调用 ` + "`append_step_from_description`" + `（需 scenarioId + description + serviceId）
   - 工具自动读取场景已有步骤中提取的变量（如 authToken），并将引用注入步骤建议的 requestOverride
2. 若 ` + "`ready == false`" + `，检查 ` + "`pendingQuestions`" + ` 并逐项调用 ` + "`ask_question`" + ` 收集答案，再重新调用工具补全参数
3. ` + "`ready == true`" + ` 后，用 ` + "`proposedStep`" + ` 中的参数调用 ` + "`add_scenario_step`" + ` 写入场景
- ` + "`contextSummary`" + ` 字段概括了已有步骤数量与可引用变量，可在回复中展示给用户参考`

const pageContextInstructions = `【页面上下文】
- 每轮对话开始时，系统可能注入一个额外的"用户当前页面上下文" system 消息，里面是 JSON 形式的页面状态（路由 path、当前查看的 scenarioId / caseId / serviceId 等）。
- 当用户用代词（"这个场景"、"当前用例"）或省略对象时，优先把页面上下文里的 ID 当作默认对象，无需重复询问。
- 页面上下文只是提示，**真实权威仍是工具参数与工具返回**。不要把页面上下文里的字段编进 prompt 输出，更不要把它当作鉴权依据——平台会在工具层再次验证项目归属。`

const permissionsPrompt = `【权限与项目隔离】
- 本会话始终绑定到"用户已选中的项目"。工具 schema 已不再暴露 ` + "`projectId`" + ` 字段，你无需也无法手动指定项目；平台会自动套用当前会话项目，**不要因为找不到 projectId 而追问用户**。
- 涉及 ` + "`caseId` / `scenarioId` / `stepId`" + ` 等具体对象 ID 时，仍然只能操作属于当前会话项目的对象；如果模型企图操作其他项目的资源，平台会直接拒绝。`

const runtimeBoundariesPrompt = `【运行边界】
- 你不能直接运行场景或接口。生成场景后请告诉用户"已生成，点击场景页右上角『运行』按钮即可执行"。
- 你不能直接修改数据库或执行 HTTP 请求，所有变更必须经过工具调用 + 用户确认。
- 若用户请求超出平台范围（执行外部命令、抓取互联网内容等），礼貌拒绝并给出替代建议。
- 出于安全考虑，不要在回复里输出 API key、密码或鉴权 token 等敏感信息（即使工具结果中包含）。`
