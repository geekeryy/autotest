package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/aitools"
	"autotest/internal/testdata"

	"github.com/google/uuid"
)

func listTestDataTablesTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_test_data_tables",
		"列出当前项目下全部测试数据表摘要。",
		rawSchema(`{
            "type": "object",
            "properties": {},
            "additionalProperties": false
        }`),
		func(ctx context.Context, _ json.RawMessage) (any, error) {
			if deps.TestData == nil {
				return nil, errors.New("list_test_data_tables: test data 服务未配置")
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_test_data_tables: %w", err)
			}
			tables, err := deps.TestData.ListTables(ctx, projectID)
			if err != nil {
				return nil, fmt.Errorf("list_test_data_tables: 查询失败: %w", err)
			}
			return map[string]any{
				"tables": tables,
				"count":  len(tables),
			}, nil
		})
}

func getTestDataTableTool(deps Deps) aitools.Tool {
	return readOnlyTool("get_test_data_table",
		"查询单个测试数据表详情（含列定义）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "tableId": {"type": "string"}
            },
            "required": ["tableId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.TestData == nil {
				return nil, errors.New("get_test_data_table: test data 服务未配置")
			}
			var p struct {
				TableID string `json:"tableId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("get_test_data_table: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("get_test_data_table: %w", err)
			}
			tableID, err := uuid.Parse(strings.TrimSpace(p.TableID))
			if err != nil {
				return nil, fmt.Errorf("get_test_data_table: tableId 不是合法 UUID: %w", err)
			}
			table, err := deps.TestData.GetTable(ctx, projectID, tableID)
			if err != nil {
				return nil, fmt.Errorf("get_test_data_table: 查询失败: %w", err)
			}
			return table, nil
		})
}

func createTestDataTableTool(deps Deps) aitools.Tool {
	return writeTool("create_test_data_table",
		"创建测试数据表（key / name / columns）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "key":         {"type": "string"},
                "name":        {"type": "string"},
                "description": {"type": "string"},
                "columns":     {"type": "array", "items": {"type": "object"}, "minItems": 1}
            },
            "required": ["key", "name", "columns"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.TestData == nil {
				return nil, errors.New("create_test_data_table: test data 服务未配置")
			}
			var p struct {
				Key         string              `json:"key"`
				Name        string              `json:"name"`
				Description string              `json:"description"`
				Columns     []testdata.ColumnDef `json:"columns"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("create_test_data_table: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("create_test_data_table: %w", err)
			}
			table, err := deps.TestData.CreateTable(ctx, projectID, testdata.TableInput{
				Key:         strings.TrimSpace(p.Key),
				Name:        strings.TrimSpace(p.Name),
				Description: strings.TrimSpace(p.Description),
				Columns:     p.Columns,
			})
			if err != nil {
				return nil, fmt.Errorf("create_test_data_table: 创建失败: %w", err)
			}
			return table, nil
		})
}

