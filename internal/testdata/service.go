package testdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"autotest/internal/aiprovider"
	"autotest/internal/mockdata"
	"autotest/internal/projectprompt"

	"github.com/google/uuid"
)

const (
	maxGenerateRowCount = 100
	maxRowsForAIContext = 5
)

var (
	tableKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	columnKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
	supportedColumnTypes = map[string]struct{}{
		ColumnTypeString:  {},
		ColumnTypeInteger: {},
		ColumnTypeNumber:  {},
		ColumnTypeBoolean: {},
		ColumnTypeJSON:    {},
	}
)

// AIProviderService 是 testdata 服务依赖的 `internal/aiprovider` 子集。
// 单测可以注入 fake，避免构造真实 provider 客户端。
//   - Chat 用于发起 AI 调用；
//   - List 用于在 `cfg.ProviderID` 为空时回退到项目默认 AI 提供商
//     （IsDefault=true && Enabled=true）。
type AIProviderService interface {
	Chat(ctx context.Context, projectID uuid.UUID, req aiprovider.ChatRequest) (*aiprovider.ChatResponse, error)
	List(ctx context.Context) ([]aiprovider.Provider, error)
}

// PromptService 是 testdata 服务依赖的 `internal/projectprompt` 子集，
// 用于按 action 读取项目级 SystemPrompt / providerId / defaultModel。
type PromptService interface {
	GetByAction(ctx context.Context, action string) (*projectprompt.ProjectPrompt, error)
}

// repositoryAPI is the slice of `Repository` the service depends on. Defined
// as an interface so unit tests can inject an in-memory fake without spinning
// up a Postgres instance.
type repositoryAPI interface {
	ListTables(ctx context.Context, projectID uuid.UUID) ([]Table, error)
	GetTable(ctx context.Context, projectID, tableID uuid.UUID) (*Table, error)
	CreateTable(ctx context.Context, projectID uuid.UUID, input TableInput) (*Table, error)
	UpdateTable(ctx context.Context, projectID, tableID uuid.UUID, input TableInput) (*Table, error)
	DeleteTable(ctx context.Context, projectID, tableID uuid.UUID) error
	ListRows(ctx context.Context, tableID uuid.UUID) ([]Row, error)
	GetRow(ctx context.Context, tableID, rowID uuid.UUID) (*Row, error)
	ReplaceRows(ctx context.Context, tableID uuid.UUID, rows []RowInput) ([]Row, error)
	UpdateRowValues(ctx context.Context, tableID, rowID uuid.UUID, values map[string]string) (*Row, error)
	ListAllForProject(ctx context.Context, projectID uuid.UUID) ([]TableWithRows, error)
}

// Service implements the test data table use cases backed by Repository, an
// optional AI provider service for `ai` columns, and an optional project-level
// prompt service to source the action-scoped configuration.
type Service struct {
	repo      repositoryAPI
	ai        AIProviderService
	promptSvc PromptService
}

// NewService constructs a Service. Either dependency may be nil when the
// project has no AI provider configured; AI columns will then return a clear
// warning/error during generation.
func NewService(repo *Repository, aiSvc AIProviderService, promptSvc PromptService) *Service {
	return &Service{repo: repo, ai: aiSvc, promptSvc: promptSvc}
}

// newServiceForTests is used by package tests to inject a fake repository.
func newServiceForTests(repo repositoryAPI, aiSvc AIProviderService, promptSvc PromptService) *Service {
	return &Service{repo: repo, ai: aiSvc, promptSvc: promptSvc}
}

// ListTables returns all active tables for a project.
func (s *Service) ListTables(ctx context.Context, projectID uuid.UUID) ([]Table, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	return s.repo.ListTables(ctx, projectID)
}

// GetTable returns a single table by id (project-scoped).
func (s *Service) GetTable(ctx context.Context, projectID, tableID uuid.UUID) (*Table, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if tableID == uuid.Nil {
		return nil, errors.New("tableId is required")
	}
	return s.repo.GetTable(ctx, projectID, tableID)
}

