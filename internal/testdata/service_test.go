package testdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"autotest/internal/aiprovider"
	"autotest/internal/projectprompt"

	"github.com/google/uuid"
)

func TestGenerateRowsManualPreservesExistingValues(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID:        tableID,
		ProjectID: projectID,
		Key:       "users",
		Name:      "Users",
		Columns: []ColumnDef{
			{Key: "name", Generator: Generator{Kind: KindManual}},
		},
	}
	repo.rows[tableID] = []Row{
		{ID: uuid.New(), TableID: tableID, RowIndex: 0, Values: map[string]string{"name": "Alice"}},
	}

	svc := newServiceForTests(repo, nil, nil)
	out, err := svc.GenerateRows(context.Background(), projectID, tableID, GenerateRowsInput{RowCount: 3, PreserveManual: true})
	if err != nil {
		t.Fatalf("generate rows: %v", err)
	}
	if len(out.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(out.Rows))
	}
	if out.Rows[0].Values["name"] != "Alice" {
		t.Fatalf("expected first row name preserved, got %#v", out.Rows[0].Values)
	}
	if out.Rows[1].Values["name"] != "" {
		t.Fatalf("expected empty manual cell, got %#v", out.Rows[1].Values)
	}
}

func TestGenerateRowsMockColumnFillsEachRow(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID:        tableID,
		ProjectID: projectID,
		Key:       "users",
		Name:      "Users",
		Columns: []ColumnDef{
			{Key: "name", Generator: Generator{Kind: KindMock, Mock: &MockGenerator{Helper: "string", Args: []string{"12"}}}},
		},
	}

	svc := newServiceForTests(repo, nil, nil)
	out, err := svc.GenerateRows(context.Background(), projectID, tableID, GenerateRowsInput{RowCount: 5})
	if err != nil {
		t.Fatalf("generate rows: %v", err)
	}
	if len(out.Rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(out.Rows))
	}
	seen := map[string]int{}
	for index, row := range out.Rows {
		value := row.Values["name"]
		if strings.TrimSpace(value) == "" {
			t.Fatalf("expected non-empty value for row %d, got %q", index, value)
		}
		if len([]rune(value)) != 12 {
			t.Fatalf("expected helper string(12) length 12, got %q (%d)", value, len([]rune(value)))
		}
		seen[value]++
	}
	if len(seen) < 2 {
		t.Fatalf("expected mock helper to produce diverse values across 5 rows, got %#v", seen)
	}
}

func TestGenerateRowsMockColumnPreservesManualEdits(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID:        tableID,
		ProjectID: projectID,
		Key:       "users",
		Name:      "Users",
		Columns: []ColumnDef{
			{Key: "name", Generator: Generator{Kind: KindMock, Mock: &MockGenerator{Helper: "string", Args: []string{"5"}}}},
		},
	}
	repo.rows[tableID] = []Row{
		{ID: uuid.New(), TableID: tableID, RowIndex: 0, Values: map[string]string{"name": "manual-value"}},
	}

	svc := newServiceForTests(repo, nil, nil)
	out, err := svc.GenerateRows(context.Background(), projectID, tableID, GenerateRowsInput{RowCount: 3, PreserveManual: true})
	if err != nil {
		t.Fatalf("generate rows: %v", err)
	}
	if out.Rows[0].Values["name"] != "manual-value" {
		t.Fatalf("expected manual value preserved, got %q", out.Rows[0].Values["name"])
	}
	if out.Rows[1].Values["name"] == "" {
		t.Fatalf("expected mock generated value for row 1, got empty")
	}
}

