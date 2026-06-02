// Package builtin assembles the default tool set wired against the
// platform's domain services.
//
// Tools are grouped along two axes:
//
//   - Read-only vs. mutating vs. RequiresConfirm. Read-only tools are also
//     served to smart-analysis. Mutating create/update tools run immediately
//     in the assistant stream; delete_* tools require user confirmation.
//   - Domain. `builtin_meta.go` covers project/service/environment/endpoint
//     introspection. `builtin_cases.go` covers API request templates.
//     `builtin_scenarios.go` covers scenarios and their steps — which is
//     where the AI-driven scenario generation flow lives.
package builtin

import (
	"encoding/json"
	"fmt"

	"autotest/internal/aitools"
)

// ReadOnly returns the read-only tools shared between smart analysis and
// the conversational assistant.
func ReadOnly(deps Deps) []aitools.Tool {
	return []aitools.Tool{
		// meta / discovery
		listServicesTool(deps).WithMeta("meta", "列出当前项目下的全部服务及其ID"),
		listEndpointsTool(deps).WithMeta("meta", "列出某服务下所有接口的摘要"),
		listEnvironmentsTool(deps).WithMeta("meta", "列出项目或服务级的运行环境配置"),
		getEndpointTool(deps).WithMeta("meta", "按method+path查询接口完整定义与Schema"),
		getEnvironmentTool(deps).WithMeta("meta", "查询单个服务级运行环境详情"),
		// specs
		listSpecsTool(deps).WithMeta("spec", "列出服务已导入的OpenAPI规范版本"),
		// cases
		listCasesTool(deps).WithMeta("cases", "列出项目下的接口请求模板摘要"),
		getCaseTool(deps).WithMeta("cases", "查询单个接口模板或用例的详情"),
		// scenarios
		listScenariosTool(deps).WithMeta("scenarios", "列出项目下的全部场景摘要"),
		getScenarioTool(deps).WithMeta("scenarios", "查询场景详情及其所有步骤"),
		listScenarioLoginHintsTool(deps).WithMeta("scenarios", "发现场景生成可用的登录凭据候选（已保存用例/环境配置）"),
		// mock servers
		listMockServersTool(deps).WithMeta("mock", "列出项目下全部Mock Server及状态"),
		getMockServerStatusTool(deps).WithMeta("mock", "查询单个Mock Server运行状态"),
		listMockRoutesTool(deps).WithMeta("mock", "列出某Mock Server下的全部路由"),
		// mock value sets
		listMockValueSetsTool(deps).WithMeta("mockset", "列出项目下全部命名值集合"),
		// test data
		listTestDataTablesTool(deps).WithMeta("testdata", "列出项目下全部测试数据表"),
		getTestDataTableTool(deps).WithMeta("testdata", "查询单个测试数据表及列定义"),
		listTestDataRowsTool(deps).WithMeta("testdata", "列出某测试数据表的全部行"),
		// param sources
		listDataSourcesTool(deps).WithMeta("paramsource", "列出平台全部数据库数据源"),
		listSQLParameterSourcesTool(deps).WithMeta("paramsource", "列出某服务下的SQL参数源"),
		previewSQLParameterSourceTool(deps).WithMeta("paramsource", "预览SQL参数源执行结果（只读）"),
		getDataSourceSchemaTool(deps).WithMeta("paramsource", "查询数据源的表结构（表名+列名）"),
		executeSQLQueryTool(deps).WithMeta("paramsource", "对数据源执行只读SQL查询，获取真实业务数据"),
		// scripts
		listScriptTemplatesTool(deps).WithMeta("scripts", "列出项目可用的脚本模板"),
		// runs
		listRunsTool(deps).WithMeta("runs", "分页列出项目的场景运行记录"),
		getRunResultTool(deps).WithMeta("runs", "查询单次运行详情与步骤结果"),
	}
}