func updateTestDataTableTool(deps Deps) aitools.Tool {
	return writeTool("update_test_data_table",
		"更新测试数据表元信息与列定义（key 不可修改）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "tableId":     {"type": "string"},
                "name":        {"type": "string"},
                "description": {"type": "string"},
                "columns":     {"type": "array", "items": {"type": "object"}, "minItems": 1}
            },
            "required": ["tableId", "name", "columns"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.TestData == nil {
				return nil, errors.New("update_test_data_table: test data 服务未配置")
			}
			var p struct {
				TableID     string              `json:"tableId"`
				Name        string              `json:"name"`
				Description string              `json:"description"`
				Columns     []testdata.ColumnDef `json:"columns"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("update_test_data_table: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("update_test_data_table: %w", err)
			}
			tableID, err := uuid.Parse(strings.TrimSpace(p.TableID))
			if err != nil {
				return nil, fmt.Errorf("update_test_data_table: tableId 不是合法 UUID: %w", err)
			}
			existing, err := deps.TestData.GetTable(ctx, projectID, tableID)
			if err != nil {
				return nil, fmt.Errorf("update_test_data_table: 查询表失败: %w", err)
			}
			table, err := deps.TestData.UpdateTable(ctx, projectID, tableID, testdata.TableInput{
				Key:         existing.Key,
				Name:        strings.TrimSpace(p.Name),
				Description: strings.TrimSpace(p.Description),
				Columns:     p.Columns,
			})
			if err != nil {
				return nil, fmt.Errorf("update_test_data_table: 更新失败: %w", err)
			}
			return table, nil
		})
}

func deleteTestDataTableTool(deps Deps) aitools.Tool {
	return deleteTool("delete_test_data_table",
		"删除测试数据表及其全部行（不可恢复）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "tableId": {"type": "string"}
            },
            "required": ["tableId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.TestData == nil {
				return nil, errors.New("delete_test_data_table: test data 服务未配置")
			}
			var p struct {
				TableID string `json:"tableId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("delete_test_data_table: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("delete_test_data_table: %w", err)
			}
			tableID, err := uuid.Parse(strings.TrimSpace(p.TableID))
			if err != nil {
				return nil, fmt.Errorf("delete_test_data_table: tableId 不是合法 UUID: %w", err)
			}
			if err := deps.TestData.DeleteTable(ctx, projectID, tableID); err != nil {
				return nil, fmt.Errorf("delete_test_data_table: 删除失败: %w", err)
			}
			return map[string]any{
				"tableId": tableID,
				"deleted": true,
			}, nil
		})
}

func listTestDataRowsTool(deps Deps) aitools.Tool {
	return readOnlyTool("list_test_data_rows",
		"列出某测试数据表的全部行。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "tableId": {"type": "string"}
            },
            "required": ["tableId"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.TestData == nil {
				return nil, errors.New("list_test_data_rows: test data 服务未配置")
			}
			var p struct {
				TableID string `json:"tableId"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("list_test_data_rows: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("list_test_data_rows: %w", err)
			}
			tableID, err := uuid.Parse(strings.TrimSpace(p.TableID))
			if err != nil {
				return nil, fmt.Errorf("list_test_data_rows: tableId 不是合法 UUID: %w", err)
			}
			rows, err := deps.TestData.ListRows(ctx, projectID, tableID)
			if err != nil {
				return nil, fmt.Errorf("list_test_data_rows: 查询失败: %w", err)
			}
			return map[string]any{
				"rows":  rows,
				"count": len(rows),
			}, nil
		})
}

func replaceTestDataRowsTool(deps Deps) aitools.Tool {
	return writeTool("replace_test_data_rows",
		"整体替换测试数据表的行数据（rows 数组将完全覆盖现有行）。",
		rawSchema(`{
            "type": "object",
            "properties": {
                "tableId": {"type": "string"},
                "rows": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "properties": {
                            "values": {"type": "object", "additionalProperties": {"type": "string"}}
                        },
                        "required": ["values"],
                        "additionalProperties": false
                    }
                }
            },
            "required": ["tableId", "rows"],
            "additionalProperties": false
        }`),
		func(ctx context.Context, args json.RawMessage) (any, error) {
			if deps.TestData == nil {
				return nil, errors.New("replace_test_data_rows: test data 服务未配置")
			}
			var p struct {
				TableID string              `json:"tableId"`
				Rows    []testdata.RowInput `json:"rows"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return nil, fmt.Errorf("replace_test_data_rows: 解析参数失败: %w", err)
			}
			projectID, err := aitools.ResolveProjectID(ctx, "")
			if err != nil {
				return nil, fmt.Errorf("replace_test_data_rows: %w", err)
			}
			tableID, err := uuid.Parse(strings.TrimSpace(p.TableID))
			if err != nil {
				return nil, fmt.Errorf("replace_test_data_rows: tableId 不是合法 UUID: %w", err)
			}
			rows, err := deps.TestData.ReplaceRows(ctx, projectID, tableID, testdata.RowsReplaceInput{Rows: p.Rows})
			if err != nil {
				return nil, fmt.Errorf("replace_test_data_rows: 替换失败: %w", err)
			}
			return map[string]any{
				"rows":  rows,
				"count": len(rows),
			}, nil
		})
}
