package paramsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var sourceKeyAllowed = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type Service struct {
	repo     *Repository
	executor *Executor
}

func NewService(repo *Repository, executor *Executor) *Service {
	if executor == nil {
		executor = NewExecutor()
	}
	return &Service{repo: repo, executor: executor}
}

func (s *Service) CreateDataSource(ctx context.Context, input DataSourceInput) (*DataSource, error) {
	if err := normalizeDataSourceInput(&input, true); err != nil {
		return nil, err
	}
	return s.repo.CreateDataSource(ctx, input)
}

func (s *Service) ListDataSources(ctx context.Context) ([]DataSource, error) {
	return s.repo.ListDataSources(ctx)
}

func (s *Service) UpdateDataSource(ctx context.Context, id uuid.UUID, input DataSourceInput) (*DataSource, error) {
	if id == uuid.Nil {
		return nil, errors.New("data source id is required")
	}
	if err := normalizeDataSourceInput(&input, false); err != nil {
		return nil, err
	}
	return s.repo.UpdateDataSource(ctx, id, input)
}

func (s *Service) DeleteDataSource(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("data source id is required")
	}
	return s.repo.DeleteDataSource(ctx, id)
}

func (s *Service) GetDataSourceSchema(ctx context.Context, id uuid.UUID) ([]SchemaTable, error) {
	if id == uuid.Nil {
		return nil, errors.New("data source id is required")
	}
	ds, err := s.repo.GetDataSource(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.executor.IntrospectSchema(ctx, *ds)
}

func (s *Service) TestDataSource(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("data source id is required")
	}
	ds, err := s.repo.GetDataSource(ctx, id)
	if err != nil {
		return err
	}
	source := SourceWithDataSource{
		DataSource: *ds,
		Source: SQLParameterSource{
			ID:            uuid.New(),
			Name:          "connection test",
			SQL:           "select 1 as ok",
			Enabled:       true,
			TimeoutMillis: defaultQueryTimeoutMillis,
		},
	}
	_, _, err = s.executor.Execute(ctx, source, nil)
	return err
}

func (s *Service) ExecuteSQLStep(ctx context.Context, projectID, serviceID, environmentID uuid.UUID, input SQLStepInput, vars map[string]string) (SQLStepResult, error) {
	if projectID == uuid.Nil {
		return SQLStepResult{}, errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return SQLStepResult{}, errors.New("serviceId is required")
	}
	if environmentID == uuid.Nil {
		return SQLStepResult{}, errors.New("environmentId is required")
	}
	if input.DataSourceID == uuid.Nil {
		return SQLStepResult{}, errors.New("dataSourceId is required")
	}
	if len(input.InputParams) == 0 {
		input.InputParams = []byte(`[]`)
	}
	ds, err := s.repo.GetDataSource(ctx, input.DataSourceID)
	if err != nil {
		return SQLStepResult{}, err
	}
	return s.executor.ExecuteSQLStep(ctx, *ds, input, vars)
}

func (s *Service) CreateSQLParameterSource(ctx context.Context, input SQLParameterSourceInput) (*SQLParameterSource, error) {
	if err := normalizeSQLParameterSourceInput(&input); err != nil {
		return nil, err
	}
	return s.repo.CreateSQLParameterSource(ctx, input)
}

func (s *Service) ListSQLParameterSources(ctx context.Context, projectID, serviceID uuid.UUID) ([]SQLParameterSource, error) {
	if projectID == uuid.Nil {
		return nil, errors.New("projectId is required")
	}
	if serviceID == uuid.Nil {
		return nil, errors.New("serviceId is required")
	}
	return s.repo.ListSQLParameterSources(ctx, projectID, serviceID)
}

func (s *Service) UpdateSQLParameterSource(ctx context.Context, id uuid.UUID, input SQLParameterSourceInput) (*SQLParameterSource, error) {
	if id == uuid.Nil {
		return nil, errors.New("sql parameter source id is required")
	}
	if err := normalizeSQLParameterSourceInput(&input); err != nil {
		return nil, err
	}
	return s.repo.UpdateSQLParameterSource(ctx, id, input)
}

func (s *Service) DeleteSQLParameterSource(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("sql parameter source id is required")
	}
	return s.repo.DeleteSQLParameterSource(ctx, id)
}

func (s *Service) PreviewSQLParameterSource(ctx context.Context, id uuid.UUID, input PreviewInput) (*PreviewOutput, error) {
	if id == uuid.Nil {
		return nil, errors.New("sql parameter source id is required")
	}
	source, err := s.repo.GetExecutableSource(ctx, id)
	if err != nil {
		return nil, err
	}
	previewVars := input.Variables
	if previewVars == nil {
		previewVars = input.InputParams
	}
	vars := stringifyVariables(previewVars)
	results, snapshot, err := s.executor.ExecuteRows(ctx, *source, vars)
	if err != nil {
		return nil, err
	}
	variables := map[string]string{}
	if len(results) > 0 {
		variables = results[0]
	}
	return &PreviewOutput{Variables: variables, Rows: results, Snapshot: snapshot}, nil
}

