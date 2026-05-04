package testcase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrTestCaseNotFound = errors.New("test case not found")

type Source string

const (
	SourceAuto    Source = "auto"
	SourceManual  Source = "manual"
	SourceDerived Source = "derived"
)

type Status string

const (
	StatusDraft  Status = "draft"
	StatusActive Status = "active"
)

type TestCase struct {
	ID               uuid.UUID       `json:"id"`
	ProjectID        uuid.UUID       `json:"projectId"`
	ServiceID        uuid.UUID       `json:"serviceId"`
	EndpointID       *uuid.UUID      `json:"endpointId,omitempty"`
	Source           Source          `json:"source"`
	Name             string          `json:"name"`
	Method           string          `json:"method"`
	Path             string          `json:"path"`
	Fingerprint      string          `json:"fingerprint,omitempty"`
	GenerationRuleID string          `json:"generationRuleId,omitempty"`
	Request          json.RawMessage `json:"request"`
	Assertions       json.RawMessage `json:"assertions"`
	LastResponse     json.RawMessage `json:"lastResponseSnapshot,omitempty"`
	Status           Status          `json:"status"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type Step struct {
	ID         uuid.UUID       `json:"id"`
	TestCaseID uuid.UUID       `json:"testCaseId"`
	StepOrder  int             `json:"stepOrder"`
	Name       string          `json:"name"`
	Request    json.RawMessage `json:"request"`
	Assertions json.RawMessage `json:"assertions"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

type Draft struct {
	ProjectID        uuid.UUID
	ServiceID        uuid.UUID
	EndpointID       *uuid.UUID
	Source           Source
	Name             string
	Method           string
	Path             string
	Fingerprint      string
	GenerationRuleID string
	Request          json.RawMessage
	Assertions       json.RawMessage
	Status           Status
}

type CreateManualInput struct {
	ProjectID  uuid.UUID       `json:"projectId"`
	ServiceID  uuid.UUID       `json:"serviceId"`
	EndpointID *uuid.UUID      `json:"endpointId"`
	Name       string          `json:"name"`
	Method     string          `json:"method"`
	Path       string          `json:"path"`
	Request    json.RawMessage `json:"request"`
	Assertions json.RawMessage `json:"assertions"`
}

type ListFilter struct {
	ProjectID uuid.UUID
	ServiceID uuid.UUID
}

// EndpointSchemaGetter retrieves the request schema for an endpoint.
// Implemented by spec.Repository; defined here as an interface to avoid import cycles.
type EndpointSchemaGetter interface {
	GetEndpointRequestSchema(ctx context.Context, endpointID uuid.UUID) (json.RawMessage, error)
}

// GeneratedParams is the response body for the generate-params endpoint.
//
// The "generate params" entry point only fills path/query/body; headers and
// authentication are intentionally omitted so that triggering the action does
// not silently overwrite headers or strip the request's `security` field.
type GeneratedParams struct {
	Query map[string]string `json:"query"`
	Path  map[string]string `json:"path"`
	Body  any               `json:"body"`
}

func NewFingerprint(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.TrimSpace(strings.ToLower(part)))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return hex.EncodeToString(sum[:])
}

func CompactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	compact, err := json.Marshal(value)
	if err != nil {
		return string(raw)
	}
	return string(compact)
}
