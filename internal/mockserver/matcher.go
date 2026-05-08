package mockserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
)

// MatchRules returns the first enabled route that matches the request, ordered by priority.
func MatchRules(routes []MockRoute, r *http.Request, body []byte) (*MockRoute, error) {
	ordered := append([]MockRoute(nil), routes...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Priority > ordered[j].Priority
	})
	for i := range ordered {
		ok, err := MatchRoute(ordered[i], r, body)
		if err != nil {
			return nil, err
		}
		if ok {
			return &ordered[i], nil
		}
	}
	return nil, nil
}

// MatchRoute reports whether a route matches an HTTP request and already-read body.
func MatchRoute(route MockRoute, r *http.Request, body []byte) (bool, error) {
	if !route.Enabled {
		return false, nil
	}
	if route.Method != "" && route.Method != "*" && route.Method != "ANY" && strings.ToUpper(route.Method) != r.Method {
		return false, nil
	}
	if !matchPath(route.Path, r.URL.Path) {
		return false, nil
	}
	match, err := decodeRequestMatch(route.RequestMatch)
	if err != nil {
		return false, err
	}
	if !matchQuery(match.Query, r) {
		return false, nil
	}
	if !matchHeaders(match.Headers, r) {
		return false, nil
	}
	if match.BodyContains != "" && !bytes.Contains(body, []byte(match.BodyContains)) {
		return false, nil
	}
	if len(match.BodyJSON) > 0 {
		ok, err := matchBodyJSON(body, match.BodyJSON)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

func decodeRequestMatch(raw json.RawMessage) (RequestMatch, error) {
	if len(raw) == 0 {
		return RequestMatch{}, nil
	}
	var match RequestMatch
	if err := json.Unmarshal(raw, &match); err != nil {
		return RequestMatch{}, fmt.Errorf("decode requestMatch: %w", err)
	}
	return match, nil
}

func matchPath(pattern, actual string) bool {
	_, ok := extractPathVars(pattern, actual)
	return ok
}

func extractPathVars(pattern, actual string) (map[string]string, bool) {
	patternParts := splitPath(pattern)
	actualParts := splitPath(actual)
	if len(patternParts) != len(actualParts) {
		return nil, false
	}
	values := map[string]string{}
	for i, part := range patternParts {
		if isPlaceholder(part) {
			if actualParts[i] == "" {
				return nil, false
			}
			values[placeholderName(part)] = actualParts[i]
			continue
		}
		if part != actualParts[i] {
			return nil, false
		}
	}
	return values, true
}

func splitPath(path string) []string {
	if path == "/" {
		return []string{}
	}
	return strings.Split(strings.Trim(path, "/"), "/")
}

func isPlaceholder(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") && len(segment) > 2
}

func placeholderName(segment string) string {
	return strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
}

func matchQuery(expected map[string]string, r *http.Request) bool {
	for key, want := range expected {
		if r.URL.Query().Get(key) != want {
			return false
		}
	}
	return true
}

func matchHeaders(expected map[string]string, r *http.Request) bool {
	for key, want := range expected {
		if r.Header.Get(key) != want {
			return false
		}
	}
	return true
}

func matchBodyJSON(body []byte, expected map[string]any) (bool, error) {
	var actual any
	if err := json.Unmarshal(body, &actual); err != nil {
		return false, fmt.Errorf("decode request body JSON: %w", err)
	}
	for key, want := range expected {
		got, ok := lookupJSON(actual, key)
		if !ok {
			return false, nil
		}
		if !jsonValueMatches(got, want) {
			return false, nil
		}
	}
	return true, nil
}

func lookupJSON(actual any, key string) (any, bool) {
	if object, ok := actual.(map[string]any); ok {
		if value, exists := object[key]; exists {
			return value, true
		}
	}
	parts := strings.Split(key, ".")
	value := actual
	for _, part := range parts {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return value, true
}

func jsonValueMatches(got, want any) bool {
	wantMap, wantIsMap := want.(map[string]any)
	gotMap, gotIsMap := got.(map[string]any)
	if wantIsMap && gotIsMap {
		for key, nestedWant := range wantMap {
			nestedGot, ok := gotMap[key]
			if !ok || !jsonValueMatches(nestedGot, nestedWant) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(got, want)
}