// Mutating returns write tools for the conversational assistant. Delete
// tools set RequiresConfirm and pause the stream until the user approves.
//
// The deliberate omission here is anything that "runs" a scenario or
// triggers a test execution: by product decision, the user always pushes
// the run button themselves, so the AI's contract ends at generating the
// scenario ready to run.
func Mutating(deps Deps) []aitools.Tool {
	return []aitools.Tool{
		// projects / services / environments
		createServiceTool(deps).WithMeta("meta", "在当前项目下新建服务"),
		updateServiceTool(deps).WithMeta("meta", "更新服务的名称与描述"),
		deleteServiceTool(deps).WithMeta("meta", "删除服务及其关联配置（不可恢复）"),
		createServiceEnvironmentTool(deps).WithMeta("meta", "为服务创建运行环境"),
		updateServiceEnvironmentTool(deps).WithMeta("meta", "更新服务级运行环境"),
		deleteServiceEnvironmentTool(deps).WithMeta("meta", "删除服务级运行环境（不可恢复）"),
		// specs
		importOpenAPITool(deps).WithMeta("spec", "向服务导入OpenAPI/Swagger文档"),
		// cases
		createCaseFromEndpointTool(deps).WithMeta("cases", "为接口创建可运行的请求模板"),
		updateCaseAssertionsTool(deps).WithMeta("cases", "覆盖测试用例的断言数组"),
		updateCaseTool(deps).WithMeta("cases", "部分更新接口模板的名称或断言"),
		// scenarios
		generateCoverageScenariosTool(deps).WithMeta("scenarios", "一键生成覆盖全部接口的测试场景"),
		createScenarioWithStepsTool(deps).WithMeta("scenarios", "一次性创建场景及其全部步骤"),
		addScenarioStepTool(deps).WithMeta("scenarios", "向已有场景追加一个步骤"),
		updateScenarioStepTool(deps).WithMeta("scenarios", "按stepOrder修改场景步骤字段"),
		deleteScenarioStepTool(deps).WithMeta("scenarios", "按stepId删除场景步骤"),
		reorderScenarioStepsTool(deps).WithMeta("scenarios", "重新排列场景步骤顺序"),
		updateScenarioTool(deps).WithMeta("scenarios", "更新场景的名称与描述"),
		deleteScenarioTool(deps).WithMeta("scenarios", "删除整个场景及其步骤（不可恢复）"),
		// mock servers
		createMockServerTool(deps).WithMeta("mock", "创建Mock Server"),
		updateMockServerTool(deps).WithMeta("mock", "更新Mock Server配置"),
		deleteMockServerTool(deps).WithMeta("mock", "删除Mock Server及其路由（不可恢复）"),
		createMockRouteTool(deps).WithMeta("mock", "为Mock Server新增路由规则"),
		updateMockRouteTool(deps).WithMeta("mock", "更新Mock Server路由配置"),
		deleteMockRouteTool(deps).WithMeta("mock", "删除Mock Server路由（不可恢复）"),
		// mock value sets
		createMockValueSetTool(deps).WithMeta("mockset", "创建命名值集合"),
		updateMockValueSetTool(deps).WithMeta("mockset", "更新命名值集合"),
		deleteMockValueSetTool(deps).WithMeta("mockset", "删除命名值集合（不可恢复）"),
		// test data
		createTestDataTableTool(deps).WithMeta("testdata", "创建测试数据表"),
		updateTestDataTableTool(deps).WithMeta("testdata", "更新测试数据表与列定义"),
		deleteTestDataTableTool(deps).WithMeta("testdata", "删除测试数据表及其数据（不可恢复）"),
		replaceTestDataRowsTool(deps).WithMeta("testdata", "整体替换测试数据表的行数据"),
		// param sources
		createDataSourceTool(deps).WithMeta("paramsource", "创建数据库数据源"),
		updateDataSourceTool(deps).WithMeta("paramsource", "更新数据库数据源配置"),
		deleteDataSourceTool(deps).WithMeta("paramsource", "删除数据库数据源（不可恢复）"),
		createSQLParameterSourceTool(deps).WithMeta("paramsource", "创建SQL参数源"),
		updateSQLParameterSourceTool(deps).WithMeta("paramsource", "更新SQL参数源配置"),
		deleteSQLParameterSourceTool(deps).WithMeta("paramsource", "删除SQL参数源（不可恢复）"),
		// scripts
		createScriptTemplateTool(deps).WithMeta("scripts", "创建项目自定义脚本模板"),
		updateScriptTemplateTool(deps).WithMeta("scripts", "更新项目自定义脚本模板"),
		deleteScriptTemplateTool(deps).WithMeta("scripts", "删除项目自定义脚本模板"),
	}
}

// All concatenates ReadOnly + Mutating + optional gated tools in a stable order.
func All(deps Deps) []aitools.Tool {
	tools := ReadOnly(deps)
	tools = append(tools, Mutating(deps)...)
	tools = append(tools, GatedMutating(deps)...)
	tools = append(tools, askQuestionTool().WithMeta("meta", "向用户提出交互式问题（单选/多选/文本输入），等待用户回答后继续"))
	return tools
}

// rawSchema is a tiny helper to author JSON Schema bodies as Go raw
// strings while still going through json.Unmarshal as a syntax sanity
// check. A parse failure is a programming bug and panics; production
// callers never see it.
func rawSchema(s string) json.RawMessage {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		panic(fmt.Errorf("aitools/builtin: invalid JSON schema literal: %w\n%s", err, s))
	}
	return json.RawMessage(s)
}