// TestGenerateRowsAIColumnUsesPromptCfgProvider 验证 AI 列在调用 Chat 时使用了
// project prompt cfg 的 ProviderID / DefaultModel / SystemPrompt，并通过 action
// `generate_case_data` 与 fake 服务对接。
func TestGenerateRowsAIColumnUsesPromptCfgProvider(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	cfgProviderID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID:        tableID,
		ProjectID: projectID,
		Key:       "users",
		Name:      "Users",
		Columns: []ColumnDef{
			{Key: "email", Generator: Generator{Kind: KindAI, AI: &AIGenerator{}}},
		},
	}
	ai := &fakeAIService{response: &aiprovider.ChatResponse{
		Parsed: json.RawMessage(`{"cases":[{"email":"alice@example.test"},{"email":"bob@example.test"},{"email":"carol@example.test"}]}`),
	}}
	prompt := &fakePromptService{prompts: map[string]*projectprompt.ProjectPrompt{
		projectprompt.ActionGenerateCaseData: {
			ProviderID:   &cfgProviderID,
			DefaultModel: "deepseek-chat",
			SystemPrompt: "你是一名测试数据生成器，请只输出 JSON。",
			Enabled:      true,
		},
	}}

	svc := newServiceForTests(repo, ai, prompt)
	out, err := svc.GenerateRows(context.Background(), projectID, tableID, GenerateRowsInput{RowCount: 3})
	if err != nil {
		t.Fatalf("generate rows: %v", err)
	}
	if len(out.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(out.Rows))
	}
	want := []string{"alice@example.test", "bob@example.test", "carol@example.test"}
	for index, row := range out.Rows {
		if row.Values["email"] != want[index] {
			t.Fatalf("expected email[%d]=%q, got %q", index, want[index], row.Values["email"])
		}
	}
	if ai.calls != 1 {
		t.Fatalf("expected single AI call for column, got %d", ai.calls)
	}
	if len(ai.requests) == 0 {
		t.Fatalf("missing recorded AI request")
	}
	got := ai.requests[0]
	if got.ProviderID != cfgProviderID {
		t.Fatalf("expected providerId from prompt cfg, got %v", got.ProviderID)
	}
	if got.Model != "deepseek-chat" {
		t.Fatalf("expected model from prompt cfg, got %q", got.Model)
	}
	if got.Action != aiprovider.ActionGenerateCaseData {
		t.Fatalf("expected action %q, got %q", aiprovider.ActionGenerateCaseData, got.Action)
	}
	if got.SystemPromptOverride == "" {
		t.Fatalf("expected SystemPromptOverride from prompt cfg, got empty")
	}
}

// TestGenerateRowsAIColumnFallsBackToDefaultProvider 验证 cfg 缺失或 cfg.ProviderID
// 为空时，service 会回退到项目内 IsDefault=true && Enabled=true 的 provider。
func TestGenerateRowsAIColumnFallsBackToDefaultProvider(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	defaultProviderID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID:        tableID,
		ProjectID: projectID,
		Key:       "users",
		Name:      "Users",
		Columns: []ColumnDef{
			{Key: "name", Generator: Generator{Kind: KindAI, AI: &AIGenerator{}}},
		},
	}
	ai := &fakeAIService{
		response: &aiprovider.ChatResponse{Parsed: json.RawMessage(`[{"name":"Eve"},{"name":"Frank"}]`)},
		providers: []aiprovider.Provider{
			{ID: uuid.New(), Enabled: true, IsDefault: false},
			{ID: defaultProviderID, Enabled: true, IsDefault: true},
		},
	}
	svc := newServiceForTests(repo, ai, &fakePromptService{})
	out, err := svc.GenerateRows(context.Background(), projectID, tableID, GenerateRowsInput{RowCount: 2})
	if err != nil {
		t.Fatalf("generate rows: %v", err)
	}
	if out.Rows[0].Values["name"] != "Eve" || out.Rows[1].Values["name"] != "Frank" {
		t.Fatalf("expected AI values to be applied, got %#v", out.Rows)
	}
	if len(ai.requests) == 0 {
		t.Fatalf("missing recorded AI request")
	}
	if ai.requests[0].ProviderID != defaultProviderID {
		t.Fatalf("expected fallback to default provider %v, got %v", defaultProviderID, ai.requests[0].ProviderID)
	}
	if ai.requests[0].SystemPromptOverride != "" {
		t.Fatalf("expected empty SystemPromptOverride when cfg missing, got %q", ai.requests[0].SystemPromptOverride)
	}
}