// CreateTable validates and persists a new table for the project.
func (s *Service) CreateTable(ctx context.Context, projectID uuid.UUID, input TableInput) (*Table, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if err := normalizeTableInput(&input); err != nil {
		return nil, err
	}
	return s.repo.CreateTable(ctx, projectID, input)
}

// UpdateTable validates and persists changes to an existing table.
func (s *Service) UpdateTable(ctx context.Context, projectID, tableID uuid.UUID, input TableInput) (*Table, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if tableID == uuid.Nil {
		return nil, errors.New("tableId is required")
	}
	if err := normalizeTableInput(&input); err != nil {
		return nil, err
	}
	return s.repo.UpdateTable(ctx, projectID, tableID, input)
}

// DeleteTable soft-deletes an existing table.
func (s *Service) DeleteTable(ctx context.Context, projectID, tableID uuid.UUID) error {
	if projectID == uuid.Nil {
		return errors.New("projectId is required")
	}
	if tableID == uuid.Nil {
		return errors.New("tableId is required")
	}
	return s.repo.DeleteTable(ctx, projectID, tableID)
}

// ListRows returns all rows of a table, ordered by row index.
func (s *Service) ListRows(ctx context.Context, projectID, tableID uuid.UUID) ([]Row, error) {
	if _, err := s.requireTable(ctx, projectID, tableID); err != nil {
		return nil, err
	}
	return s.repo.ListRows(ctx, tableID)
}

// ReplaceRows replaces every row of a table with the supplied list.
func (s *Service) ReplaceRows(ctx context.Context, projectID, tableID uuid.UUID, input RowsReplaceInput) ([]Row, error) {
	table, err := s.requireTable(ctx, projectID, tableID)
	if err != nil {
		return nil, err
	}
	rows := input.Rows
	if rows == nil {
		rows = []RowInput{}
	}
	for index, row := range rows {
		rows[index].Values = sanitizeRowValues(table.Columns, row.Values)
	}
	return s.repo.ReplaceRows(ctx, tableID, rows)
}

// GenerateRows produces `RowCount` rows for a table, applying the column
// generator strategies. Existing rows are preserved up to the requested count;
// missing rows are appended. AI calls that fail are reported via warnings so
// the user can fix the offending column without losing the whole batch.
func (s *Service) GenerateRows(ctx context.Context, projectID, tableID uuid.UUID, input GenerateRowsInput) (*GenerateRowsOutput, error) {
	table, err := s.requireTable(ctx, projectID, tableID)
	if err != nil {
		return nil, err
	}
	rowCount := input.RowCount
	if rowCount <= 0 {
		return nil, errors.New("rowCount must be positive")
	}
	if rowCount > maxGenerateRowCount {
		return nil, fmt.Errorf("rowCount must not exceed %d", maxGenerateRowCount)
	}

	existingRows, err := s.repo.ListRows(ctx, tableID)
	if err != nil {
		return nil, err
	}

	result := make([]RowInput, rowCount)
	for index := range result {
		values := map[string]string{}
		if input.PreserveManual && index < len(existingRows) {
			for key, value := range existingRows[index].Values {
				values[key] = value
			}
		}
		result[index] = RowInput{Values: values}
	}

	warnings := []string{}
	for _, column := range table.Columns {
		switch column.Generator.Kind {
		case "", KindManual:
			ensureColumnPresent(result, column.Key)
		case KindMock:
			if column.Generator.Mock == nil {
				warnings = append(warnings, fmt.Sprintf("column %s: missing mock helper", column.Key))
				ensureColumnPresent(result, column.Key)
				continue
			}
			helper := column.Generator.Mock.Helper
			args := column.Generator.Mock.Args
			for index := range result {
				if input.PreserveManual && hasNonEmptyCell(result[index].Values, column.Key) {
					continue
				}
				value, ok := mockdata.Eval(helper, args)
				if !ok {
					warnings = append(warnings, fmt.Sprintf("column %s: unknown mock helper %q", column.Key, helper))
					result[index].Values[column.Key] = ""
					continue
				}
				result[index].Values[column.Key] = value
			}
		case KindAI:
			if s.ai == nil {
				warnings = append(warnings, fmt.Sprintf("column %s: %s", column.Key, ErrAIProviderUnavailable))
				ensureColumnPresent(result, column.Key)
				continue
			}
			providerID, model, systemPrompt, cfgErr := s.resolveAIConfig(ctx, projectID)
			if cfgErr != nil {
				warnings = append(warnings, fmt.Sprintf("column %s: %s", column.Key, cfgErr.Error()))
				ensureColumnPresent(result, column.Key)
				continue
			}
			generated, warning := s.generateAIColumn(ctx, projectID, providerID, model, systemPrompt, column, table, result, input.PreserveManual)
			if warning != "" {
				warnings = append(warnings, warning)
			}
			for index, value := range generated {
				if index >= len(result) {
					break
				}
				if input.PreserveManual && hasNonEmptyCell(result[index].Values, column.Key) {
					continue
				}
				result[index].Values[column.Key] = value
			}
			ensureColumnPresent(result, column.Key)
		default:
			warnings = append(warnings, fmt.Sprintf("column %s: unsupported generator kind %q", column.Key, column.Generator.Kind))
			ensureColumnPresent(result, column.Key)
		}
	}

	stored, err := s.repo.ReplaceRows(ctx, tableID, result)
	if err != nil {
		return nil, err
	}
	out := &GenerateRowsOutput{Rows: stored}
	if len(warnings) > 0 {
		out.Warnings = warnings
	}
	return out, nil
}

