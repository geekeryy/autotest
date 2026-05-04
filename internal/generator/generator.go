package generator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	testcase "autotest/internal/case"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID             uuid.UUID
	ProjectID      uuid.UUID
	ServiceID      uuid.UUID
	Method         string
	Path           string
	OperationID    string
	RequestSchema  json.RawMessage
	ResponseSchema json.RawMessage
}

type Rule interface {
	ID() string
	Generate(endpoint Endpoint) ([]testcase.Draft, error)
}

type Generator struct {
	rules []Rule
}

func NewDefault() *Generator {
	return &Generator{
		rules: []Rule{
			HappyPathRule{},
			RequiredMissingRule{},
			TypeMismatchRule{},
		},
	}
}

func (g *Generator) Generate(endpoint Endpoint) ([]testcase.Draft, error) {
	var drafts []testcase.Draft
	for _, rule := range g.rules {
		ruleDrafts, err := rule.Generate(endpoint)
		if err != nil {
			return nil, fmt.Errorf("generate %s: %w", rule.ID(), err)
		}
		drafts = append(drafts, ruleDrafts...)
	}
	return drafts, nil
}

func buildFingerprint(endpoint Endpoint, ruleID string) string {
	return testcase.NewFingerprint(
		endpoint.ProjectID.String(),
		endpoint.ServiceID.String(),
		endpoint.ID.String(),
		ruleID,
		endpoint.Method,
		endpoint.Path,
	)
}

func defaultExpectedStatus(responseSchema json.RawMessage) int {
	var value map[string]any
	if err := json.Unmarshal(responseSchema, &value); err != nil {
		return 200
	}
	status, _ := value["status"].(string)
	if status == "" || status == "default" {
		return 200
	}
	code, err := strconv.Atoi(status)
	if err != nil {
		return 200
	}
	return code
}

func caseName(prefix string, endpoint Endpoint) string {
	name := endpoint.OperationID
	if name == "" {
		name = strings.ToUpper(endpoint.Method) + " " + endpoint.Path
	}
	return prefix + " " + name
}
