package paramsource

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestValueStringPostgresUUIDBytes(t *testing.T) {
	t.Parallel()

	// Matches pgx/pgtype.UUIDCodec.DecodeValue: uuid columns become [16]byte.
	raw := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	got := valueString(raw)
	want := "00000000-0000-0000-0000-000000000001"
	if got != want {
		t.Fatalf("valueString([16]byte): got %q want %q", got, want)
	}

	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	if valueString(id) != id.String() {
		t.Fatalf("valueString(uuid.UUID): got %q want %q", valueString(id), id.String())
	}
}

func TestNormalizeReadOnlySQL(t *testing.T) {
	t.Parallel()

	allowed := []string{
		"select id from users",
		"  -- comment\nselect id from users;",
		"with latest as (select id from users) select id from latest",
	}
	for _, sqlText := range allowed {
		if _, err := normalizeReadOnlySQL(sqlText); err != nil {
			t.Fatalf("expected %q to be allowed: %v", sqlText, err)
		}
	}

	rejected := []string{
		"delete from users",
		"update users set name = 'x'",
		"select id from users; select id from admins",
		"with deleted as (delete from users returning id) select id from deleted",
	}
	for _, sqlText := range rejected {
		if _, err := normalizeReadOnlySQL(sqlText); err == nil {
			t.Fatalf("expected %q to be rejected", sqlText)
		}
	}
}

func TestNormalizeSQLStepOperation(t *testing.T) {
	t.Parallel()

	allowed := []string{
		"select id from users",
		"insert into users(name) values ($1)",
		"update users set name = $1 where id = $2",
		"delete from users where id = $1",
	}
	for _, sqlText := range allowed {
		if _, err := normalizeSQLStepOperation(sqlText); err != nil {
			t.Fatalf("expected %q to be allowed: %v", sqlText, err)
		}
	}

	rejected := []string{
		"drop table users",
		"alter table users add column x text",
		"select id from users; delete from users",
	}
	for _, sqlText := range rejected {
		if _, err := normalizeSQLStepOperation(sqlText); err == nil {
			t.Fatalf("expected %q to be rejected", sqlText)
		}
	}
}

func TestBuildArgsRendersVariablesWithoutChangingSQL(t *testing.T) {
	t.Parallel()

	args, snapshot, err := buildArgs(json.RawMessage(`[
		{"name":"userId","required":true},
		{"name":"tenant","value":"tenant-{{tenantId}}"}
	]`), map[string]string{"userId": "42", "tenantId": "acme"})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}
	if len(args) != 2 || args[0] != "42" || args[1] != "tenant-acme" {
		t.Fatalf("unexpected args: %#v", args)
	}
	if snapshot["userId"] != "42" || snapshot["tenant"] != "tenant-acme" {
		t.Fatalf("unexpected input snapshot: %#v", snapshot)
	}
}

func TestBuildArgsSingleKeyShorthand(t *testing.T) {
	t.Parallel()

	stepVar := "$steps[2].body.id"
	args, snapshot, err := buildArgs(json.RawMessage(`[
		{"id":"{{`+stepVar+`}}"}
	]`), map[string]string{stepVar: "550e8400-e29b-41d4-a716-446655440000"})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}
	if len(args) != 1 || args[0] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected args: %#v", args)
	}
	if snapshot["id"] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected input snapshot: %#v", snapshot)
	}

	_, _, err = buildArgs(json.RawMessage(`[{"id":"a","id2":"b"}]`), nil)
	if err == nil {
		t.Fatal("expected error for ambiguous multi-key shorthand")
	}
}

func TestBuildArgsPositionalScalarValues(t *testing.T) {
	t.Parallel()

	stepVar := "$steps[2].body.data.id"
	args, snapshot, err := buildArgs(json.RawMessage(`[
		"{{`+stepVar+`}}",
		123,
		true
	]`), map[string]string{stepVar: "550e8400-e29b-41d4-a716-446655440000"})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}
	if len(args) != 3 || args[0] != "550e8400-e29b-41d4-a716-446655440000" || args[1] != "123" || args[2] != "true" {
		t.Fatalf("unexpected args: %#v", args)
	}
	if snapshot["$1"] != "550e8400-e29b-41d4-a716-446655440000" || snapshot["$2"] != "123" || snapshot["$3"] != "true" {
		t.Fatalf("unexpected input snapshot: %#v", snapshot)
	}
}

func TestDataSourceInputUnmarshalAcceptsStringPorts(t *testing.T) {
	t.Parallel()

	var input DataSourceInput
	if err := json.Unmarshal([]byte(`{
		"name":"orders",
		"driver":"postgres",
		"host":"127.0.0.1",
		"port":"5432",
		"database":"biz",
		"username":"readonly",
		"config":{"port":"15432","connectTimeout":3}
	}`), &input); err != nil {
		t.Fatalf("unmarshal data source input: %v", err)
	}
	if input.Port != 5432 {
		t.Fatalf("expected top-level string port to win, got %d", input.Port)
	}
	if string(input.Options) == "" || !strings.Contains(string(input.Options), `"connectTimeout":3`) {
		t.Fatalf("expected config to be kept as options, got %s", string(input.Options))
	}
}

