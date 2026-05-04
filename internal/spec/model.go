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

type ImportResult struct {
	Hash      string          `json:"hash"`
	Title     string          `json:"title,omitempty"`
	Version   string          `json:"version,omitempty"`
	Endpoints []Endpoint      `json:"endpoints"`
	Snapshot  json.RawMessage `json:"snapshot"`
}

type ImportSummary struct {
	Spec           *APISpec   `json:"spec"`
	Endpoints      []Endpoint `json:"endpoints"`
	GeneratedCases int        `json:"generatedCases"`
}
