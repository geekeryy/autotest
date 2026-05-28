package testprofile

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Profile is the aggregated test knowledge for a project. It captures
// patterns learned from OpenAPI schemas and historical run data so that
// generators and AI prompts can produce project-aware test cases.
type Profile struct {
	ID                 uuid.UUID            `json:"id"`
	ProjectID          uuid.UUID            `json:"projectId"`
	ResponseConvention *ResponseConvention  `json:"responseConvention,omitempty"`
	FieldProfiles      []FieldProfile       `json:"fieldProfiles,omitempty"`
	DependencyGraph    []EndpointDependency `json:"dependencyGraph,omitempty"`
	AuthPatterns       []AuthPattern        `json:"authPatterns,omitempty"`
	ExampleCaseIDs     []uuid.UUID          `json:"exampleCaseIds,omitempty"`
	UpdatedAt          time.Time            `json:"updatedAt"`
}

// ResponseConvention describes the project's common API response envelope.
// Most REST APIs wrap responses in a standard structure like {"code":0,"msg":"ok","data":{...}}.
type ResponseConvention struct {
	WrapperFields []string `json:"wrapperFields"`
	SuccessField  string   `json:"successField,omitempty"`
	SuccessValue  any      `json:"successValue,omitempty"`
	DataField     string   `json:"dataField,omitempty"`
	MessageField  string   `json:"messageField,omitempty"`
	Confidence    float64  `json:"confidence"`
}

// FieldProfile captures the observed value domain for a specific request field
// across the project. When IsEnum is true, the field has a bounded set of
// valid values that should be used instead of random generation.
type FieldProfile struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	Location       string `json:"location"`
	FieldPath      string `json:"fieldPath"`
	ValueType      string `json:"valueType"`
	IsEnum         bool   `json:"isEnum"`
	ObservedValues []any  `json:"observedValues,omitempty"`
}

// EndpointDependency represents a data flow between two endpoints: one
// produces a value in its response and another consumes it in its request.
type EndpointDependency struct {
	SourceMethod string  `json:"sourceMethod"`
	SourcePath   string  `json:"sourcePath"`
	SourceField  string  `json:"sourceField"`
	TargetMethod string  `json:"targetMethod"`
	TargetPath   string  `json:"targetPath"`
	TargetField  string  `json:"targetField"`
	Confidence   float64 `json:"confidence"`
}

// AuthPattern describes a commonly observed authentication header pattern.
type AuthPattern struct {
	HeaderName   string  `json:"headerName"`
	ValuePattern string  `json:"valuePattern"`
	Frequency    float64 `json:"frequency"`
}

// BuildInput holds the data sources used to build/rebuild a project profile.
type BuildInput struct {
	ProjectID  uuid.UUID
	Endpoints  []EndpointInfo
	Cases      []CaseInfo
	RunResults []RunResultInfo
}

// EndpointInfo is a simplified view of an API endpoint for profile analysis.
type EndpointInfo struct {
	ID             uuid.UUID
	Method         string
	Path           string
	OperationID    string
	Summary        string
	Tags           []string
	RequestSchema  json.RawMessage
	ResponseSchema json.RawMessage
}

// CaseInfo is a simplified view of a test case for profile analysis.
type CaseInfo struct {
	ID                   uuid.UUID
	Method               string
	Path                 string
	Source               string
	Request              json.RawMessage
	Assertions           json.RawMessage
	LastResponseSnapshot json.RawMessage
	HasManualAssertions  bool
	RunCount             int
	SuccessCount         int
}

// RunResultInfo is a simplified view of a run result for profile analysis.
type RunResultInfo struct {
	TestCaseID       uuid.UUID
	Method           string
	Path             string
	Status           string
	RequestSnapshot  json.RawMessage
	ResponseSnapshot json.RawMessage
}