// TestGenerateRowsAIColumnWarnsWhenNoProviderConfigured 验证当 prompt 与默认
// provider 都不存在时返回中文 warning（保留其他列结果）。
func TestGenerateRowsAIColumnWarnsWhenNoProviderConfigured(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID:        tableID,
		ProjectID: projectID,
		Key:       "users",
		Name:      "Users",
		Columns: []ColumnDef{
			{Key: "name", Generator: Generator{Kind: KindAI, AI: &AIGenerator{}}},
		},
	}
	ai := &fakeAIService{providers: []aiprovider.Provider{}}
	svc := newServiceForTests(repo, ai, &fakePromptService{})
	out, err := svc.GenerateRows(context.Background(), projectID, tableID, GenerateRowsInput{RowCount: 2})
	if err != nil {
		t.Fatalf("generate rows: %v", err)
	}
	if len(out.Warnings) == 0 || !strings.Contains(out.Warnings[0], "请先在「AI 能力管理") {
		t.Fatalf("expected unconfigured warning, got %#v", out.Warnings)
	}
	if ai.calls != 0 {
		t.Fatalf("expected no AI call when provider unconfigured, got %d", ai.calls)
	}
	for _, row := range out.Rows {
		if row.Values["name"] != "" {
			t.Fatalf("expected empty cell when provider unconfigured, got %q", row.Values["name"])
		}
	}
}

func TestGenerateRowsAIColumnReturnsWarningOnError(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	defaultProviderID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID: tableID, ProjectID: projectID, Key: "users", Name: "Users",
		Columns: []ColumnDef{{Key: "name", Generator: Generator{Kind: KindAI, AI: &AIGenerator{}}}},
	}
	ai := &fakeAIService{
		err:       errors.New("boom"),
		providers: []aiprovider.Provider{{ID: defaultProviderID, Enabled: true, IsDefault: true}},
	}
	svc := newServiceForTests(repo, ai, &fakePromptService{})
	out, err := svc.GenerateRows(context.Background(), projectID, tableID, GenerateRowsInput{RowCount: 2})
	if err != nil {
		t.Fatalf("generate rows: %v", err)
	}
	if len(out.Warnings) == 0 || !strings.Contains(out.Warnings[0], "ai generation failed") {
		t.Fatalf("expected warning when AI fails, got %#v", out.Warnings)
	}
	for _, row := range out.Rows {
		if row.Values["name"] != "" {
			t.Fatalf("expected empty name when AI failed, got %q", row.Values["name"])
		}
	}
}

func TestGenerateRowsAIColumnRequiresProviderService(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID: tableID, ProjectID: projectID, Key: "users", Name: "Users",
		Columns: []ColumnDef{{Key: "name", Generator: Generator{Kind: KindAI, AI: &AIGenerator{}}}},
	}
	svc := newServiceForTests(repo, nil, nil)
	out, err := svc.GenerateRows(context.Background(), projectID, tableID, GenerateRowsInput{RowCount: 1})
	if err != nil {
		t.Fatalf("generate rows: %v", err)
	}
	if len(out.Warnings) == 0 || !strings.Contains(out.Warnings[0], "ai provider service is unavailable") {
		t.Fatalf("expected warning when AI service missing, got %#v", out.Warnings)
	}
}

func TestRegenerateCellsRunsGeneratorForRequestedColumnsOnly(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	repo := newFakeRepository()
	rowID := uuid.New()
	repo.tables[tableID] = &Table{
		ID: tableID, ProjectID: projectID, Key: "users", Name: "Users",
		Columns: []ColumnDef{
			{Key: "name", Generator: Generator{Kind: KindMock, Mock: &MockGenerator{Helper: "string", Args: []string{"6"}}}},
			{Key: "email", Generator: Generator{Kind: KindManual}},
		},
	}
	repo.rows[tableID] = []Row{{ID: rowID, TableID: tableID, RowIndex: 0, Values: map[string]string{"name": "old", "email": "kept@example.test"}}}

	svc := newServiceForTests(repo, nil, nil)
	row, err := svc.RegenerateCells(context.Background(), projectID, tableID, rowID, RegenerateCellsInput{ColumnKeys: []string{"name"}})
	if err != nil {
		t.Fatalf("regenerate cells: %v", err)
	}
	if row.Values["name"] == "old" {
		t.Fatalf("expected name to be regenerated, got %q", row.Values["name"])
	}
	if row.Values["email"] != "kept@example.test" {
		t.Fatalf("expected email to be untouched, got %q", row.Values["email"])
	}
}