// RegenerateCells re-runs the configured generator for the requested columns
// on a single row. Manual columns are zeroed out so the user can edit them
// without leftover values; mock and ai columns use the same logic as
// `GenerateRows`.
func (s *Service) RegenerateCells(ctx context.Context, projectID, tableID, rowID uuid.UUID, input RegenerateCellsInput) (*Row, error) {
	if rowID == uuid.Nil {
		return nil, errors.New("rowId is required")
	}
	table, err := s.requireTable(ctx, projectID, tableID)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.GetRow(ctx, tableID, rowID)
	if err != nil {
		return nil, err
	}
	requested := normalizeColumnKeys(input.ColumnKeys)
	if len(requested) == 0 {
		return row, nil
	}

	values := map[string]string{}
	for key, value := range row.Values {
		values[key] = value
	}

	for _, column := range table.Columns {
		if _, ok := requested[column.Key]; !ok {
			continue
		}
		switch column.Generator.Kind {
		case "", KindManual:
			values[column.Key] = ""
		case KindMock:
			if column.Generator.Mock == nil {
				return nil, fmt.Errorf("column %s: missing mock helper", column.Key)
			}
			value, ok := mockdata.Eval(column.Generator.Mock.Helper, column.Generator.Mock.Args)
			if !ok {
				return nil, fmt.Errorf("column %s: %w %q", column.Key, ErrUnknownMockHelper, column.Generator.Mock.Helper)
			}
			values[column.Key] = value
		case KindAI:
			if s.ai == nil {
				return nil, fmt.Errorf("column %s: %w", column.Key, ErrAIProviderUnavailable)
			}
			providerID, model, systemPrompt, cfgErr := s.resolveAIConfig(ctx, projectID)
			if cfgErr != nil {
				return nil, fmt.Errorf("column %s: %w", column.Key, cfgErr)
			}
			singleRow := []RowInput{{Values: copyStringMap(values)}}
			generated, warning := s.generateAIColumn(ctx, projectID, providerID, model, systemPrompt, column, table, singleRow, false)
			if warning != "" {
				return nil, errors.New(warning)
			}
			if len(generated) == 0 {
				return nil, fmt.Errorf("column %s: ai response did not include a value", column.Key)
			}
			values[column.Key] = generated[0]
		default:
			return nil, fmt.Errorf("column %s: unsupported generator kind %q", column.Key, column.Generator.Kind)
		}
	}
	values = sanitizeRowValues(table.Columns, values)
	return s.repo.UpdateRowValues(ctx, tableID, rowID, values)
}

