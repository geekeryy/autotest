package runner

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseRunListFilterDefaults(t *testing.T) {
	filter, err := parseRunListFilter(staticQuery{vals: map[string]string{}}, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	if filter.Limit != 20 {
		t.Fatalf("limit: %d", filter.Limit)
	}
}

func TestParseRunListFilterScenarioID(t *testing.T) {
	scID := uuid.New()
	filter, err := parseRunListFilter(staticQuery{vals: map[string]string{
		"scenarioId": scID.String(),
		"page":       "2",
		"pageSize":   "10",
	}}, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if filter.ScenarioID == nil || *filter.ScenarioID != scID {
		t.Fatalf("scenario id: %v", filter.ScenarioID)
	}
	if filter.Offset != 10 || filter.Limit != 10 {
		t.Fatalf("pagination: offset=%d limit=%d", filter.Offset, filter.Limit)
	}
}

type staticQuery struct {
	vals map[string]string
}

func (s staticQuery) URLQueryGet(key string) string {
	return s.vals[key]
}