// TestRegenerateCellsAIColumnUsesPromptCfg 验证 RegenerateCells AI 列同样使用
// projectprompt cfg / 默认 provider 的解析路径。
func TestRegenerateCellsAIColumnUsesPromptCfg(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	rowID := uuid.New()
	cfgProviderID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID: tableID, ProjectID: projectID, Key: "users", Name: "Users",
		Columns: []ColumnDef{
			{Key: "email", Generator: Generator{Kind: KindAI, AI: &AIGenerator{}}},
		},
	}
	repo.rows[tableID] = []Row{{ID: rowID, TableID: tableID, RowIndex: 0, Values: map[string]string{"email": ""}}}

	ai := &fakeAIService{response: &aiprovider.ChatResponse{
		Parsed: json.RawMessage(`{"cases":[{"email":"new@example.test"}]}`),
	}}
	prompt := &fakePromptService{prompts: map[string]*projectprompt.ProjectPrompt{
		projectprompt.ActionGenerateCaseData: {
			ProviderID:   &cfgProviderID,
			DefaultModel: "deepseek-chat",
			Enabled:      true,
		},
	}}

	svc := newServiceForTests(repo, ai, prompt)
	row, err := svc.RegenerateCells(context.Background(), projectID, tableID, rowID, RegenerateCellsInput{ColumnKeys: []string{"email"}})
	if err != nil {
		t.Fatalf("regenerate cells: %v", err)
	}
	if row.Values["email"] != "new@example.test" {
		t.Fatalf("expected regenerated email, got %q", row.Values["email"])
	}
	if ai.requests[0].ProviderID != cfgProviderID {
		t.Fatalf("expected provider from cfg, got %v", ai.requests[0].ProviderID)
	}
}

// TestRegenerateCellsAIColumnErrorsWhenUnconfigured 验证 prompt 与默认 provider
// 都缺失时，RegenerateCells 返回中文错误信息。
func TestRegenerateCellsAIColumnErrorsWhenUnconfigured(t *testing.T) {
	t.Parallel()

	tableID := uuid.New()
	projectID := uuid.New()
	rowID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID: tableID, ProjectID: projectID, Key: "users", Name: "Users",
		Columns: []ColumnDef{
			{Key: "email", Generator: Generator{Kind: KindAI, AI: &AIGenerator{}}},
		},
	}
	repo.rows[tableID] = []Row{{ID: rowID, TableID: tableID, RowIndex: 0, Values: map[string]string{"email": "a"}}}

	ai := &fakeAIService{providers: []aiprovider.Provider{}}
	svc := newServiceForTests(repo, ai, &fakePromptService{})
	_, err := svc.RegenerateCells(context.Background(), projectID, tableID, rowID, RegenerateCellsInput{ColumnKeys: []string{"email"}})
	if err == nil || !errors.Is(err, ErrAIProviderUnconfigured) {
		t.Fatalf("expected ErrAIProviderUnconfigured, got %v", err)
	}
}

