package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/paramsource"

	"github.com/google/uuid"
)

func listDataSourcesTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_data_sources",
		"列出平台全部数据库数据源（Postgres 连接配置）。",
		rawSchema(`{
            "type": "object",
            "properties": {},
            "additionalProperties": false
        }`),
		func(ctx context.Context, _ json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("list_data_sources: param source 服务未配置")
			}
			items, err := deps.ParamSource.ListDataSources(ctx)
			if err != nil {
				return nil, fmt.Errorf("list_data_sources: 查询失败: %w", err)
			}
			return map[string]any{
				"dataSources": items,
				"count":       len(items),
			}, nil
		})
}

func createDataSourceTool(deps Deps) aitools.Tool {
	return writeTool("create_data_source",
		"创建数据库数据源（name / driver / host / port / database / 凭据等）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "name":        {"type": "string"},
                "description": {"type": "string"},
                "driver":      {"type": "string"},
                "host":        {"type": "string"},
                "port":        {"type": "integer"},
                "database":    {"type": "string"},
                "username":    {"type": "string"},
                "password":    {"type": "string"},
                "sslMode":     {"type": "string"},
                "enabled":     {"type": "boolean"},
                "options":     {"type": ["object", "null"]}
            },
            "required": ["name", "driver", "host", "database"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("create_data_source: param source 服务未配置")
			}
			input, err := decodeDataSourceInput(args)
			if err != nil {
				return nil, fmt.Errorf("create_data_source: %w", err)
			}
			ds, err := deps.ParamSource.CreateDataSource(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("create_data_source: 创建失败: %w", err)
			}
			return ds, nil
		})
}

func updateDataSourceTool(deps Deps) aitools.Tool {
	return writeTool("update_data_source",
		"更新数据库数据源配置。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "dataSourceId": {"type": "string"},
                "name":         {"type": "string"},
                "description":  {"type": "string"},
                "driver":       {"type": "string"},
                "host":         {"type": "string"},
                "port":         {"type": "integer"},
                "database":     {"type": "string"},
                "username":     {"type": "string"},
                "password":     {"type": "string"},
                "sslMode":      {"type": "string"},
                "enabled":      {"type": "boolean"},
                "options":      {"type": ["object", "null"]}
            },
            "required": ["dataSourceId", "name", "driver", "host", "database"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("update_data_source: param source 服务未配置")
			}
			var wrapper struct {
				DataSourceID string `json:"dataSourceId"`
			}
			if err := json.Unmarshal(args, &wrapper); err != nil {
				return nil, fmt.Errorf("update_data_source: 解析参数失败: %w", err)
			}
			id, err := uuid.Parse(strings.TrimSpace(wrapper.DataSourceID))
			if err != nil {
				return nil, fmt.Errorf("update_data_source: dataSourceId 不是合法 UUID: %w", err)
			}
			input, err := decodeDataSourceInput(args)
			if err != nil {
				return nil, fmt.Errorf("update_data_source: %w", err)
			}
			ds, err := deps.ParamSource.UpdateDataSource(ctx, id, input)
			if err != nil {
				return nil, fmt.Errorf("update_data_source: 更新失败: %w", err)
			}
			return ds, nil
		})
}

