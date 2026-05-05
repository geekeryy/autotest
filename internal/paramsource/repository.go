package paramsource

import (
	"context"
	"errors"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	store.Repository
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

func (r *Repository) CreateDataSource(ctx context.Context, input DataSourceInput) (*DataSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	password := ""
	if input.Password != nil {
		password = *input.Password
	}
	row := r.DB.QueryRow(ctx, `
		insert into business_data_sources (
			project_id, name, description, driver,
			host, port, database_name, username, password_secret, ssl_mode,
			enabled, options
		)
		select p.id, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		from projects p
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, name, description, driver,
			host, port, database_name, username, password_secret, ssl_mode, enabled,
			options, created_at, updated_at
	`, input.ProjectID, input.Name, input.Description, input.Driver,
		input.Host, input.Port, input.Database, input.Username, password, input.SSLMode,
		boolValue(input.Enabled, true), defaultJSON(input.Options, "{}"))
	return scanDataSource(row)
}

func (r *Repository) ListDataSources(ctx context.Context, projectID uuid.UUID) ([]DataSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select ds.id, ds.project_id, ds.name, ds.description,
			ds.driver, ds.host, ds.port, ds.database_name, ds.username, ds.password_secret,
			ds.ssl_mode, ds.enabled, ds.options, ds.created_at, ds.updated_at
		from business_data_sources ds
		join projects p on p.id = ds.project_id and p.deleted_at is null
		where ds.project_id = $1
		  and ds.deleted_at is null
		order by ds.created_at desc
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list data sources: %w", err)
	}
	defer rows.Close()
	return scanDataSources(rows)
}

func (r *Repository) GetDataSource(ctx context.Context, id uuid.UUID) (*DataSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select ds.id, ds.project_id, ds.name, ds.description,
			ds.driver, ds.host, ds.port, ds.database_name, ds.username, ds.password_secret,
			ds.ssl_mode, ds.enabled, ds.options, ds.created_at, ds.updated_at
		from business_data_sources ds
		join projects p on p.id = ds.project_id and p.deleted_at is null
		where ds.id = $1 and ds.deleted_at is null
	`, id)
	ds, err := scanDataSource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDataSourceNotFound
		}
		return nil, err
	}
	return ds, nil
}

func (r *Repository) UpdateDataSource(ctx context.Context, id uuid.UUID, input DataSourceInput) (*DataSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	var password any
	if input.Password != nil {
		password = *input.Password
	}
	row := r.DB.QueryRow(ctx, `
		update business_data_sources ds
		set name = $2,
			description = $3,
			driver = $4,
			host = $5,
			port = $6,
			database_name = $7,
			username = $8,
			password_secret = coalesce($9, password_secret),
			ssl_mode = $10,
			enabled = $11,
			options = $12,
			updated_at = now()
		from projects p
		where ds.id = $1
		  and ds.project_id = $13
		  and p.id = ds.project_id
		  and p.deleted_at is null
		  and ds.deleted_at is null
		returning ds.id, ds.project_id, ds.name, ds.description,
			ds.driver, ds.host, ds.port, ds.database_name, ds.username, ds.password_secret,
			ds.ssl_mode, ds.enabled, ds.options, ds.created_at, ds.updated_at
	`, id, input.Name, input.Description, input.Driver, input.Host, input.Port, input.Database,
		input.Username, password, input.SSLMode, boolValue(input.Enabled, true),
		defaultJSON(input.Options, "{}"), input.ProjectID)
	ds, err := scanDataSource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDataSourceNotFound
		}
		return nil, fmt.Errorf("update data source: %w", err)
	}
	return ds, nil
}

func (r *Repository) DeleteDataSource(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update business_data_sources
		set deleted_at = now(), updated_at = now()
		where id = $1 and deleted_at is null
	`, id)
	if err != nil {
		return fmt.Errorf("delete data source: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDataSourceNotFound
	}
	return nil
}

