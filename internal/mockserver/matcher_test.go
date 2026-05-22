package mockserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMatchRouteExactAndPlaceholderPath(t *testing.T) {
	tests := []struct {
		name  string
		route MockRoute
		url   string
		want  bool
	}{
		{
			name:  "exact path",
			route: testRoute(http.MethodGet, "/api/users"),
			url:   "/api/users",
			want:  true,
		},
		{
			name:  "placeholder path",
			route: testRoute(http.MethodGet, "/api/users/{id}/orders/{orderID}"),
			url:   "/api/users/42/orders/A100",
			want:  true,
		},
		{
			name:  "placeholder requires same segment count",
			route: testRoute(http.MethodGet, "/api/users/{id}"),
			url:   "/api/users/42/orders",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			got, err := MatchRoute(tt.route, req, nil)
			if err != nil {
				t.Fatalf("MatchRoute returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("MatchRoute = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchRouteRequestConditions(t *testing.T) {
	route := testRoute(http.MethodPost, "/api/users/{id}")
	route.RequestMatch = mustJSON(t, RequestMatch{
		Query:        map[string]string{"mode": "preview"},
		Headers:      map[string]string{"X-Mock-Case": "happy"},
		BodyContains: "jiangyang",
		BodyJSON: map[string]any{
			"user.id": "42",
			"profile": map[string]any{
				"active": true,
			},
		},
	})
	body := `{"user":{"id":"42","name":"jiangyang"},"profile":{"active":true,"role":"tester"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/users/42?mode=preview", strings.NewReader(body))
	req.Header.Set("X-Mock-Case", "happy")

	got, err := MatchRoute(route, req, []byte(body))
	if err != nil {
		t.Fatalf("MatchRoute returned error: %v", err)
	}
	if !got {
		t.Fatal("MatchRoute = false, want true")
	}
}

func TestMatchRouteRejectsMismatchedConditions(t *testing.T) {
	route := testRoute(http.MethodPost, "/api/users/{id}")
	route.RequestMatch = mustJSON(t, RequestMatch{
		Query:   map[string]string{"mode": "preview"},
		Headers: map[string]string{"X-Mock-Case": "happy"},
		BodyJSON: map[string]any{
			"user.id": "42",
		},
	})
	body := `{"user":{"id":"99"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/users/99?mode=preview", strings.NewReader(body))
	req.Header.Set("X-Mock-Case", "happy")

	got, err := MatchRoute(route, req, []byte(body))
	if err != nil {
		t.Fatalf("MatchRoute returned error: %v", err)
	}
	if got {
		t.Fatal("MatchRoute = true, want false")
	}
}

func TestMatchRulesUsesHighestPriorityMatch(t *testing.T) {
	low := testRoute(http.MethodGet, "/api/users/{id}")
	low.Priority = 1
	low.ResponseBody = "low"
	high := testRoute(http.MethodGet, "/api/users/{id}")
	high.Priority = 10
	high.ResponseBody = "high"

	req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	got, err := MatchRules([]MockRoute{low, high}, req, nil)
	if err != nil {
		t.Fatalf("MatchRules returned error: %v", err)
	}
	if got == nil {
		t.Fatal("MatchRules returned nil, want route")
	}
	if got.ResponseBody != "high" {
		t.Fatalf("matched response body = %q, want high", got.ResponseBody)
	}
}

func testRoute(method, path string) MockRoute {
	return MockRoute{
		Method:           method,
		Path:             path,
		Enabled:          true,
		RequestMatch:     []byte(`{}`),
		ResponseMode:     ResponseModeBody,
		ResponseStatus:   http.StatusOK,
		ResponseHeaders:  []byte(`{}`),
		ResponseBodyType: ResponseBodyTypeJSON,
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return raw
}