func deleteDataSourceTool(deps Deps) aitools.Tool {
	return deleteTool("delete_data_source",
		"删除数据库数据源（不可恢复）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "dataSourceId": {"type": "string"}
            },
            "required": ["dataSourceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("delete_data_source: param source 服务未配置")
			}
			var p struct {
				DataSourceID string `json:"dataSourceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_data_source: 解析参数失败: %w", err)
			}
			id, err := uuid.Parse(strings.TrimSpace(p.DataSourceID))
			if err != nil {
				return nil, fmt.Errorf("delete_data_source: dataSourceId 不是合法 UUID: %w", err)
			}
			if err := deps.ParamSource.DeleteDataSource(ctx, id); err != nil {
				return nil, fmt.Errorf("delete_data_source: 删除失败: %w", err)
			}
			return map[string]any{
				"dataSourceId": id,
				"deleted":      true,
			}, nil
		})
}

func listSQLParameterSourcesTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_sql_parameter_sources",
		"列出某服务下的 SQL 参数源。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string"}
            },
            "required": ["serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("list_sql_parameter_sources: param source 服务未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_sql_parameter_sources: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_sql_parameter_sources: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("list_sql_parameter_sources: serviceId 不是合法 UUID: %w", err)
			}
			items, err := deps.ParamSource.ListSQLParameterSources(ctx, projectID, serviceID)
			if err != nil {
				return nil, fmt.Errorf("list_sql_parameter_sources: 查询失败: %w", err)
			}
			return map[string]any{
				"sources": items,
				"count":   len(items),
			}, nil
		})
}

func createSQLParameterSourceTool(deps Deps) aitools.Tool {
	return writeTool("create_sql_parameter_source",
		"创建 SQL 参数源（key / name / sql / dataSourceId / inputParams 等）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId":     {"type": "string"},
                "dataSourceId":  {"type": "string"},
                "key":           {"type": "string"},
                "name":          {"type": "string"},
                "description":   {"type": "string"},
                "sql":           {"type": "string"},
                "inputParams":   {"type": ["array", "object", "null"]},
                "enabled":       {"type": "boolean"},
                "timeoutMillis": {"type": "integer"}
            },
            "required": ["serviceId", "dataSourceId", "key", "name", "sql"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("create_sql_parameter_source: param source 服务未配置")
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_sql_parameter_source: %w", err)
			}
			input, err := decodeSQLParameterSourceInput(args, projectID)
			if err != nil {
				return nil, fmt.Errorf("create_sql_parameter_source: %w", err)
			}
			src, err := deps.ParamSource.CreateSQLParameterSource(ctx, input)
			if err != nil {
				return nil, fmt.Errorf("create_sql_parameter_source: 创建失败: %w", err)
			}
			return src, nil
		})
}

func updateSQLParameterSourceTool(deps Deps) aitools.Tool {
	return writeTool("update_sql_parameter_source",
		"更新 SQL 参数源配置。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "sourceId":      {"type": "string"},
                "serviceId":     {"type": "string"},
                "dataSourceId":  {"type": "string"},
                "key":           {"type": "string"},
                "name":          {"type": "string"},
                "description":   {"type": "string"},
                "sql":           {"type": "string"},
                "inputParams":   {"type": ["array", "object", "null"]},
                "enabled":       {"type": "boolean"},
                "timeoutMillis": {"type": "integer"}
            },
            "required": ["sourceId", "serviceId", "dataSourceId", "key", "name", "sql"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("update_sql_parameter_source: param source 服务未配置")
			}
			var wrapper struct {
				SourceID string `json:"sourceId"`
			}
			if err := json.Unmarshal(args, &wrapper); err != nil {
				return nil, fmt.Errorf("update_sql_parameter_source: 解析参数失败: %w", err)
			}
			sourceID, err := uuid.Parse(strings.TrimSpace(wrapper.SourceID))
			if err != nil {
				return nil, fmt.Errorf("update_sql_parameter_source: sourceId 不是合法 UUID: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("update_sql_parameter_source: %w", err)
			}
			input, err := decodeSQLParameterSourceInput(args, projectID)
			if err != nil {
				return nil, fmt.Errorf("update_sql_parameter_source: %w", err)
			}
			if err := verifySQLParameterSourceInProject(ctx, deps, projectID, input.ServiceID, sourceID); err != nil {
				return nil, fmt.Errorf("update_sql_parameter_source: %w", err)
			}
			src, err := deps.ParamSource.UpdateSQLParameterSource(ctx, sourceID, input)
			if err != nil {
				return nil, fmt.Errorf("update_sql_parameter_source: 更新失败: %w", err)
			}
			return src, nil
		})
}

func deleteSQLParameterSourceTool(deps Deps) aitools.Tool {
	return deleteTool("delete_sql_parameter_source",
		"删除 SQL 参数源（不可恢复）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "serviceId": {"type": "string"},
                "sourceId":  {"type": "string"}
            },
            "required": ["serviceId", "sourceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("delete_sql_parameter_source: param source 服务未配置")
			}
			var p struct {
				ServiceID string `json:"serviceId"`
				SourceID  string `json:"sourceId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_sql_parameter_source: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("delete_sql_parameter_source: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("delete_sql_parameter_source: serviceId 不是合法 UUID: %w", err)
			}
			sourceID, err := uuid.Parse(strings.TrimSpace(p.SourceID))
			if err != nil {
				return nil, fmt.Errorf("delete_sql_parameter_source: sourceId 不是合法 UUID: %w", err)
			}
			if err := verifySQLParameterSourceInProject(ctx, deps, projectID, serviceID, sourceID); err != nil {
				return nil, fmt.Errorf("delete_sql_parameter_source: %w", err)
			}
			if err := deps.ParamSource.DeleteSQLParameterSource(ctx, sourceID); err != nil {
				return nil, fmt.Errorf("delete_sql_parameter_source: 删除失败: %w", err)
			}
			return map[string]any{
				"sourceId": sourceID,
				"deleted":  true,
			}, nil
		})
}