func TestDataSourceInputUnmarshalFallsBackToConfigPort(t *testing.T) {
	t.Parallel()

	var input DataSourceInput
	if err := json.Unmarshal([]byte(`{
		"name":"orders",
		"driver":"postgres",
		"host":"127.0.0.1",
		"database":"biz",
		"username":"readonly",
		"config":{"port":"15432"}
	}`), &input); err != nil {
		t.Fatalf("unmarshal data source input: %v", err)
	}
	if input.Port != 15432 {
		t.Fatalf("expected config string port, got %d", input.Port)
	}
}

func TestMapColumnsReturnsRawSQLColumns(t *testing.T) {
	t.Parallel()

	fields := []pgconn.FieldDescription{{Name: "id"}, {Name: "email"}}
	values := []any{int64(42), "u@example.test"}

	result := mapColumns(fields, values)
	if result["id"] != "42" || result["email"] != "u@example.test" {
		t.Fatalf("unexpected raw column result: %#v", result)
	}
}

func TestExecuteInlineReferencesResolvesFirstRowAndFilter(t *testing.T) {
	t.Parallel()

	source := SourceWithDataSource{Source: SQLParameterSource{ID: mustParseUUIDForTest("11111111-1111-1111-1111-111111111111"), Key: "userSeed", Name: "用户种子"}}
	var executed int
	vars, snapshots, err := executeInlineReferences(
		testContext(),
		[]InlineReference{
			{Expression: "sql.userSeed.id", SourceKey: "userSeed", Column: "id"},
			{Expression: "sql.userSeed[status=1].email", SourceKey: "userSeed", FilterColumn: "status", FilterValue: "1", Column: "email"},
		},
		map[string]string{"tenant": "acme"},
		[]SourceWithDataSource{source},
		func(_ context.Context, item SourceWithDataSource, input map[string]string) ([]map[string]string, ExecutionSnapshot, error) {
			executed++
			if item.Source.Key != "userSeed" || input["tenant"] != "acme" {
				t.Fatalf("unexpected execution input: %#v %#v", item.Source, input)
			}
			return []map[string]string{
				{"id": "10", "status": "0", "email": "inactive@example.test"},
				{"id": "20", "status": "1", "email": "active@example.test"},
			}, ExecutionSnapshot{SourceID: item.Source.ID, SourceName: item.Source.Name}, nil
		},
	)
	if err != nil {
		t.Fatalf("execute inline references: %v", err)
	}
	if executed != 1 {
		t.Fatalf("expected source to execute once, got %d", executed)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one execution snapshot, got %d", len(snapshots))
	}
	if vars["sql.userSeed.id"] != "10" {
		t.Fatalf("expected first row id, got %q", vars["sql.userSeed.id"])
	}
	if vars["sql.userSeed[status=1].email"] != "active@example.test" {
		t.Fatalf("expected filtered email, got %q", vars["sql.userSeed[status=1].email"])
	}
}

func TestExecuteInlineReferencesFallsBackToNameAndID(t *testing.T) {
	t.Parallel()

	sourceID := mustParseUUIDForTest("22222222-2222-2222-2222-222222222222")
	source := SourceWithDataSource{Source: SQLParameterSource{ID: sourceID, Name: "legacySource"}}
	vars, _, err := executeInlineReferences(
		testContext(),
		[]InlineReference{
			{Expression: "sql.legacySource.id", SourceKey: "legacySource", Column: "id"},
			{Expression: "sql." + sourceID.String() + ".id", SourceKey: sourceID.String(), Column: "id"},
		},
		nil,
		[]SourceWithDataSource{source},
		func(_ context.Context, _ SourceWithDataSource, _ map[string]string) ([]map[string]string, ExecutionSnapshot, error) {
			return []map[string]string{{"id": "42"}}, ExecutionSnapshot{SourceID: sourceID}, nil
		},
	)
	if err != nil {
		t.Fatalf("execute inline references: %v", err)
	}
	if vars["sql.legacySource.id"] != "42" || vars["sql."+sourceID.String()+".id"] != "42" {
		t.Fatalf("expected name and id fallbacks to resolve, got %#v", vars)
	}
}

func TestExecuteInlineReferencesReportsMissingRowAndColumn(t *testing.T) {
	t.Parallel()

	source := SourceWithDataSource{Source: SQLParameterSource{ID: mustParseUUIDForTest("33333333-3333-3333-3333-333333333333"), Key: "userSeed"}}
	execute := func(_ context.Context, _ SourceWithDataSource, _ map[string]string) ([]map[string]string, ExecutionSnapshot, error) {
		return []map[string]string{{"id": "42", "status": "0"}}, ExecutionSnapshot{SourceID: source.Source.ID}, nil
	}

	_, _, err := executeInlineReferences(testContext(), []InlineReference{
		{Expression: "sql.userSeed[status=1].id", SourceKey: "userSeed", FilterColumn: "status", FilterValue: "1", Column: "id"},
	}, nil, []SourceWithDataSource{source}, execute)
	if err == nil || !strings.Contains(err.Error(), "found no row") {
		t.Fatalf("expected missing row error, got %v", err)
	}

	_, _, err = executeInlineReferences(testContext(), []InlineReference{
		{Expression: "sql.userSeed.email", SourceKey: "userSeed", Column: "email"},
	}, nil, []SourceWithDataSource{source}, execute)
	if err == nil || !strings.Contains(err.Error(), "column \"email\" not found") {
		t.Fatalf("expected missing column error, got %v", err)
	}
}

func testContext() context.Context {
	return context.Background()
}

func mustParseUUIDForTest(raw string) uuid.UUID {
	id, err := uuid.Parse(raw)
	if err != nil {
		panic(err)
	}
	return id
}
