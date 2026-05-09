package testdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository persists test data tables and their rows.
type Repository struct {
	store.Repository
}

// NewRepository creates a Repository from the shared store repository.
func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

// CreateTable inserts a new table for the given project. Column definitions
// are stored as JSONB after validation by the service layer.
func (r *Repository) CreateTable(ctx context.Context, projectID uuid.UUID, input TableInput) (*Table, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	columns, err := encodeColumns(input.Columns)
	if err != nil {
		return nil, err
	}
	row := r.DB.QueryRow(ctx, `
		insert into test_data_tables (project_id, key, name, description, columns)
		select p.id, $2, $3, $4, $5::jsonb
		from projects p
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, key, name, description, columns, created_at, updated_at
	`, projectID, input.Key, input.Name, input.Description, columns)
	t, err := scanTable(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("project not found or inaccessible")
		}
		if isUniqueViolation(err) {
			return nil, ErrTableKeyConflict
		}
		return nil, fmt.Errorf("create test data table: %w", err)
	}
	return t, nil
}

// ListTables returns all active tables for a project (without rows).
func (r *Repository) ListTables(ctx context.Context, projectID uuid.UUID) ([]Table, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select t.id, t.project_id, t.key, t.name, t.description, t.columns, t.created_at, t.updated_at
		from test_data_tables t
		join projects p on p.id = t.project_id and p.deleted_at is null
		where t.project_id = $1 and t.deleted_at is null
		order by t.created_at asc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list test data tables: %w", err)
	}
	defer rows.Close()
	var tables []Table
	for rows.Next() {
		t, err := scanTable(rows)
		if err != nil {
			return nil, err
		}
		tables = append(tables, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if tables == nil {
		tables = []Table{}
	}
	return tables, nil
}

// GetTable fetches a single table by id, scoped by project.
func (r *Repository) GetTable(ctx context.Context, projectID, tableID uuid.UUID) (*Table, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select t.id, t.project_id, t.key, t.name, t.description, t.columns, t.created_at, t.updated_at
		from test_data_tables t
		join projects p on p.id = t.project_id and p.deleted_at is null
		where t.project_id = $1 and t.id = $2 and t.deleted_at is null
	`, projectID, tableID)
	t, err := scanTable(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTableNotFound
		}
		return nil, fmt.Errorf("get test data table: %w", err)
	}
	return t, nil
}

// UpdateTable modifies an existing table's metadata and columns. Rows are not
// touched here; the service layer is responsible for keeping cell values in
// sync with column changes.
func (r *Repository) UpdateTable(ctx context.Context, projectID, tableID uuid.UUID, input TableInput) (*Table, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	columns, err := encodeColumns(input.Columns)
	if err != nil {
		return nil, err
	}
	row := r.DB.QueryRow(ctx, `
		update test_data_tables
		set key = $3,
			name = $4,
			description = $5,
			columns = $6::jsonb,
			updated_at = now()
		where project_id = $1 and id = $2 and deleted_at is null
		returning id, project_id, key, name, description, columns, created_at, updated_at
	`, projectID, tableID, input.Key, input.Name, input.Description, columns)
	t, err := scanTable(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTableNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrTableKeyConflict
		}
		return nil, fmt.Errorf("update test data table: %w", err)
	}
	return t, nil
}

// DeleteTable soft-deletes a table; the cascade on the rows table is preserved
// for history but rows are filtered out by future loads via the join.
func (r *Repository) DeleteTable(ctx context.Context, projectID, tableID uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update test_data_tables
		set deleted_at = now(), updated_at = now()
		where project_id = $1 and id = $2 and deleted_at is null
	`, projectID, tableID)
	if err != nil {
		return fmt.Errorf("delete test data table: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTableNotFound
	}
	return nil
}

// ListRows returns all rows for a table, ordered by row_index then created_at.
func (r *Repository) ListRows(ctx context.Context, tableID uuid.UUID) ([]Row, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select r.id, r.table_id, r.row_index, r.values, r.created_at, r.updated_at
		from test_data_rows r
		join test_data_tables t on t.id = r.table_id and t.deleted_at is null
		where r.table_id = $1
		order by r.row_index asc, r.created_at asc
	`, tableID)
	if err != nil {
		return nil, fmt.Errorf("list test data rows: %w", err)
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		row, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Row{}
	}
	return out, nil
}

// GetRow fetches a single row by id, scoped to its parent table.
func (r *Repository) GetRow(ctx context.Context, tableID, rowID uuid.UUID) (*Row, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select r.id, r.table_id, r.row_index, r.values, r.created_at, r.updated_at
		from test_data_rows r
		join test_data_tables t on t.id = r.table_id and t.deleted_at is null
		where r.table_id = $1 and r.id = $2
	`, tableID, rowID)
	out, err := scanRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRowNotFound
		}
		return nil, fmt.Errorf("get test data row: %w", err)
	}
	return out, nil
}

