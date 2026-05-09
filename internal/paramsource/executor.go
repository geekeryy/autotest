package paramsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const defaultQueryTimeoutMillis = 3000

var disallowedSQLWord = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|create|grant|revoke|merge|copy|call)\b`)
var disallowedSQLStepWord = regexp.MustCompile(`(?i)\b(drop|alter|truncate|create|grant|revoke|merge|copy|call)\b`)

type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) Execute(ctx context.Context, item SourceWithDataSource, vars map[string]string) (map[string]string, ExecutionSnapshot, error) {
	results, snapshot, err := e.ExecuteRows(ctx, item, vars)
	if err != nil {
		return nil, snapshot, err
	}
	if len(results) == 0 {
		return map[string]string{}, snapshot, nil
	}
	return results[0], snapshot, nil
}

func (e *Executor) ExecuteRows(ctx context.Context, item SourceWithDataSource, vars map[string]string) ([]map[string]string, ExecutionSnapshot, error) {
	return e.executeRows(ctx, item, vars)
}

func (e *Executor) ExecuteSQLStep(ctx context.Context, ds DataSource, input SQLStepInput, vars map[string]string) (SQLStepResult, error) {
	result := SQLStepResult{
		DataSourceID:   ds.ID,
		DataSourceName: ds.Name,
		InputParams:    map[string]string{},
		Rows:           []map[string]string{},
	}
	if !ds.Enabled {
		err := fmt.Errorf("data source %q is disabled", ds.Name)
		result.Error = err.Error()
		return result, err
	}
	if ds.Driver != DriverPostgres {
		err := fmt.Errorf("%w: %s", ErrUnsupportedDriver, ds.Driver)
		result.Error = err.Error()
		return result, err
	}
	sqlText, err := normalizeSQLStepOperation(input.SQL)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	args, inputSnapshot, err := buildArgs(input.InputParams, vars)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.SQL = sqlText
	result.InputParams = inputSnapshot

	timeout := time.Duration(input.TimeoutMillis) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultQueryTimeoutMillis * time.Millisecond
	}
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := pgx.Connect(queryCtx, postgresURL(ds))
	if err != nil {
		err = fmt.Errorf("connect data source %q: %w", ds.Name, err)
		result.Error = err.Error()
		return result, err
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(queryCtx, sqlText, args...)
	if err != nil {
		err = fmt.Errorf("execute database step on %q: %w", ds.Name, err)
		result.Error = err.Error()
		return result, err
	}
	fields := rows.FieldDescriptions()
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			rows.Close()
			err = fmt.Errorf("scan database step rows: %w", err)
			result.Error = err.Error()
			return result, err
		}
		result.Rows = append(result.Rows, mapColumns(fields, values))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		err = fmt.Errorf("read database step rows: %w", err)
		result.Error = err.Error()
		return result, err
	}
	rows.Close()
	tag := rows.CommandTag()
	result.CommandTag = tag.String()
	result.RowsAffected = tag.RowsAffected()
	if len(result.Rows) > 0 {
		result.FirstRow = result.Rows[0]
	}
	return result, nil
}

func (e *Executor) executeRows(ctx context.Context, item SourceWithDataSource, vars map[string]string) ([]map[string]string, ExecutionSnapshot, error) {
	snapshot := ExecutionSnapshot{
		DataSourceID:   item.DataSource.ID,
		DataSourceName: item.DataSource.Name,
		SourceID:       item.Source.ID,
		SourceName:     item.Source.Name,
		InputParams:    map[string]string{},
		Result:         map[string]string{},
	}
	if !item.Source.Enabled {
		err := fmt.Errorf("sql parameter source %q is disabled", item.Source.Name)
		snapshot.Error = err.Error()
		return nil, snapshot, err
	}
	if !item.DataSource.Enabled {
		err := fmt.Errorf("data source %q is disabled", item.DataSource.Name)
		snapshot.Error = err.Error()
		return nil, snapshot, err
	}
	if item.DataSource.Driver != DriverPostgres {
		err := fmt.Errorf("%w: %s", ErrUnsupportedDriver, item.DataSource.Driver)
		snapshot.Error = err.Error()
		return nil, snapshot, err
	}

	sqlText, err := normalizeReadOnlySQL(item.Source.SQL)
	if err != nil {
		snapshot.Error = err.Error()
		return nil, snapshot, err
	}
	args, inputSnapshot, err := buildArgs(item.Source.InputParams, vars)
	if err != nil {
		snapshot.Error = err.Error()
		return nil, snapshot, err
	}
	snapshot.InputParams = inputSnapshot

	timeout := time.Duration(item.Source.TimeoutMillis) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultQueryTimeoutMillis * time.Millisecond
	}
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := pgx.Connect(queryCtx, postgresURL(item.DataSource))
	if err != nil {
		err = fmt.Errorf("connect data source %q: %w", item.DataSource.Name, err)
		snapshot.Error = err.Error()
		return nil, snapshot, err
	}
	defer conn.Close(context.Background())

	query := "select * from (" + sqlText + ") as sql_parameter_source_result"
	rows, err := conn.Query(queryCtx, query, args...)
	if err != nil {
		err = fmt.Errorf("execute sql parameter source %q: %w", item.Source.Name, err)
		snapshot.Error = err.Error()
		return nil, snapshot, err
	}
	defer rows.Close()

	results := []map[string]string{}
	fields := rows.FieldDescriptions()
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			err = fmt.Errorf("scan sql parameter source %q: %w", item.Source.Name, err)
			snapshot.Error = err.Error()
			return nil, snapshot, err
		}
		result := mapColumns(fields, values)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		err = fmt.Errorf("read sql parameter source %q: %w", item.Source.Name, err)
		snapshot.Error = err.Error()
		return nil, snapshot, err
	}
	if len(results) > 0 {
		snapshot.Result = results[0]
	}
	snapshot.Results = results
	return results, snapshot, nil
}

type inputParam struct {
	Name     string `json:"name"`
	Value    any    `json:"value"`
	Required bool   `json:"required"`
}

// parseInputParamElement decodes one SQL input parameter.
// Besides {"name","value","required"}, a bare JSON scalar or single-key shorthand
// is accepted, e.g. "{{$steps[2].body.id}}" or {"id":"{{$steps[2].body.id}}"}.
func parseInputParamElement(raw json.RawMessage) (inputParam, error) {
	var p inputParam
	if err := json.Unmarshal(raw, &p); err == nil {
		if p.Name != "" || p.Value != nil || p.Required {
			return p, nil
		}
	} else if !isJSONScalar(raw) {
		return p, err
	}

	if isJSONScalar(raw) {
		var val any
		if err := json.Unmarshal(raw, &val); err != nil {
			return inputParam{}, err
		}
		return inputParam{Value: val}, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj) == 0 {
		return p, nil
	}
	reserved := map[string]struct{}{"name": {}, "value": {}, "required": {}}
	customKeys := make([]string, 0, len(obj))
	for k := range obj {
		if _, ok := reserved[k]; ok {
			continue
		}
		customKeys = append(customKeys, k)
	}
	if len(customKeys) == 0 {
		return p, nil
	}
	if len(customKeys) > 1 || len(obj) > 1 {
		return inputParam{}, fmt.Errorf("sql input param must use {\"name\",\"value\",\"required\"} or a single-key object like {\"id\":\"{{$steps[1].body.x}}\"}; multiple keys in one object are ambiguous")
	}
	k := customKeys[0]
	var val any
	if err := json.Unmarshal(obj[k], &val); err != nil {
		return inputParam{}, fmt.Errorf("sql input param %q: %w", k, err)
	}
	return inputParam{Name: k, Value: val}, nil
}

func isJSONScalar(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null" && trimmed[0] != '{' && trimmed[0] != '['
}

func buildArgs(raw json.RawMessage, vars map[string]string) ([]any, map[string]string, error) {
	if len(raw) == 0 {
		return nil, map[string]string{}, nil
	}
	var rawElems []json.RawMessage
	if err := json.Unmarshal(raw, &rawElems); err != nil {
		return nil, nil, fmt.Errorf("decode sql input params: %w", err)
	}
	params := make([]inputParam, 0, len(rawElems))
	for _, elem := range rawElems {
		p, err := parseInputParamElement(elem)
		if err != nil {
			return nil, nil, err
		}
		params = append(params, p)
	}
	args := make([]any, 0, len(params))
	snapshot := make(map[string]string, len(params))
	for idx, param := range params {
		name := strings.TrimSpace(param.Name)
		value, hasValue := paramValue(param, vars)
		if param.Required && strings.TrimSpace(value) == "" {
			if name == "" {
				name = fmt.Sprintf("$%d", idx+1)
			}
			return nil, nil, fmt.Errorf("sql input param %q is required", name)
		}
		args = append(args, value)
		if name == "" {
			name = fmt.Sprintf("$%d", idx+1)
		}
		if !hasValue && param.Required {
			return nil, nil, fmt.Errorf("sql input param %q is required", name)
		}
		snapshot[name] = value
	}
	return args, snapshot, nil
}

func paramValue(param inputParam, vars map[string]string) (string, bool) {
	if param.Value != nil {
		return renderTemplate(valueString(param.Value), vars), true
	}
	if param.Name == "" {
		return "", false
	}
	value, ok := vars[param.Name]
	return value, ok
}

func mapColumns(fields []pgconn.FieldDescription, values []any) map[string]string {
	result := make(map[string]string, len(fields))
	for idx, field := range fields {
		value := ""
		if idx < len(values) {
			value = valueString(values[idx])
		}
		result[field.Name] = value
	}
	return result
}

func normalizeReadOnlySQL(sqlText string) (string, error) {
	clean := strings.TrimSpace(stripLeadingSQLComments(sqlText))
	clean = strings.TrimSuffix(clean, ";")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "", errors.New("sql text is required")
	}
	lower := strings.ToLower(clean)
	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with")) {
		return "", errors.New("only read-only SELECT or WITH sql is allowed")
	}
	if strings.Contains(clean, ";") {
		return "", errors.New("multiple sql statements are not allowed")
	}
	if disallowedSQLWord.MatchString(clean) {
		return "", errors.New("sql contains a disallowed write operation")
	}
	return clean, nil
}

func normalizeSQLStepOperation(sqlText string) (string, error) {
	clean := strings.TrimSpace(stripLeadingSQLComments(sqlText))
	clean = strings.TrimSuffix(clean, ";")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "", errors.New("sql text is required")
	}
	lower := strings.ToLower(clean)
	allowed := strings.HasPrefix(lower, "select") ||
		strings.HasPrefix(lower, "with") ||
		strings.HasPrefix(lower, "insert") ||
		strings.HasPrefix(lower, "update") ||
		strings.HasPrefix(lower, "delete")
	if !allowed {
		return "", errors.New("database step only allows select, with, insert, update or delete")
	}
	if strings.Contains(clean, ";") {
		return "", errors.New("multiple sql statements are not allowed")
	}
	if disallowedSQLStepWord.MatchString(clean) {
		return "", errors.New("sql contains a disallowed schema or privileged operation")
	}
	return clean, nil
}

func stripLeadingSQLComments(sqlText string) string {
	for {
		text := strings.TrimSpace(sqlText)
		switch {
		case strings.HasPrefix(text, "--"):
			if idx := strings.IndexByte(text, '\n'); idx >= 0 {
				sqlText = text[idx+1:]
				continue
			}
			return ""
		case strings.HasPrefix(text, "/*"):
			if idx := strings.Index(text, "*/"); idx >= 0 {
				sqlText = text[idx+2:]
				continue
			}
			return ""
		default:
			return text
		}
	}
}

func postgresURL(ds DataSource) string {
	host := strings.TrimSpace(ds.Host)
	port := ds.Port
	if port <= 0 {
		port = 5432
	}
	sslMode := strings.TrimSpace(ds.SSLMode)
	if sslMode == "" {
		sslMode = "disable"
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(ds.Username, ds.Password),
		Host:   net.JoinHostPort(host, fmt.Sprint(port)),
		Path:   "/" + strings.TrimLeft(ds.Database, "/"),
	}
	q := u.Query()
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func valueString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case uuid.UUID:
		return v.String()
	case [16]byte:
		// pgx uuid OID decodes to raw [16]byte (see pgtype.UUIDCodec.DecodeValue).
		return uuid.UUID(v).String()
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

// SchemaTable holds a single table and its columns for autocomplete purposes.
type SchemaTable struct {
	Schema  string   `json:"schema"`
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// IntrospectSchema queries information_schema to fetch user tables and their columns.
// Only non-system schemas are included. Limited to 500 tables to keep response fast.
func (e *Executor) IntrospectSchema(ctx context.Context, ds DataSource) ([]SchemaTable, error) {
	if ds.Driver != DriverPostgres {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, ds.Driver)
	}
	conn, err := pgx.Connect(ctx, postgresURL(ds))
	if err != nil {
		return nil, fmt.Errorf("connect data source %q: %w", ds.Name, err)
	}
	defer conn.Close(context.Background())

	const q = `
		SELECT c.table_schema, c.table_name, c.column_name
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE t.table_type IN ('BASE TABLE', 'VIEW')
		  AND c.table_schema NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
		ORDER BY c.table_schema, c.table_name, c.ordinal_position
		LIMIT 10000
	`
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("introspect schema: %w", err)
	}
	defer rows.Close()

	tableMap := map[string]*SchemaTable{}
	var tableOrder []string
	for rows.Next() {
		var schema, table, col string
		if err := rows.Scan(&schema, &table, &col); err != nil {
			return nil, fmt.Errorf("scan schema row: %w", err)
		}
		key := schema + "." + table
		if _, ok := tableMap[key]; !ok {
			tableMap[key] = &SchemaTable{Schema: schema, Name: table}
			tableOrder = append(tableOrder, key)
		}
		tableMap[key].Columns = append(tableMap[key].Columns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema rows: %w", err)
	}

	result := make([]SchemaTable, 0, len(tableOrder))
	for _, k := range tableOrder {
		result = append(result, *tableMap[k])
	}
	return result, nil
}

func renderTemplate(input string, vars map[string]string) string {
	rendered := input
	for key, value := range vars {
		if key == "" {
			continue
		}
		pattern := regexp.MustCompile(`\{\{+` + regexp.QuoteMeta(key) + `\}\}+`)
		rendered = pattern.ReplaceAllString(rendered, value)
	}
	return rendered
}
