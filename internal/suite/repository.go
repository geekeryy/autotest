package suite

import (
	"context"
	"fmt"
	"strings"

	"autotest/internal/store"

	"github.com/google/uuid"
)

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

func (r *Repository) Create(ctx context.Context, input CreateSuiteInput) (*Suite, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if input.Kind == "" {
		input.Kind = KindManual
	}

	row := r.DB.QueryRow(ctx, `
		insert into test_suites (project_id, service_id, name, kind, description)
		select p.id, s.id, $3, $4, $5
		from projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, service_id, name, kind, description, created_at, updated_at
	`, input.ProjectID, input.ServiceID, input.Name, input.Kind, input.Description)

	var suite Suite
	if err := row.Scan(&suite.ID, &suite.ProjectID, &suite.ServiceID, &suite.Name, &suite.Kind, &suite.Description, &suite.CreatedAt, &suite.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create suite: %w", err)
	}
	return &suite, nil
}

func (r *Repository) List(ctx context.Context, filter ListFilter) ([]Suite, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	query := `
		select ts.id, ts.project_id, ts.service_id, ts.name, ts.kind, ts.description, ts.created_at, ts.updated_at
		from test_suites ts
		join projects p on p.id = ts.project_id and p.deleted_at is null
		join services s on s.id = ts.service_id and s.deleted_at is null
	`
	var args []any
	where := []string{"ts.deleted_at is null"}
	if filter.ProjectID != uuid.Nil {
		args = append(args, filter.ProjectID)
		where = append(where, fmt.Sprintf("ts.project_id = $%d", len(args)))
	}
	if filter.ServiceID != uuid.Nil {
		args = append(args, filter.ServiceID)
		where = append(where, fmt.Sprintf("ts.service_id = $%d", len(args)))
	}
	query += " where " + strings.Join(where, " and ")
	query += " order by ts.created_at desc"

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list suites: %w", err)
	}
	defer rows.Close()

	var suites []Suite
	for rows.Next() {
		var suite Suite
		if err := rows.Scan(&suite.ID, &suite.ProjectID, &suite.ServiceID, &suite.Name, &suite.Kind, &suite.Description, &suite.CreatedAt, &suite.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan suite: %w", err)
		}
		suites = append(suites, suite)
	}
	return suites, rows.Err()
}

func (r *Repository) AddItem(ctx context.Context, suiteID uuid.UUID, input AddItemInput) (*Item, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if input.Source == "" {
		input.Source = ItemManualAdded
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	row := r.DB.QueryRow(ctx, `
		insert into test_suite_items (suite_id, test_case_id, item_order, source, enabled)
		select ts.id, tc.id, $3, $4, $5
		from test_suites ts
		join test_cases tc on tc.id = $2 and tc.project_id = ts.project_id and tc.deleted_at is null
		join projects p on p.id = ts.project_id and p.deleted_at is null
		join services s on s.id = ts.service_id and s.deleted_at is null
		where ts.id = $1 and ts.deleted_at is null
		returning id, suite_id, test_case_id, item_order, source, enabled, created_at, updated_at
	`, suiteID, input.TestCaseID, input.ItemOrder, input.Source, enabled)

	var item Item
	if err := row.Scan(&item.ID, &item.SuiteID, &item.TestCaseID, &item.ItemOrder, &item.Source, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, fmt.Errorf("add suite item: %w", err)
	}
	return &item, nil
}

func (r *Repository) ListItems(ctx context.Context, suiteID uuid.UUID) ([]Item, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	rows, err := r.DB.Query(ctx, `
		select ti.id, ti.suite_id, ti.test_case_id, ti.item_order, ti.source, ti.enabled, ti.created_at, ti.updated_at
		from test_suite_items ti
		join test_suites ts on ts.id = ti.suite_id and ts.deleted_at is null
		join test_cases tc on tc.id = ti.test_case_id and tc.deleted_at is null
		where ti.suite_id = $1 and ti.deleted_at is null
		order by ti.item_order asc, ti.created_at asc
	`, suiteID)
	if err != nil {
		return nil, fmt.Errorf("list suite items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.SuiteID, &item.TestCaseID, &item.ItemOrder, &item.Source, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan suite item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