func previewSQLParameterSourceTool(deps Deps) aitools.Tool {
	return readOnlyTool("preview_sql_parameter_source",
		"预览 SQL 参数源执行结果（只读，不写入数据）。可传入 variables / inputParams 作为绑定参数。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "sourceId":    {"type": "string"},
                "serviceId":   {"type": "string", "description": "用于校验 source 属于当前项目/服务"},
                "variables":   {"type": "object"},
                "inputParams": {"type": "object"}
            },
            "required": ["sourceId", "serviceId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.ParamSource == nil {
				return nil, errors.New("preview_sql_parameter_source: param source 服务未配置")
			}
			var p struct {
				SourceID    string         `json:"sourceId"`
				ServiceID   string         `json:"serviceId"`
				Variables   map[string]any `json:"variables"`
				InputParams map[string]any `json:"inputParams"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("preview_sql_parameter_source: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("preview_sql_parameter_source: %w", err)
			}
			serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
			if err != nil {
				return nil, fmt.Errorf("preview_sql_parameter_source: serviceId 不是合法 UUID: %w", err)
			}
			sourceID, err := uuid.Parse(strings.TrimSpace(p.SourceID))
			if err != nil {
				return nil, fmt.Errorf("preview_sql_parameter_source: sourceId 不是合法 UUID: %w", err)
			}
			if err := verifySQLParameterSourceInProject(ctx, deps, projectID, serviceID, sourceID); err != nil {
				return nil, fmt.Errorf("preview_sql_parameter_source: %w", err)
			}
			out, err := deps.ParamSource.PreviewSQLParameterSource(ctx, sourceID, paramsource.PreviewInput{
				Variables:   p.Variables,
				InputParams: p.InputParams,
			})
			if err != nil {
				return nil, fmt.Errorf("preview_sql_parameter_source: 预览失败: %w", err)
			}
			return out, nil
		})
}

func decodeDataSourceInput(args json.RawMessage) (paramsource.DataSourceInput, error) {
	var input paramsource.DataSourceInput
	if err := json.Unmarshal(args, &input); err != nil {
		return paramsource.DataSourceInput{}, fmt.Errorf("解析参数失败: %w", err)
	}
	return input, nil
}

func decodeSQLParameterSourceInput(args json.RawMessage, projectID uuid.UUID) (paramsource.SQLParameterSourceInput, error) {
	var p struct {
		ServiceID     string          `json:"serviceId"`
		DataSourceID  string          `json:"dataSourceId"`
		Key           string          `json:"key"`
		Name          string          `json:"name"`
		Description   string          `json:"description"`
		SQL           string          `json:"sql"`
		InputParams   json.RawMessage `json:"inputParams"`
		Enabled       *bool           `json:"enabled"`
		TimeoutMillis int             `json:"timeoutMillis"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return paramsource.SQLParameterSourceInput{}, fmt.Errorf("解析参数失败: %w", err)
	}
	serviceID, err := uuid.Parse(strings.TrimSpace(p.ServiceID))
	if err != nil {
		return paramsource.SQLParameterSourceInput{}, fmt.Errorf("serviceId 不是合法 UUID: %w", err)
	}
	dataSourceID, err := uuid.Parse(strings.TrimSpace(p.DataSourceID))
	if err != nil {
		return paramsource.SQLParameterSourceInput{}, fmt.Errorf("dataSourceId 不是合法 UUID: %w", err)
	}
	inputParams := p.InputParams
	if len(inputParams) == 0 || isJSONNull(inputParams) {
		inputParams = json.RawMessage(`[]`)
	}
	return paramsource.SQLParameterSourceInput{
		ProjectID:     projectID,
		ServiceID:     serviceID,
		DataSourceID:  dataSourceID,
		Key:           strings.TrimSpace(p.Key),
		Name:          strings.TrimSpace(p.Name),
		Description:   strings.TrimSpace(p.Description),
		SQL:           strings.TrimSpace(p.SQL),
		InputParams:   inputParams,
		Enabled:       p.Enabled,
		TimeoutMillis: p.TimeoutMillis,
	}, nil
}

func verifySQLParameterSourceInProject(ctx context.Context, deps Deps, projectID, serviceID, sourceID uuid.UUID) error {
	sources, err := deps.ParamSource.ListSQLParameterSources(ctx, projectID, serviceID)
	if err != nil {
		return fmt.Errorf("校验 SQL 参数源归属失败: %w", err)
	}
	for _, s := range sources {
		if s.ID == sourceID {
			return nil
		}
	}
	return fmt.Errorf("sourceId %s 不属于当前项目/服务", sourceID)
}