// Resolve evaluates the supplied inline references against the project's test
// data tables. The first return value maps the original `$ds` expression to
// its resolved string value; the snapshots are intended to be appended to the
// run report alongside SQL parameter source snapshots.
func (s *Service) Resolve(ctx context.Context, projectID uuid.UUID, refs []InlineReference) (map[string]string, []Snapshot, error) {
	if len(refs) == 0 {
		return map[string]string{}, nil, nil
	}
	if projectID == uuid.Nil {
		return nil, nil, errors.New("projectId is required for test data resolution")
	}
	tables, err := s.repo.ListAllForProject(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	results := map[string]string{}
	snapshots := []Snapshot{}
	cachedRowsByTable := map[uuid.UUID][]Row{}
	snapshotIndexByTable := map[uuid.UUID]int{}

	for _, ref := range DedupeReferences(refs) {
		table, ok := findTableByKey(tables, ref.TableKey)
		if !ok {
			snap := Snapshot{TableKey: ref.TableKey, Error: fmt.Sprintf("test data table %q not found", ref.TableKey)}
			snapshots = append(snapshots, snap)
			return results, snapshots, errors.New(snap.Error)
		}
		rows, cached := cachedRowsByTable[table.Table.ID]
		if !cached {
			rows = table.Rows
			cachedRowsByTable[table.Table.ID] = rows
			snapshots = append(snapshots, Snapshot{
				TableID:   table.Table.ID,
				TableKey:  table.Table.Key,
				TableName: table.Table.Name,
				Result:    map[string]string{},
			})
			snapshotIndexByTable[table.Table.ID] = len(snapshots) - 1
		}
		row, err := selectRow(ref, rows)
		if err != nil {
			snapshotUpdate(snapshots, snapshotIndexByTable[table.Table.ID], func(snap *Snapshot) {
				snap.Error = err.Error()
			})
			return results, snapshots, err
		}
		value, ok := lookupColumn(row.Values, ref.Column)
		if !ok {
			err := fmt.Errorf("test data inline reference %q column %q not found", ref.Expression, ref.Column)
			snapshotUpdate(snapshots, snapshotIndexByTable[table.Table.ID], func(snap *Snapshot) {
				snap.Error = err.Error()
			})
			return results, snapshots, err
		}
		results[ref.Expression] = value
		snapshotUpdate(snapshots, snapshotIndexByTable[table.Table.ID], func(snap *Snapshot) {
			snap.RowID = row.ID
			snap.RowIndex = row.RowIndex
			if snap.Result == nil {
				snap.Result = map[string]string{}
			}
			snap.Result[ref.Column] = value
		})
	}
	return results, snapshots, nil
}

// MarshalSnapshots returns a JSON encoding of the supplied snapshots. It
// always returns a valid JSON array even when no snapshots are present.
func MarshalSnapshots(snapshots []Snapshot) json.RawMessage {
	if len(snapshots) == 0 {
		return []byte(`[]`)
	}
	raw, _ := json.Marshal(snapshots)
	return raw
}

func (s *Service) requireTable(ctx context.Context, projectID, tableID uuid.UUID) (*Table, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if tableID == uuid.Nil {
		return nil, errors.New("tableId is required")
	}
	return s.repo.GetTable(ctx, projectID, tableID)
}

// resolveAIConfig 读取项目级 prompt 配置并选择最终使用的 providerID / model /
// systemPrompt：
//   - cfg.ProviderID 优先；为空时回退到项目默认 AI provider
//     （IsDefault=true && Enabled=true）；
//   - cfg.DefaultModel 优先，否则交给 aiprovider.Service.Chat 自行处理；
//   - cfg.SystemPrompt 仅当 cfg.Enabled 时透传，由 aiprovider 注入到消息中。
//
// 找不到任何可用 provider 时返回 ErrAIProviderUnconfigured（中文）以便调用方
// 将其作为 warning 收集；不会因为 prompt 表查询失败而阻断整次生成。
func (s *Service) resolveAIConfig(ctx context.Context, projectID uuid.UUID) (uuid.UUID, string, string, error) {
	var cfg *projectprompt.ProjectPrompt
	if s.promptSvc != nil {
		got, err := s.promptSvc.GetByAction(ctx, projectprompt.ActionGenerateCaseData)
		if err == nil && got != nil {
			cfg = got
		}
	}

	var providerID uuid.UUID
	if cfg != nil && cfg.ProviderID != nil && *cfg.ProviderID != uuid.Nil {
		providerID = *cfg.ProviderID
	}
	if providerID == uuid.Nil {
		fallback, err := s.lookupDefaultProvider(ctx)
		if err != nil {
			return uuid.Nil, "", "", err
		}
		providerID = fallback
	}
	if providerID == uuid.Nil {
		return uuid.Nil, "", "", ErrAIProviderUnconfigured
	}

	model := ""
	systemPrompt := ""
	if cfg != nil {
		model = strings.TrimSpace(cfg.DefaultModel)
		if cfg.Enabled {
			systemPrompt = cfg.SystemPrompt
		}
	}
	return providerID, model, systemPrompt, nil
}

// lookupDefaultProvider 找到项目内 IsDefault=true 且 Enabled=true 的 provider；
// 若没有 IsDefault 但有任意启用 provider，则返回第一个启用项以便用户在仅配置一个
// provider 时也能直接使用。两者都没有时返回 ErrAIProviderUnconfigured。
func (s *Service) lookupDefaultProvider(ctx context.Context) (uuid.UUID, error) {
	if s.ai == nil {
		return uuid.Nil, ErrAIProviderUnavailable
	}
	providers, err := s.ai.List(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("查询平台默认 AI 提供商失败: %w", err)
	}
	var firstEnabled uuid.UUID
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		if p.IsDefault {
			return p.ID, nil
		}
		if firstEnabled == uuid.Nil {
			firstEnabled = p.ID
		}
	}
	if firstEnabled != uuid.Nil {
		return firstEnabled, nil
	}
	return uuid.Nil, ErrAIProviderUnconfigured
}

