// Package testdata manages project-scoped test data tables: rows of arbitrary
// columns whose values can be authored manually, generated through the shared
// `internal/mockdata` helpers, or produced by an AI provider via
// `internal/aiprovider`. Tables are referenced from request templates and
// scenarios via the inline `{{$ds.<tableKey>[<col>=<value>].<col>}}` syntax,
// resolved by the runner before request rendering.
package testdata

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors returned by the testdata service and repository.
var (
	ErrTableNotFound         = errors.New("test data table not found")
	ErrRowNotFound           = errors.New("test data row not found")
	ErrTableKeyConflict      = errors.New("test data table key already exists in project")
	ErrInvalidColumn         = errors.New("invalid column definition")
	ErrInvalidGenerator      = errors.New("invalid column generator")
	ErrUnknownMockHelper     = errors.New("unknown mock helper")
	ErrAIProviderUnavailable = errors.New("ai provider service is unavailable")
	// ErrAIProviderUnconfigured 表示既没有项目级 prompt 也没有项目默认 AI 提供商，调用方需先配置。
	ErrAIProviderUnconfigured = errors.New("请先在「AI 能力管理 → 动作 Prompt」或「AI 提供商」中配置 generate_case_data 的提供商")
)

// Generator kinds supported by a column. Each row is generated independently
// according to the column's kind during a `GenerateRows` invocation.
const (
	KindManual = "manual"
	KindMock   = "mock"
	KindAI     = "ai"
)

// Supported column data types. The runtime always returns string values for
// inline references; the type only documents intent and is used by the UI.
const (
	ColumnTypeString  = "string"
	ColumnTypeInteger = "integer"
	ColumnTypeNumber  = "number"
	ColumnTypeBoolean = "boolean"
	ColumnTypeJSON    = "json"
)

// Table is the API representation of a test data table.
type Table struct {
	ID          uuid.UUID   `json:"id"`
	ProjectID   uuid.UUID   `json:"projectId"`
	Key         string      `json:"key"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Columns     []ColumnDef `json:"columns"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

// ColumnDef describes a single column inside a Table.
type ColumnDef struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Description string    `json:"description,omitempty"`
	Generator   Generator `json:"generator"`
}

// Generator describes how a column value is produced when running
// `GenerateRows` or `RegenerateCells`. Only the section matching `Kind` is
// considered; the others may be empty.
type Generator struct {
	Kind string         `json:"kind"`
	Mock *MockGenerator `json:"mock,omitempty"`
	AI   *AIGenerator   `json:"ai,omitempty"`
}

// MockGenerator selects a helper from the shared mockdata catalogue and
// optional positional arguments forwarded to `mockdata.Eval`.
type MockGenerator struct {
	Helper string   `json:"helper"`
	Args   []string `json:"args,omitempty"`
}

// AIGenerator 标记一列使用 AI 生成。具体的 provider / model / system prompt
// 全部由项目级 Prompt 管理（action=generate_case_data）控制，列上不再保存
// providerId / prompt / model。结构体保留为空 struct 以便序列化兼容旧数据：
// 反序列化时旧字段会被忽略，不会报错。
type AIGenerator struct{}

// Row is the API representation of a single row inside a table.
type Row struct {
	ID        uuid.UUID         `json:"id"`
	TableID   uuid.UUID         `json:"tableId"`
	RowIndex  int               `json:"rowIndex"`
	Values    map[string]string `json:"values"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// TableInput carries the create/update payload for a Table.
type TableInput struct {
	Key         string      `json:"key"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Columns     []ColumnDef `json:"columns"`
}

// RowsReplaceInput replaces the entire row set for a table.
type RowsReplaceInput struct {
	Rows []RowInput `json:"rows"`
}

// RowInput carries the user-provided values for a single row.
type RowInput struct {
	Values map[string]string `json:"values"`
}

// GenerateRowsInput controls a batch row generation request.
type GenerateRowsInput struct {
	RowCount       int  `json:"rowCount"`
	PreserveManual bool `json:"preserveManual"`
}

// GenerateRowsOutput is returned to the caller after a generate request,
// pairing the materialised rows with any non-fatal warnings (for example AI
// columns whose provider call failed).
type GenerateRowsOutput struct {
	Rows     []Row    `json:"rows"`
	Warnings []string `json:"warnings,omitempty"`
}

// RegenerateCellsInput limits regeneration to a subset of columns of a single
// row.
type RegenerateCellsInput struct {
	ColumnKeys []string `json:"columnKeys"`
}

// InlineReference describes a single `{{$ds.<table>[<col>=<val>].<col>}}`
// occurrence parsed from a request template or scenario step.
type InlineReference struct {
	Expression   string
	TableKey     string
	Column       string
	FilterColumn string
	FilterValue  string
}

// Snapshot is recorded per-table during inline reference resolution; it
// mirrors the shape of `paramsource.ExecutionSnapshot` so the run report can
// surface it consistently next to SQL inline snapshots.
type Snapshot struct {
	TableID   uuid.UUID         `json:"tableId"`
	TableKey  string            `json:"tableKey"`
	TableName string            `json:"tableName"`
	RowID     uuid.UUID         `json:"rowId,omitempty"`
	RowIndex  int               `json:"rowIndex"`
	Result    map[string]string `json:"result"`
	Error     string            `json:"error,omitempty"`
}