func (r *Repository) CreateSQLParameterSource(ctx context.Context, input SQLParameterSourceInput) (*SQLParameterSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		with new_source_id as (select gen_random_uuid() as id)
		insert into sql_parameter_sources (
			id, project_id, service_id, data_source_id, source_key, name, description, sql_text,
			input_params, enabled, timeout_millis
		)
		select new_source_id.id, p.id, s.id, ds.id,
			coalesce(nullif($4, ''), 'sql_' || left(replace(new_source_id.id::text, '-', ''), 12)),
			$5, $6, $7, $8, $9, $10
		from new_source_id
		cross join projects p
		join services s on s.project_id = p.id and s.id = $2 and s.deleted_at is null
		join business_data_sources ds on ds.project_id = p.id and ds.id = $3 and ds.deleted_at is null
		where p.id = $1 and p.deleted_at is null
		returning id, project_id, service_id, data_source_id, source_key, name, description,
			sql_text, input_params, enabled, timeout_millis,
			created_at, updated_at
	`, input.ProjectID, input.ServiceID, input.DataSourceID, input.Key, input.Name, input.Description, input.SQL,
		defaultJSON(input.InputParams, "[]"), boolValue(input.Enabled, true), input.TimeoutMillis)
	return scanSQLParameterSource(row)
}

func (r *Repository) ListSQLParameterSources(ctx context.Context, projectID, serviceID uuid.UUID) ([]SQLParameterSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select sps.id, sps.project_id, sps.service_id, sps.data_source_id, sps.source_key, sps.name,
			sps.description, sps.sql_text, sps.input_params,
			sps.enabled, sps.timeout_millis, sps.created_at, sps.updated_at
		from sql_parameter_sources sps
		join projects p on p.id = sps.project_id and p.deleted_at is null
		join services s on s.id = sps.service_id and s.deleted_at is null
		where sps.project_id = $1 and sps.service_id = $2 and sps.deleted_at is null
		order by sps.created_at desc
	`, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list sql parameter sources: %w", err)
	}
	defer rows.Close()
	return scanSQLParameterSources(rows)
}

func (r *Repository) GetSQLParameterSource(ctx context.Context, id uuid.UUID) (*SQLParameterSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select sps.id, sps.project_id, sps.service_id, sps.data_source_id, sps.source_key, sps.name,
			sps.description, sps.sql_text, sps.input_params,
			sps.enabled, sps.timeout_millis, sps.created_at, sps.updated_at
		from sql_parameter_sources sps
		join projects p on p.id = sps.project_id and p.deleted_at is null
		join services s on s.id = sps.service_id and s.deleted_at is null
		where sps.id = $1 and sps.deleted_at is null
	`, id)
	source, err := scanSQLParameterSource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSQLParameterSourceNotFound
		}
		return nil, err
	}
	return source, nil
}

func (r *Repository) GetExecutableSource(ctx context.Context, id uuid.UUID) (*SourceWithDataSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		select
			sps.id, sps.project_id, sps.service_id, sps.data_source_id, sps.source_key, sps.name,
			sps.description, sps.sql_text, sps.input_params,
			sps.enabled, sps.timeout_millis, sps.created_at, sps.updated_at,
			ds.id, ds.project_id, ds.name, ds.description,
			ds.driver, ds.host, ds.port, ds.database_name, ds.username, ds.password_secret,
			ds.ssl_mode, ds.enabled, ds.options, ds.created_at, ds.updated_at
		from sql_parameter_sources sps
		join business_data_sources ds on ds.id = sps.data_source_id and ds.deleted_at is null
		join projects p on p.id = sps.project_id and p.deleted_at is null
		join services s on s.id = sps.service_id and s.deleted_at is null
		where sps.id = $1 and sps.deleted_at is null
	`, id)
	return scanSourceWithDataSource(row)
}

func (r *Repository) UpdateSQLParameterSource(ctx context.Context, id uuid.UUID, input SQLParameterSourceInput) (*SQLParameterSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := r.DB.QueryRow(ctx, `
		update sql_parameter_sources sps
		set data_source_id = ds.id,
			source_key = coalesce(nullif($2, ''), sps.source_key),
			name = $3,
			description = $4,
			sql_text = $5,
			input_params = $6,
			enabled = $7,
			timeout_millis = $8,
			updated_at = now()
		from projects p
		join services s on s.project_id = p.id and s.id = $9 and s.deleted_at is null
		join business_data_sources ds on ds.project_id = p.id and ds.id = $10 and ds.deleted_at is null
		where sps.id = $1
		  and sps.project_id = $11
		  and sps.service_id = s.id
		  and p.id = sps.project_id
		  and p.deleted_at is null
		  and sps.deleted_at is null
		returning sps.id, sps.project_id, sps.service_id, sps.data_source_id, sps.source_key, sps.name,
			sps.description, sps.sql_text, sps.input_params,
			sps.enabled, sps.timeout_millis, sps.created_at, sps.updated_at
	`, id, input.Key, input.Name, input.Description, input.SQL, defaultJSON(input.InputParams, "[]"),
		boolValue(input.Enabled, true), input.TimeoutMillis, input.ServiceID, input.DataSourceID, input.ProjectID)
	source, err := scanSQLParameterSource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSQLParameterSourceNotFound
		}
		return nil, fmt.Errorf("update sql parameter source: %w", err)
	}
	return source, nil
}

