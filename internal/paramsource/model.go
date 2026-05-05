package paramsource

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDataSourceNotFound         = errors.New("data source not found")
	ErrSQLParameterSourceNotFound = errors.New("sql parameter source not found")
	ErrUnsupportedDriver          = errors.New("unsupported data source driver")
)

const (
	DriverPostgres = "postgres"
)

type DataSource struct {
	ID        uuid.UUID       `json:"id"`
	ProjectID uuid.UUID       `json:"projectId"`
	Name      string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	Driver        string          `json:"driver"`
	Host          string          `json:"host"`
	Port          int             `json:"port"`
	Database      string          `json:"database"`
	Username      string          `json:"username"`
	Password      string          `json:"-"`
	SSLMode       string          `json:"sslMode"`
	Enabled       bool            `json:"enabled"`
	Options       json.RawMessage `json:"options,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type DataSourceInput struct {
	ProjectID uuid.UUID       `json:"projectId"`
	Name      string          `json:"name"`
	Description   string          `json:"description"`
	Driver        string          `json:"driver"`
	Host          string          `json:"host"`
	Port          int             `json:"port"`
	Database      string          `json:"database"`
	Username      string          `json:"username"`
	Password      *string         `json:"password"`
	SSLMode       string          `json:"sslMode"`
	Enabled       *bool           `json:"enabled"`
	Options       json.RawMessage `json:"options"`
}

func (input *DataSourceInput) UnmarshalJSON(data []byte) error {
	type dataSourceInputAlias DataSourceInput
	var raw struct {
		dataSourceInputAlias
		Port    any             `json:"port"`
		Config  json.RawMessage `json:"config"`
		Options json.RawMessage `json:"options"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}

	*input = DataSourceInput(raw.dataSourceInputAlias)
	port, hasPort, err := parseFlexiblePort(raw.Port)
	if err != nil {
		return err
	}
	if hasPort {
		input.Port = port
	}
	options := raw.Options
	if len(options) == 0 {
		options = raw.Config
	}
	if len(options) > 0 {
		input.Options = options
		if !hasPort {
			configPort, ok, err := portFromConfig(options)
			if err != nil {
				return err
			}
			if ok {
				input.Port = configPort
			}
		}
	}
	return nil
}

func parseFlexiblePort(value any) (int, bool, error) {
	switch v := value.(type) {
	case nil:
		return 0, false, nil
	case json.Number:
		port, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, true, fmt.Errorf("port must be an integer: %w", err)
		}
		return port, true, nil
	case float64:
		port := int(v)
		if float64(port) != v {
			return 0, true, fmt.Errorf("port must be an integer")
		}
		return port, true, nil
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return 0, false, nil
		}
		port, err := strconv.Atoi(text)
		if err != nil {
			return 0, true, fmt.Errorf("port must be an integer: %w", err)
		}
		return port, true, nil
	default:
		return 0, true, fmt.Errorf("port must be a number or numeric string")
	}
}

func portFromConfig(raw json.RawMessage) (int, bool, error) {
	var config map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&config); err != nil {
		return 0, false, fmt.Errorf("decode data source config: %w", err)
	}
	return parseFlexiblePort(config["port"])
}

type SQLParameterSource struct {
	ID            uuid.UUID       `json:"id"`
	ProjectID     uuid.UUID       `json:"projectId"`
	ServiceID     uuid.UUID       `json:"serviceId"`
	DataSourceID  uuid.UUID       `json:"dataSourceId"`
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	SQL           string          `json:"sql"`
	InputParams   json.RawMessage `json:"inputParams"`
	Enabled       bool            `json:"enabled"`
	TimeoutMillis int             `json:"timeoutMillis"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type SQLParameterSourceInput struct {
	ProjectID     uuid.UUID       `json:"projectId"`
	ServiceID     uuid.UUID       `json:"serviceId"`
	DataSourceID  uuid.UUID       `json:"dataSourceId"`
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	SQL           string          `json:"sql"`
	InputParams   json.RawMessage `json:"inputParams"`
	Enabled       *bool           `json:"enabled"`
	TimeoutMillis int             `json:"timeoutMillis"`
}

type SourceWithDataSource struct {
	Source     SQLParameterSource
	DataSource DataSource
}

type ExecutionSnapshot struct {
	DataSourceID   uuid.UUID           `json:"dataSourceId"`
	DataSourceName string              `json:"dataSourceName"`
	SourceID       uuid.UUID           `json:"sourceId"`
	SourceName     string              `json:"sourceName"`
	InputParams    map[string]string   `json:"inputParams"`
	Result         map[string]string   `json:"result"`
	Results        []map[string]string `json:"results,omitempty"`
	Error          string              `json:"error,omitempty"`
}

type PreviewInput struct {
	Variables   map[string]any `json:"variables"`
	InputParams map[string]any `json:"inputParams,omitempty"`
}

// PreviewDraftInput 在未持久化时根据表单字段连接数据源执行只读 SQL，用于新增/编辑弹窗内测试。
type PreviewDraftInput struct {
	ProjectID     uuid.UUID       `json:"projectId"`
	ServiceID     uuid.UUID       `json:"serviceId"`
	DataSourceID  uuid.UUID       `json:"dataSourceId"`
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	SQL           string          `json:"sql"`
	InputParams   json.RawMessage `json:"inputParams"`
	Enabled       *bool           `json:"enabled"`
	TimeoutMillis int             `json:"timeoutMillis"`
	Variables     map[string]any  `json:"variables"`
}

type PreviewOutput struct {
	Variables map[string]string   `json:"variables"`
	Rows      []map[string]string `json:"rows,omitempty"`
	Snapshot  ExecutionSnapshot   `json:"snapshot"`
}

type SQLStepInput struct {
	DataSourceID  uuid.UUID       `json:"dataSourceId"`
	SQL           string          `json:"sql"`
	InputParams   json.RawMessage `json:"inputParams"`
	TimeoutMillis int             `json:"timeoutMillis"`
}

type SQLStepResult struct {
	DataSourceID   uuid.UUID           `json:"dataSourceId"`
	DataSourceName string              `json:"dataSourceName"`
	SQL            string              `json:"sql"`
	InputParams    map[string]string   `json:"inputParams"`
	Rows           []map[string]string `json:"rows"`
	FirstRow       map[string]string   `json:"firstRow,omitempty"`
	RowsAffected   int64               `json:"rowsAffected"`
	CommandTag     string              `json:"commandTag"`
	Error          string              `json:"error,omitempty"`
}
