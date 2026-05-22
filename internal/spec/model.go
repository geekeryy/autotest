package spec

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type APISpec struct {
	ID                 uuid.UUID       `json:"id"`
	ServiceID          uuid.UUID       `json:"serviceId"`
	Version            int             `json:"version"`
	ContentHash        string          `json:"contentHash"`
	RawContent         []byte          `json:"-"`
	NormalizedSnapshot json.RawMessage `json:"normalizedSnapshot"`
	Status             string          `json:"status"`
	CreatedAt          time.Time       `json:"createdAt"`
}

type Endpoint struct {
	ID             uuid.UUID       `json:"id"`
	ServiceID      uuid.UUID       `json:"serviceId"`
	SpecID         uuid.UUID       `json:"specId"`
	Method         string          `json:"method"`
	Path           string          `json:"path"`
	OperationID    string          `json:"operationId,omitempty"`
	Summary        string          `json:"summary,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	RequestSchema  json.RawMessage `json:"requestSchema"`
	ResponseSchema json.RawMessage `json:"responseSchema"`
	Fingerprint    string          `json:"fingerprint"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// ensureTags returns an empty slice when tags is nil so PostgreSQL text[] receives '{}' not NULL.
func ensureTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

type ImportResult struct {
	Hash      string          `json:"hash"`
	Title     string          `json:"title,omitempty"`
	Version   string          `json:"version,omitempty"`
	Endpoints []Endpoint      `json:"endpoints"`
	Snapshot  json.RawMessage `json:"snapshot"`
}

// ImportSummary 为导入接口的 HTTP 响应体：仅含统计与小体量标识，不含端点列表或 snapshot。
type ImportSummary struct {
	SpecID           uuid.UUID `json:"specId"`
	SpecVersion      int       `json:"specVersion"`
	ContentHash      string    `json:"contentHash"`
	ApiCount         int       `json:"apiCount"`
	CreatedEndpoints int       `json:"createdEndpoints"`
	UpdatedEndpoints int       `json:"updatedEndpoints"`
	GeneratedCases   int       `json:"generatedCases"`
}