func TestResolveLooksUpRowsByKeyAndFilter(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	tableID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID: tableID, ProjectID: projectID, Key: "users", Name: "Users",
		Columns: []ColumnDef{{Key: "name", Generator: Generator{Kind: KindManual}}, {Key: "role", Generator: Generator{Kind: KindManual}}},
	}
	repo.rows[tableID] = []Row{
		{ID: uuid.New(), TableID: tableID, RowIndex: 0, Values: map[string]string{"name": "Alice", "role": "admin"}},
		{ID: uuid.New(), TableID: tableID, RowIndex: 1, Values: map[string]string{"name": "Bob", "role": "viewer"}},
	}

	svc := newServiceForTests(repo, nil, nil)
	resolved, snapshots, err := svc.Resolve(context.Background(), projectID, []InlineReference{
		{Expression: "$ds.users.name", TableKey: "users", Column: "name"},
		{Expression: "$ds.users[role=viewer].name", TableKey: "users", FilterColumn: "role", FilterValue: "viewer", Column: "name"},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved["$ds.users.name"] != "Alice" {
		t.Fatalf("expected first row resolved, got %#v", resolved)
	}
	if resolved["$ds.users[role=viewer].name"] != "Bob" {
		t.Fatalf("expected filtered row resolved, got %#v", resolved)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected single snapshot per table, got %d", len(snapshots))
	}
}

func TestResolveErrorsWhenTableMissing(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := newServiceForTests(repo, nil, nil)
	_, _, err := svc.Resolve(context.Background(), uuid.New(), []InlineReference{
		{Expression: "$ds.unknown.name", TableKey: "unknown", Column: "name"},
	})
	if err == nil || !strings.Contains(err.Error(), "test data table \"unknown\" not found") {
		t.Fatalf("expected missing table error, got %v", err)
	}
}

func TestResolveErrorsWhenColumnMissing(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	tableID := uuid.New()
	repo := newFakeRepository()
	repo.tables[tableID] = &Table{
		ID: tableID, ProjectID: projectID, Key: "users", Name: "Users",
		Columns: []ColumnDef{{Key: "name", Generator: Generator{Kind: KindManual}}},
	}
	repo.rows[tableID] = []Row{{ID: uuid.New(), TableID: tableID, RowIndex: 0, Values: map[string]string{"name": "Alice"}}}

	svc := newServiceForTests(repo, nil, nil)
	_, _, err := svc.Resolve(context.Background(), projectID, []InlineReference{
		{Expression: "$ds.users.email", TableKey: "users", Column: "email"},
	})
	if err == nil || !strings.Contains(err.Error(), "column \"email\" not found") {
		t.Fatalf("expected missing column error, got %v", err)
	}
}

func TestNormalizeTableInputValidatesKeysAndGenerators(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   TableInput
		wantErr string
	}{
		{
			name:    "missing name",
			input:   TableInput{Key: "users"},
			wantErr: "name is required",
		},
		{
			name:    "missing key",
			input:   TableInput{Name: "Users"},
			wantErr: "key is required",
		},
		{
			name:    "invalid key",
			input:   TableInput{Key: "users!", Name: "Users"},
			wantErr: "letters, numbers",
		},
		{
			name: "duplicate column",
			input: TableInput{Key: "users", Name: "Users", Columns: []ColumnDef{
				{Key: "name", Generator: Generator{Kind: KindManual}},
				{Key: "name", Generator: Generator{Kind: KindManual}},
			}},
			wantErr: "duplicated",
		},
		{
			name: "unknown column type",
			input: TableInput{Key: "users", Name: "Users", Columns: []ColumnDef{
				{Key: "name", Type: "blob", Generator: Generator{Kind: KindManual}},
			}},
			wantErr: "unsupported type",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := tc.input
			err := normalizeTableInput(&input)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestNormalizeTableInputAcceptsLegacyAIPayload 验证旧版 AI 列保存时携带
// providerId/prompt/model 字段，反序列化与 normalize 不应报错并保留 KindAI。
func TestNormalizeTableInputAcceptsLegacyAIPayload(t *testing.T) {
	t.Parallel()
	rawColumn := `{"key":"email","name":"email","type":"string","required":false,` +
		`"generator":{"kind":"ai","ai":{"providerId":"00000000-0000-0000-0000-000000000001",` +
		`"prompt":"legacy","model":"legacy-model"}}}`
	var col ColumnDef
	if err := json.Unmarshal([]byte(rawColumn), &col); err != nil {
		t.Fatalf("unmarshal legacy column: %v", err)
	}
	input := TableInput{Key: "users", Name: "Users", Columns: []ColumnDef{col}}
	if err := normalizeTableInput(&input); err != nil {
		t.Fatalf("expected legacy AI payload to be accepted, got %v", err)
	}
	if input.Columns[0].Generator.Kind != KindAI {
		t.Fatalf("expected kind=ai preserved, got %q", input.Columns[0].Generator.Kind)
	}
}

// fakeRepository is an in-memory stand-in for Repository used by service tests.
type fakeRepository struct {
	tables map[uuid.UUID]*Table
	rows   map[uuid.UUID][]Row
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		tables: map[uuid.UUID]*Table{},
		rows:   map[uuid.UUID][]Row{},
	}
}

func (f *fakeRepository) ListTables(_ context.Context, projectID uuid.UUID) ([]Table, error) {
	out := []Table{}
	for _, table := range f.tables {
		if table.ProjectID == projectID {
			out = append(out, *table)
		}
	}
	return out, nil
}

func (f *fakeRepository) GetTable(_ context.Context, projectID, tableID uuid.UUID) (*Table, error) {
	table, ok := f.tables[tableID]
	if !ok || table.ProjectID != projectID {
		return nil, ErrTableNotFound
	}
	clone := *table
	clone.Columns = append([]ColumnDef(nil), table.Columns...)
	return &clone, nil
}

func (f *fakeRepository) CreateTable(_ context.Context, projectID uuid.UUID, input TableInput) (*Table, error) {
	for _, table := range f.tables {
		if table.ProjectID == projectID && table.Key == input.Key {
			return nil, ErrTableKeyConflict
		}
	}
	id := uuid.New()
	now := time.Now()
	table := &Table{
		ID:          id,
		ProjectID:   projectID,
		Key:         input.Key,
		Name:        input.Name,
		Description: input.Description,
		Columns:     input.Columns,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	f.tables[id] = table
	return table, nil
}

func (f *fakeRepository) UpdateTable(_ context.Context, projectID, tableID uuid.UUID, input TableInput) (*Table, error) {
	table, ok := f.tables[tableID]
	if !ok || table.ProjectID != projectID {
		return nil, ErrTableNotFound
	}
	table.Key = input.Key
	table.Name = input.Name
	table.Description = input.Description
	table.Columns = input.Columns
	table.UpdatedAt = time.Now()
	return table, nil
}

func (f *fakeRepository) DeleteTable(_ context.Context, projectID, tableID uuid.UUID) error {
	table, ok := f.tables[tableID]
	if !ok || table.ProjectID != projectID {
		return ErrTableNotFound
	}
	delete(f.tables, tableID)
	delete(f.rows, tableID)
	return nil
}

func (f *fakeRepository) ListRows(_ context.Context, tableID uuid.UUID) ([]Row, error) {
	rows := append([]Row(nil), f.rows[tableID]...)
	return rows, nil
}

func (f *fakeRepository) GetRow(_ context.Context, tableID, rowID uuid.UUID) (*Row, error) {
	for _, row := range f.rows[tableID] {
		if row.ID == rowID {
			clone := row
			clone.Values = copyStringMap(row.Values)
			return &clone, nil
		}
	}
	return nil, ErrRowNotFound
}

func (f *fakeRepository) ReplaceRows(_ context.Context, tableID uuid.UUID, rows []RowInput) ([]Row, error) {
	stored := make([]Row, 0, len(rows))
	for index, input := range rows {
		stored = append(stored, Row{
			ID:        uuid.New(),
			TableID:   tableID,
			RowIndex:  index,
			Values:    copyStringMap(input.Values),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}
	f.rows[tableID] = stored
	out := append([]Row(nil), stored...)
	return out, nil
}

func (f *fakeRepository) UpdateRowValues(_ context.Context, tableID, rowID uuid.UUID, values map[string]string) (*Row, error) {
	rows := f.rows[tableID]
	for index := range rows {
		if rows[index].ID == rowID {
			rows[index].Values = copyStringMap(values)
			rows[index].UpdatedAt = time.Now()
			f.rows[tableID] = rows
			clone := rows[index]
			clone.Values = copyStringMap(rows[index].Values)
			return &clone, nil
		}
	}
	return nil, ErrRowNotFound
}

func (f *fakeRepository) ListAllForProject(_ context.Context, projectID uuid.UUID) ([]TableWithRows, error) {
	out := []TableWithRows{}
	for _, table := range f.tables {
		if table.ProjectID != projectID {
			continue
		}
		clone := *table
		clone.Columns = append([]ColumnDef(nil), table.Columns...)
		out = append(out, TableWithRows{Table: clone, Rows: append([]Row(nil), f.rows[table.ID]...)})
	}
	return out, nil
}

type fakeAIService struct {
	response  *aiprovider.ChatResponse
	err       error
	providers []aiprovider.Provider
	calls     int
	requests  []aiprovider.ChatRequest
}

func (f *fakeAIService) Chat(_ context.Context, _ uuid.UUID, req aiprovider.ChatRequest) (*aiprovider.ChatResponse, error) {
	f.calls++
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	if f.response == nil {
		return nil, fmt.Errorf("fakeAIService.response is not set")
	}
	return f.response, nil
}

func (f *fakeAIService) List(_ context.Context) ([]aiprovider.Provider, error) {
	return append([]aiprovider.Provider(nil), f.providers...), nil
}

type fakePromptService struct {
	prompts map[string]*projectprompt.ProjectPrompt
	err     error
}

func (f *fakePromptService) GetByAction(_ context.Context, action string) (*projectprompt.ProjectPrompt, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.prompts == nil {
		return nil, projectprompt.ErrNotFound
	}
	p, ok := f.prompts[action]
	if !ok {
		return nil, projectprompt.ErrNotFound
	}
	return p, nil
}