func (s *Service) ExecuteReadOnlyQuery(ctx context.Context, dataSourceID uuid.UUID, sql string, params json.RawMessage) ([]map[string]string, error) {
	if dataSourceID == uuid.Nil {
		return nil, errors.New("dataSourceId is required")
	}
	ds, err := s.repo.GetDataSource(ctx, dataSourceID)
	if err != nil {
		return nil, err
	}
	if !ds.Enabled {
		return nil, fmt.Errorf("data source %q is disabled", ds.Name)
	}
	sqlText, err := normalizeReadOnlySQL(sql)
	if err != nil {
		return nil, err
	}
	args, _, err := buildArgs(params, nil)
	if err != nil {
		return nil, err
	}
	timeout := defaultQueryTimeoutMillis * time.Millisecond
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := pgx.Connect(queryCtx, postgresURL(*ds))
	if err != nil {
		return nil, fmt.Errorf("connect data source %q: %w", ds.Name, err)
	}
	defer conn.Close(context.Background())

	query := "select * from (" + sqlText + ") as query_result"
	rows, err := conn.Query(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execute query on %q: %w", ds.Name, err)
	}
	defer rows.Close()

	results := []map[string]string{}
	fields := rows.FieldDescriptions()
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("scan query rows: %w", err)
		}
		results = append(results, mapColumns(fields, values))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read query rows: %w", err)
	}
	return results, nil
}

func (s *Service) PreviewSQLParameterSourceDraft(ctx context.Context, input PreviewDraftInput) (*PreviewOutput, error) {
	in := SQLParameterSourceInput{
		ProjectID:     input.ProjectID,
		ServiceID:     input.ServiceID,
		DataSourceID:  input.DataSourceID,
		Key:           input.Key,
		Name:          strings.TrimSpace(input.Name),
		Description:   input.Description,
		SQL:           input.SQL,
		InputParams:   input.InputParams,
		Enabled:       input.Enabled,
		TimeoutMillis: input.TimeoutMillis,
	}
	if in.Name == "" {
		in.Name = "SQL 预览"
	}
	if err := normalizeSQLParameterSourceInput(&in); err != nil {
		return nil, err
	}
	ds, err := s.repo.GetDataSource(ctx, in.DataSourceID)
	if err != nil {
		return nil, err
	}
	item := SourceWithDataSource{
		DataSource: *ds,
		Source: SQLParameterSource{
			ID:            uuid.Nil,
			ProjectID:     in.ProjectID,
			ServiceID:     in.ServiceID,
			DataSourceID:  in.DataSourceID,
			Key:           in.Key,
			Name:          in.Name,
			Description:   in.Description,
			SQL:           in.SQL,
			InputParams:   in.InputParams,
			Enabled:       true,
			TimeoutMillis: in.TimeoutMillis,
		},
	}
	previewVars := input.Variables
	if previewVars == nil {
		previewVars = map[string]any{}
	}
	vars := stringifyVariables(previewVars)
	results, snapshot, err := s.executor.ExecuteRows(ctx, item, vars)
	if err != nil {
		return nil, err
	}
	out := &PreviewOutput{Variables: map[string]string{}, Snapshot: snapshot}
	if len(results) > 0 {
		out.Variables = results[0]
	}
	out.Rows = results
	return out, nil
}

func normalizeDataSourceInput(input *DataSourceInput, creating bool) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return errors.New("data source name is required")
	}
	if input.Driver == "" {
		input.Driver = DriverPostgres
	}
	if input.Driver != DriverPostgres {
		return ErrUnsupportedDriver
	}
	if strings.TrimSpace(input.Host) == "" {
		return errors.New("data source host is required")
	}
	if strings.TrimSpace(input.Database) == "" {
		return errors.New("data source database is required")
	}
	if input.Port <= 0 {
		input.Port = 5432
	}
	if strings.TrimSpace(input.SSLMode) == "" {
		input.SSLMode = "disable"
	}
	if creating && input.Password == nil {
		empty := ""
		input.Password = &empty
	}
	return nil
}

func normalizeSQLParameterSourceInput(input *SQLParameterSourceInput) error {
	if input.ProjectID == uuid.Nil {
		return errors.New("projectId is required")
	}
	if input.ServiceID == uuid.Nil {
		return errors.New("serviceId is required")
	}
	if input.DataSourceID == uuid.Nil {
		return errors.New("dataSourceId is required")
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return errors.New("sql parameter source name is required")
	}
	key, err := normalizeSourceKey(input.Key, input.Name)
	if err != nil {
		return err
	}
	input.Key = key
	if _, err := normalizeReadOnlySQL(input.SQL); err != nil {
		return err
	}
	if input.TimeoutMillis <= 0 {
		input.TimeoutMillis = defaultQueryTimeoutMillis
	}
	if len(input.InputParams) == 0 {
		input.InputParams = []byte(`[]`)
	}
	return nil
}

func normalizeSourceKey(key, name string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = slugSourceKey(name)
	}
	if key == "" {
		return "", nil
	}
	if !sourceKeyAllowed.MatchString(key) {
		return "", errors.New("sql parameter source key may only contain letters, numbers, underscore or hyphen, up to 64 characters")
	}
	return key, nil
}

func slugSourceKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '_' || r == '-':
			if !lastDash && b.Len() > 0 {
				b.WriteRune(r)
				lastDash = r == '-'
			}
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
		if b.Len() >= 64 {
			break
		}
	}
	return strings.Trim(b.String(), "-_")
}

func stringifyVariables(input map[string]any) map[string]string {
	vars := make(map[string]string, len(input))
	for key, value := range input {
		vars[key] = valueString(value)
	}
	return vars
}

func MarshalSnapshots(snapshots []ExecutionSnapshot) json.RawMessage {
	if len(snapshots) == 0 {
		return []byte(`[]`)
	}
	raw, _ := json.Marshal(snapshots)
	return raw
}