func (r *Repository) DeleteSQLParameterSource(ctx context.Context, id uuid.UUID) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	tag, err := r.DB.Exec(ctx, `
		update sql_parameter_sources
		set deleted_at = now(), updated_at = now()
		where id = $1 and deleted_at is null
	`, id)
	if err != nil {
		return fmt.Errorf("delete sql parameter source: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSQLParameterSourceNotFound
	}
	return nil
}

func (r *Repository) ListExecutableSources(ctx context.Context, projectID, serviceID uuid.UUID) ([]SourceWithDataSource, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := r.DB.Query(ctx, `
		select
			sps.id, sps.project_id, sps.service_id, sps.data_source_id, sps.source_key, sps.name,
			sps.description, sps.sql_text, sps.input_params,
			sps.enabled, sps.timeout_millis, sps.created_at, sps.updated_at,
			ds.id, ds.project_id, ds.name, ds.description,
			ds.driver, ds.host, ds.port, ds.database_name, ds.username, ds.password_secret,
			ds.ssl_mode, ds.enabled, ds.options, ds.created_at, ds.updated_at
		from sql_parameter_sources sps
		join business_data_sources ds on ds.id = sps.data_source_id
			and ds.project_id = sps.project_id
			and ds.deleted_at is null
			and ds.enabled
		join projects p on p.id = sps.project_id and p.deleted_at is null
		join services s on s.id = sps.service_id and s.deleted_at is null
		where sps.project_id = $1
		  and sps.service_id = $2
		  and sps.deleted_at is null
		  and sps.enabled
		order by sps.created_at asc
	`, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list executable parameter sources: %w", err)
	}
	defer rows.Close()
	var sources []SourceWithDataSource
	for rows.Next() {
		source, err := scanSourceWithDataSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, *source)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if sources == nil {
		return []SourceWithDataSource{}, nil
	}
	return sources, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDataSource(row rowScanner) (*DataSource, error) {
	var ds DataSource
	if err := row.Scan(
		&ds.ID, &ds.ProjectID, &ds.Name, &ds.Description,
		&ds.Driver, &ds.Host, &ds.Port, &ds.Database, &ds.Username, &ds.Password,
		&ds.SSLMode, &ds.Enabled, &ds.Options, &ds.CreatedAt, &ds.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &ds, nil
}

func scanDataSources(rows pgx.Rows) ([]DataSource, error) {
	var sources []DataSource
	for rows.Next() {
		ds, err := scanDataSource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan data source: %w", err)
		}
		sources = append(sources, *ds)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if sources == nil {
		return []DataSource{}, nil
	}
	return sources, nil
}

func scanSQLParameterSource(row rowScanner) (*SQLParameterSource, error) {
	var source SQLParameterSource
	if err := row.Scan(
		&source.ID, &source.ProjectID, &source.ServiceID, &source.DataSourceID,
		&source.Key, &source.Name, &source.Description, &source.SQL, &source.InputParams,
		&source.Enabled, &source.TimeoutMillis, &source.CreatedAt, &source.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &source, nil
}

func scanSQLParameterSources(rows pgx.Rows) ([]SQLParameterSource, error) {
	var sources []SQLParameterSource
	for rows.Next() {
		source, err := scanSQLParameterSource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sql parameter source: %w", err)
		}
		sources = append(sources, *source)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if sources == nil {
		return []SQLParameterSource{}, nil
	}
	return sources, nil
}

func scanSourceWithDataSource(row rowScanner) (*SourceWithDataSource, error) {
	var source SourceWithDataSource
	if err := row.Scan(
		&source.Source.ID, &source.Source.ProjectID, &source.Source.ServiceID, &source.Source.DataSourceID,
		&source.Source.Key, &source.Source.Name, &source.Source.Description, &source.Source.SQL, &source.Source.InputParams,
		&source.Source.Enabled, &source.Source.TimeoutMillis, &source.Source.CreatedAt, &source.Source.UpdatedAt,
		&source.DataSource.ID, &source.DataSource.ProjectID,
		&source.DataSource.Name, &source.DataSource.Description, &source.DataSource.Driver, &source.DataSource.Host,
		&source.DataSource.Port, &source.DataSource.Database, &source.DataSource.Username, &source.DataSource.Password,
		&source.DataSource.SSLMode, &source.DataSource.Enabled, &source.DataSource.Options,
		&source.DataSource.CreatedAt, &source.DataSource.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan executable sql parameter source: %w", err)
	}
	return &source, nil
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func defaultJSON(raw []byte, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	return raw
}