func (s *Service) generateAIColumn(
	ctx context.Context,
	projectID uuid.UUID,
	providerID uuid.UUID,
	model string,
	systemPrompt string,
	column ColumnDef,
	table *Table,
	rows []RowInput,
	preserveManual bool,
) ([]string, string) {
	contextRows := rows
	if len(contextRows) > maxRowsForAIContext {
		contextRows = contextRows[:maxRowsForAIContext]
	}
	contextPayload := map[string]any{
		"tableKey":    table.Key,
		"tableName":   table.Name,
		"description": table.Description,
		"targetColumn": map[string]any{
			"key":         column.Key,
			"name":        column.Name,
			"type":        column.Type,
			"description": column.Description,
		},
		"rowCount":   len(rows),
		"columns":    columnSummaries(table.Columns),
		"sampleRows": rowsToValueList(contextRows),
	}
	rawContext, _ := json.Marshal(contextPayload)
	prompt := fmt.Sprintf("请为列 %q 生成 %d 行符合上下文的取值。", column.Key, len(rows))
	resp, err := s.ai.Chat(ctx, projectID, aiprovider.ChatRequest{
		ProviderID:           providerID,
		Action:               aiprovider.ActionGenerateCaseData,
		Prompt:               prompt,
		Context:              rawContext,
		Model:                model,
		SystemPromptOverride: systemPrompt,
	})
	if err != nil {
		return nil, fmt.Sprintf("column %s: ai generation failed: %s", column.Key, err.Error())
	}
	values, parseErr := extractAIColumnValues(resp, column.Key, len(rows))
	if parseErr != nil {
		return nil, fmt.Sprintf("column %s: %s", column.Key, parseErr.Error())
	}
	if preserveManual {
		filtered := make([]string, 0, len(values))
		for index, value := range values {
			if index >= len(rows) {
				break
			}
			if hasNonEmptyCell(rows[index].Values, column.Key) {
				filtered = append(filtered, rows[index].Values[column.Key])
				continue
			}
			filtered = append(filtered, value)
		}
		return filtered, ""
	}
	return values, ""
}