// ReplaceRows replaces every row of a table inside a single transaction so the
// final state matches the supplied list. RowIndex is assigned by the order of
// the input slice.
func (r *Repository) ReplaceRows(ctx context.Context, tableID uuid.UUID, rows []RowInput) ([]Row, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin replace rows: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from test_data_rows where table_id = $1`, tableID); err != nil {
		return nil, fmt.Errorf("clear test data rows: %w", err)
	}

	out := make([]Row, 0, len(rows))
	for index, input := range rows {
		values := input.Values
		if values == nil {
			values = map[string]string{}
		}
		raw, err := json.Marshal(values)
		if err != nil {
			return nil, fmt.Errorf("encode row values: %w", err)
		}
		row := tx.QueryRow(ctx, `
			insert into test_data_rows (table_id, row_index, values)
			values ($1, $2, $3::jsonb)
			returning id, table_id, row_index, values, created_at, updated_at
		`, tableID, index, raw)
		stored, err := scanRow(row)
		if err != nil {
			return nil, fmt.Errorf("insert test data row: %w", err)
		}
		out = append(out, *stored)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit replace rows: %w", err)
	}
	return out, nil
}

// UpdateRowValues writes the given values for a single row and returns the
// fresh state.
func (r *Repository) UpdateRowValues(ctx context.Context, tableID, rowID uuid.UUID, values map[string]string) (*Row, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if values == nil {
		values = map[string]string{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode row values: %w", err)
	}
	row := r.DB.QueryRow(ctx, `
		update test_data_rows
		set values = $3::jsonb,
			updated_at = now()
		where table_id = $1 and id = $2
		returning id, table_id, row_index, values, created_at, updated_at
	`, tableID, rowID, raw)
	out, err := scanRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRowNotFound
		}
		return nil, fmt.Errorf("update test data row: %w", err)
	}
	return out, nil
}

// ListAllForProject returns every active table in a project paired with its
// rows. Used by the runner to dedupe inline reference resolution per request.
func (r *Repository) ListAllForProject(ctx context.Context, projectID uuid.UUID) ([]TableWithRows, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	tables, err := r.ListTables(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if len(tables) == 0 {
		return []TableWithRows{}, nil
	}
	out := make([]TableWithRows, 0, len(tables))
	for _, table := range tables {
		rows, err := r.ListRows(ctx, table.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, TableWithRows{Table: table, Rows: rows})
	}
	return out, nil
}

// TableWithRows pairs a Table with its eagerly loaded rows.
type TableWithRows struct {
	Table Table
	Rows  []Row
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTable(row rowScanner) (*Table, error) {
	var (
		t          Table
		columnsRaw []byte
	)
	if err := row.Scan(
		&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &columnsRaw, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	columns, err := decodeColumns(columnsRaw)
	if err != nil {
		return nil, err
	}
	t.Columns = columns
	return &t, nil
}

func scanRow(row rowScanner) (*Row, error) {
	var (
		out       Row
		valuesRaw []byte
	)
	if err := row.Scan(
		&out.ID, &out.TableID, &out.RowIndex, &valuesRaw, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	values, err := decodeValues(valuesRaw)
	if err != nil {
		return nil, err
	}
	out.Values = values
	return &out, nil
}

func encodeColumns(columns []ColumnDef) ([]byte, error) {
	if columns == nil {
		columns = []ColumnDef{}
	}
	raw, err := json.Marshal(columns)
	if err != nil {
		return nil, fmt.Errorf("encode column definitions: %w", err)
	}
	return raw, nil
}

func decodeColumns(raw []byte) ([]ColumnDef, error) {
	if len(raw) == 0 {
		return []ColumnDef{}, nil
	}
	var columns []ColumnDef
	if err := json.Unmarshal(raw, &columns); err != nil {
		return nil, fmt.Errorf("decode column definitions: %w", err)
	}
	if columns == nil {
		columns = []ColumnDef{}
	}
	return columns, nil
}

func decodeValues(raw []byte) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	values := map[string]string{}
	if err := json.Unmarshal(raw, &values); err == nil {
		return values, nil
	}
	loose := map[string]any{}
	if err := json.Unmarshal(raw, &loose); err != nil {
		return nil, fmt.Errorf("decode row values: %w", err)
	}
	for key, value := range loose {
		values[key] = anyToString(value)
	}
	return values, nil
}

func anyToString(value any) string {
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

// isUniqueViolation reports whether the underlying error is a Postgres unique
// constraint violation. We avoid pulling in pgconn directly to keep imports
// modest and rely on string matching against pgx's error message.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key value") || strings.Contains(msg, "23505")
}
