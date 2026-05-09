package runner

import (
	"context"
	"strings"
	"testing"

	"autotest/internal/testdata"

	"github.com/google/uuid"
)

func TestCollectInlineTestDataReferencesScansRequestRecursively(t *testing.T) {
	t.Parallel()

	refs, err := collectInlineTestDataReferences(RequestDefinition{
		Path:    "/api/v1/users/{{$ds.users.id}}",
		Query:   map[string]string{"role": "{{$ds.users.role}}"},
		Headers: map[string]string{"X-Tenant": "{{$ds.tenants[name=acme].code}}"},
		Variables: map[string]any{
			"createdBy": "{{$ds.actors.id}}",
		},
		Body: map[string]any{
			"user": map[string]any{
				"id":   "{{$ds.users.id}}",
				"tags": []any{"role-{{$ds.roles[name=admin].id}}"},
			},
		},
	})
	if err != nil {
		t.Fatalf("collect inline test data references: %v", err)
	}
	expressions := map[string]bool{}
	for _, ref := range refs {
		expressions[ref.Expression] = true
	}
	for _, want := range []string{
		"$ds.users.id",
		"$ds.users.role",
		"$ds.tenants[name=acme].code",
		"$ds.actors.id",
		"$ds.roles[name=admin].id",
	} {
		if !expressions[want] {
			t.Fatalf("expected inline reference %q in %#v", want, refs)
		}
	}
}

func TestCollectInlineTestDataReferencesScansRunVariables(t *testing.T) {
	t.Parallel()

	refs, err := collectInlineTestDataReferencesFromVariables(map[string]any{
		"id": "{{$ds.users.id}}",
		"payload": map[string]any{
			"role": "{{$ds.roles[name=admin].id}}",
		},
	})
	if err != nil {
		t.Fatalf("collect inline test data references from variables: %v", err)
	}
	expressions := map[string]bool{}
	for _, ref := range refs {
		expressions[ref.Expression] = true
	}
	for _, want := range []string{"$ds.users.id", "$ds.roles[name=admin].id"} {
		if !expressions[want] {
			t.Fatalf("expected variable inline reference %q in %#v", want, refs)
		}
	}
}

func TestCollectInlineTestDataReferencesReportsInvalidShape(t *testing.T) {
	t.Parallel()

	_, err := collectInlineTestDataReferences(RequestDefinition{
		Path: "/api/v1/users/{{$ds.users}}",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid test data inline reference") {
		t.Fatalf("expected invalid reference error, got %v", err)
	}
}

func TestApplyInlineTestDataReferencesPropagatesValuesAndSnapshots(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	tableID := uuid.New()
	rowID := uuid.New()
	fake := &fakeTestDataService{
		values: map[string]string{
			"$ds.users.email": "alice@example.test",
		},
		snapshots: []testdata.Snapshot{{
			TableID:   tableID,
			TableKey:  "users",
			TableName: "Users",
			RowID:     rowID,
			Result:    map[string]string{"email": "alice@example.test"},
		}},
	}
	service := &Service{testData: fake}
	variants, err := service.applyInlineTestDataReferences(
		context.Background(),
		projectID,
		[]testdata.InlineReference{{Expression: "$ds.users.email", TableKey: "users", Column: "email"}},
		[]caseRunVariant{{Variables: map[string]string{"tenant": "acme"}}},
	)
	if err != nil {
		t.Fatalf("apply inline test data references: %v", err)
	}
	if fake.projectID != projectID {
		t.Fatalf("expected project id forwarded, got %v", fake.projectID)
	}
	if variants[0].Variables["$ds.users.email"] != "alice@example.test" {
		t.Fatalf("expected resolved value injected, got %#v", variants[0].Variables)
	}
	if len(variants[0].TestDataSnapshots) != 1 {
		t.Fatalf("expected snapshot recorded, got %#v", variants[0].TestDataSnapshots)
	}
}

func TestApplyInlineTestDataReferencesRequiresService(t *testing.T) {
	t.Parallel()

	service := &Service{}
	_, err := service.applyInlineTestDataReferences(
		context.Background(),
		uuid.New(),
		[]testdata.InlineReference{{Expression: "$ds.users.email", TableKey: "users", Column: "email"}},
		[]caseRunVariant{{Variables: map[string]string{}}},
	)
	if err == nil || !strings.Contains(err.Error(), "test data service is unavailable") {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestApplyInlineTestDataReferencesAndSQLDoNotInterfere(t *testing.T) {
	t.Parallel()

	dsRefs, err := testdata.ParseInlineReferences(`/api/v1/users/{{$ds.users.id}}?token={{sql.userSeed.token}}`)
	if err != nil {
		t.Fatalf("parse ds refs: %v", err)
	}
	if len(dsRefs) != 1 || dsRefs[0].Expression != "$ds.users.id" {
		t.Fatalf("expected only ds reference, got %#v", dsRefs)
	}

	sqlRefs, err := parseInlineSQLReferences(`/api/v1/users/{{$ds.users.id}}?token={{sql.userSeed.token}}`)
	if err != nil {
		t.Fatalf("parse sql refs: %v", err)
	}
	if len(sqlRefs) != 1 || sqlRefs[0].Expression != "sql.userSeed.token" {
		t.Fatalf("expected only sql reference, got %#v", sqlRefs)
	}
}

type fakeTestDataService struct {
	projectID uuid.UUID
	values    map[string]string
	snapshots []testdata.Snapshot
	err       error
}

func (f *fakeTestDataService) Resolve(_ context.Context, projectID uuid.UUID, _ []testdata.InlineReference) (map[string]string, []testdata.Snapshot, error) {
	f.projectID = projectID
	if f.err != nil {
		return nil, f.snapshots, f.err
	}
	return f.values, f.snapshots, nil
}

var _ testDataService = (*fakeTestDataService)(nil)