func extractAIColumnValues(resp *aiprovider.ChatResponse, columnKey string, expected int) ([]string, error) {
	if resp == nil {
		return nil, errors.New("ai response is empty")
	}
	candidate := resp.Parsed
	if len(candidate) == 0 {
		candidate = []byte(strings.TrimSpace(resp.Text))
	}
	if len(candidate) == 0 {
		return nil, errors.New("ai response did not include any data")
	}

	var asArray []map[string]any
	if err := json.Unmarshal(candidate, &asArray); err == nil {
		return mapAIRowsToColumn(asArray, columnKey, expected), nil
	}
	var asWrapper struct {
		Cases []map[string]any `json:"cases"`
		Rows  []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(candidate, &asWrapper); err == nil {
		if len(asWrapper.Cases) > 0 {
			return mapAIRowsToColumn(asWrapper.Cases, columnKey, expected), nil
		}
		if len(asWrapper.Rows) > 0 {
			return mapAIRowsToColumn(asWrapper.Rows, columnKey, expected), nil
		}
	}
	var asObject map[string]any
	if err := json.Unmarshal(candidate, &asObject); err == nil {
		if value, ok := asObject[columnKey]; ok {
			return []string{aiValueToString(value)}, nil
		}
	}
	return nil, errors.New("ai response did not include the expected structure (array or {cases:[...]})")
}

func mapAIRowsToColumn(items []map[string]any, columnKey string, expected int) []string {
	values := make([]string, 0, expected)
	for _, item := range items {
		value, ok := item[columnKey]
		if !ok {
			if body, has := item["body"]; has {
				if bodyMap, isMap := body.(map[string]any); isMap {
					if nested, found := bodyMap[columnKey]; found {
						value = nested
						ok = true
					}
				}
			}
		}
		if ok {
			values = append(values, aiValueToString(value))
		} else {
			values = append(values, "")
		}
		if len(values) >= expected {
			break
		}
	}
	for len(values) < expected {
		values = append(values, "")
	}
	return values
}

func aiValueToString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case json.Number:
		return v.String()
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(raw)
	}
}

func columnSummaries(columns []ColumnDef) []map[string]any {
	out := make([]map[string]any, 0, len(columns))
	for _, column := range columns {
		out = append(out, map[string]any{
			"key":         column.Key,
			"name":        column.Name,
			"type":        column.Type,
			"description": column.Description,
		})
	}
	return out
}

func rowsToValueList(rows []RowInput) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		clone := map[string]string{}
		for key, value := range row.Values {
			clone[key] = value
		}
		out = append(out, clone)
	}
	return out
}

func ensureColumnPresent(rows []RowInput, key string) {
	for index := range rows {
		if rows[index].Values == nil {
			rows[index].Values = map[string]string{}
		}
		if _, ok := rows[index].Values[key]; !ok {
			rows[index].Values[key] = ""
		}
	}
}

func hasNonEmptyCell(values map[string]string, key string) bool {
	if values == nil {
		return false
	}
	value, ok := values[key]
	if !ok {
		return false
	}
	return strings.TrimSpace(value) != ""
}

func sanitizeRowValues(columns []ColumnDef, values map[string]string) map[string]string {
	if values == nil {
		values = map[string]string{}
	}
	out := map[string]string{}
	knownKeys := map[string]struct{}{}
	for _, column := range columns {
		knownKeys[column.Key] = struct{}{}
		if value, ok := values[column.Key]; ok {
			out[column.Key] = value
		} else {
			out[column.Key] = ""
		}
	}
	for key, value := range values {
		if _, ok := knownKeys[key]; ok {
			continue
		}
		out[key] = value
	}
	return out
}

func normalizeColumnKeys(keys []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = struct{}{}
	}
	return out
}

func copyStringMap(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func findTableByKey(tables []TableWithRows, key string) (*TableWithRows, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false
	}
	for index := range tables {
		table := &tables[index]
		if table.Table.Key == key {
			return table, true
		}
	}
	return nil, false
}

func selectRow(ref InlineReference, rows []Row) (*Row, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("test data inline reference %q has no rows", ref.Expression)
	}
	if ref.FilterColumn == "" {
		row := rows[0]
		return &row, nil
	}
	for index := range rows {
		row := &rows[index]
		value, ok := lookupColumn(row.Values, ref.FilterColumn)
		if ok && value == ref.FilterValue {
			clone := *row
			return &clone, nil
		}
	}
	return nil, fmt.Errorf("test data inline reference %q found no row where %s=%s", ref.Expression, ref.FilterColumn, ref.FilterValue)
}

func lookupColumn(values map[string]string, column string) (string, bool) {
	if values == nil {
		return "", false
	}
	if value, ok := values[column]; ok {
		return value, true
	}
	lower := strings.ToLower(column)
	for key, value := range values {
		if strings.ToLower(key) == lower {
			return value, true
		}
	}
	return "", false
}

func snapshotUpdate(snapshots []Snapshot, index int, mutate func(*Snapshot)) {
	if index < 0 || index >= len(snapshots) {
		return
	}
	snap := snapshots[index]
	mutate(&snap)
	snapshots[index] = snap
}

func normalizeTableInput(input *TableInput) error {
	input.Key = strings.TrimSpace(input.Key)
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return errors.New("test data table name is required")
	}
	if input.Key == "" {
		return errors.New("test data table key is required")
	}
	if !tableKeyPattern.MatchString(input.Key) {
		return errors.New("test data table key may only contain letters, numbers, underscore or hyphen, up to 64 characters")
	}
	if input.Columns == nil {
		input.Columns = []ColumnDef{}
	}
	keysSeen := map[string]struct{}{}
	for index := range input.Columns {
		column := &input.Columns[index]
		column.Key = strings.TrimSpace(column.Key)
		column.Name = strings.TrimSpace(column.Name)
		if column.Name == "" {
			column.Name = column.Key
		}
		if column.Key == "" {
			return fmt.Errorf("%w: column[%d] key is required", ErrInvalidColumn, index)
		}
		if !columnKeyPattern.MatchString(column.Key) {
			return fmt.Errorf("%w: column %q key must start with a letter and contain only letters, digits or underscore", ErrInvalidColumn, column.Key)
		}
		if _, ok := keysSeen[column.Key]; ok {
			return fmt.Errorf("%w: column key %q is duplicated", ErrInvalidColumn, column.Key)
		}
		keysSeen[column.Key] = struct{}{}
		column.Type = strings.TrimSpace(column.Type)
		if column.Type == "" {
			column.Type = ColumnTypeString
		}
		if _, ok := supportedColumnTypes[column.Type]; !ok {
			return fmt.Errorf("%w: column %q has unsupported type %q", ErrInvalidColumn, column.Key, column.Type)
		}
		if err := normalizeGenerator(column); err != nil {
			return err
		}
	}
	return nil
}

func normalizeGenerator(column *ColumnDef) error {
	column.Generator.Kind = strings.TrimSpace(column.Generator.Kind)
	if column.Generator.Kind == "" {
		column.Generator.Kind = KindManual
	}
	switch column.Generator.Kind {
	case KindManual:
		column.Generator.Mock = nil
		column.Generator.AI = nil
	case KindMock:
		if column.Generator.Mock == nil {
			return fmt.Errorf("%w: column %q mock generator is required", ErrInvalidGenerator, column.Key)
		}
		column.Generator.Mock.Helper = strings.TrimSpace(column.Generator.Mock.Helper)
		if column.Generator.Mock.Helper == "" {
			return fmt.Errorf("%w: column %q mock helper is required", ErrInvalidGenerator, column.Key)
		}
		if _, ok := mockdata.MustHelperInfo(column.Generator.Mock.Helper); !ok {
			return fmt.Errorf("%w: column %q mock helper %q is unknown", ErrUnknownMockHelper, column.Key, column.Generator.Mock.Helper)
		}
		column.Generator.AI = nil
	case KindAI:
		if column.Generator.AI == nil {
			column.Generator.AI = &AIGenerator{}
		}
		column.Generator.Mock = nil
	default:
		return fmt.Errorf("%w: column %q kind %q", ErrInvalidGenerator, column.Key, column.Generator.Kind)
	}
	return nil
}

