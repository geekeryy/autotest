package specdiff

import (
	"encoding/json"
	"sort"
	"testing"
)

// helper: build a snapshot JSON the same shape as `internal/spec.Importer`
// emits. Each endpoint is a map[string]any so callers can compose minimal
// fixtures inline.
func snapshot(t *testing.T, endpoints ...map[string]any) json.RawMessage {
	t.Helper()
	if endpoints == nil {
		endpoints = []map[string]any{}
	}
	raw, err := json.Marshal(map[string]any{
		"title":     "test",
		"version":   "1.0",
		"endpoints": endpoints,
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	return raw
}

// findChange helps test assertions: it returns the first FieldChange whose
// path matches the given prefix, or fails the test with a descriptive
// message that lists all observed paths for quick debugging.
func findChange(t *testing.T, changes []FieldChange, fieldPath string) FieldChange {
	t.Helper()
	for _, change := range changes {
		if change.FieldPath == fieldPath {
			return change
		}
	}
	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.FieldPath+"("+change.Kind+")")
	}
	sort.Strings(paths)
	t.Fatalf("expected change at %q, got: %v", fieldPath, paths)
	return FieldChange{}
}

func TestDiffSnapshots(t *testing.T) {
	t.Parallel()

	getUserEndpoint := func(extra ...func(map[string]any)) map[string]any {
		ep := map[string]any{
			"method":  "GET",
			"path":    "/api/v1/users/{id}",
			"summary": "Get user",
			"tags":    []any{"users"},
			"requestSchema": map[string]any{
				"parameters": []any{
					map[string]any{
						"name":     "id",
						"in":       "path",
						"required": true,
						"schema":   map[string]any{"type": "string"},
					},
				},
				"body": nil,
			},
			"responseSchema": map[string]any{
				"status": "200",
				"body": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":    map[string]any{"type": "string"},
						"email": map[string]any{"type": "string"},
					},
				},
			},
		}
		for _, mut := range extra {
			mut(ep)
		}
		return ep
	}

	type assertion func(t *testing.T, diff Diff)

	cases := []struct {
		name   string
		prev   json.RawMessage
		curr   json.RawMessage
		assert assertion
	}{
		{
			name: "added endpoint",
			prev: snapshot(t),
			curr: snapshot(t, getUserEndpoint()),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.AddedEndpoints) != 1 {
					t.Fatalf("expected 1 added endpoint, got %d", len(diff.AddedEndpoints))
				}
				if diff.AddedEndpoints[0].Method != "GET" || diff.AddedEndpoints[0].Path != "/api/v1/users/{id}" {
					t.Errorf("unexpected added endpoint: %+v", diff.AddedEndpoints[0])
				}
				if len(diff.RemovedEndpoints) != 0 || len(diff.ModifiedEndpoints) != 0 {
					t.Errorf("expected only an added endpoint, got %+v", diff)
				}
			},
		},
		{
			name: "removed endpoint",
			prev: snapshot(t, getUserEndpoint()),
			curr: snapshot(t),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.RemovedEndpoints) != 1 {
					t.Fatalf("expected 1 removed endpoint, got %d", len(diff.RemovedEndpoints))
				}
				if diff.RemovedEndpoints[0].Path != "/api/v1/users/{id}" {
					t.Errorf("unexpected removed endpoint: %+v", diff.RemovedEndpoints[0])
				}
			},
		},
		{
			name: "method change is treated as remove + add (different endpoint)",
			prev: snapshot(t, getUserEndpoint()),
			curr: snapshot(t, getUserEndpoint(func(ep map[string]any) {
				ep["method"] = "POST"
			})),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.AddedEndpoints) != 1 || diff.AddedEndpoints[0].Method != "POST" {
					t.Errorf("expected POST added, got %+v", diff.AddedEndpoints)
				}
				if len(diff.RemovedEndpoints) != 1 || diff.RemovedEndpoints[0].Method != "GET" {
					t.Errorf("expected GET removed, got %+v", diff.RemovedEndpoints)
				}
				if len(diff.ModifiedEndpoints) != 0 {
					t.Errorf("modified should be empty when method changes, got %+v", diff.ModifiedEndpoints)
				}
			},
		},
		{
			name: "param type change registers under name+in key",
			prev: snapshot(t, getUserEndpoint()),
			curr: snapshot(t, getUserEndpoint(func(ep map[string]any) {
				schema := ep["requestSchema"].(map[string]any)
				params := schema["parameters"].([]any)
				params[0].(map[string]any)["schema"] = map[string]any{"type": "integer"}
			})),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.ModifiedEndpoints) != 1 {
					t.Fatalf("expected 1 modified endpoint, got %d (%+v)", len(diff.ModifiedEndpoints), diff)
				}
				change := findChange(t, diff.ModifiedEndpoints[0].Changes,
					"requestSchema.parameters[id|path].schema.type")
				if change.Kind != ChangeKindChange {
					t.Errorf("expected change kind=change, got %q", change.Kind)
				}
				if change.Before != "string" || change.After != "integer" {
					t.Errorf("expected before=string after=integer, got %v -> %v", change.Before, change.After)
				}
			},
		},
		{
			name: "required field added on response body",
			prev: snapshot(t, getUserEndpoint()),
			curr: snapshot(t, getUserEndpoint(func(ep map[string]any) {
				resp := ep["responseSchema"].(map[string]any)
				body := resp["body"].(map[string]any)
				body["required"] = []any{"id"}
			})),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.ModifiedEndpoints) != 1 {
					t.Fatalf("expected 1 modified endpoint, got %d", len(diff.ModifiedEndpoints))
				}
				change := findChange(t, diff.ModifiedEndpoints[0].Changes,
					"responseSchema.body.required")
				if change.Kind != ChangeKindAdd {
					t.Errorf("expected required to be added, got kind=%q", change.Kind)
				}
			},
		},
		{
			name: "removed nested response property",
			prev: snapshot(t, getUserEndpoint()),
			curr: snapshot(t, getUserEndpoint(func(ep map[string]any) {
				resp := ep["responseSchema"].(map[string]any)
				body := resp["body"].(map[string]any)
				props := body["properties"].(map[string]any)
				delete(props, "email")
			})),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.ModifiedEndpoints) != 1 {
					t.Fatalf("expected 1 modified endpoint")
				}
				change := findChange(t, diff.ModifiedEndpoints[0].Changes,
					"responseSchema.body.properties.email")
				if change.Kind != ChangeKindRemove {
					t.Errorf("expected email removed, got kind=%q", change.Kind)
				}
			},
		},
		{
			name: "no diff when path unchanged and schema identical",
			prev: snapshot(t, getUserEndpoint()),
			curr: snapshot(t, getUserEndpoint()),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.AddedEndpoints) != 0 || len(diff.RemovedEndpoints) != 0 || len(diff.ModifiedEndpoints) != 0 {
					t.Errorf("expected empty diff, got %+v", diff)
				}
			},
		},
		{
			name: "param reorder is not a change",
			prev: snapshot(t, getUserEndpoint(func(ep map[string]any) {
				schema := ep["requestSchema"].(map[string]any)
				schema["parameters"] = []any{
					map[string]any{
						"name": "id", "in": "path", "schema": map[string]any{"type": "string"},
					},
					map[string]any{
						"name": "verbose", "in": "query", "schema": map[string]any{"type": "boolean"},
					},
				}
			})),
			curr: snapshot(t, getUserEndpoint(func(ep map[string]any) {
				schema := ep["requestSchema"].(map[string]any)
				schema["parameters"] = []any{
					map[string]any{
						"name": "verbose", "in": "query", "schema": map[string]any{"type": "boolean"},
					},
					map[string]any{
						"name": "id", "in": "path", "schema": map[string]any{"type": "string"},
					},
				}
			})),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.ModifiedEndpoints) != 0 {
					t.Errorf("reordering parameters must not be reported as a change, got %+v", diff.ModifiedEndpoints)
				}
			},
		},
		{
			name: "parameter required toggle is reported under the param key",
			prev: snapshot(t, getUserEndpoint()),
			curr: snapshot(t, getUserEndpoint(func(ep map[string]any) {
				schema := ep["requestSchema"].(map[string]any)
				params := schema["parameters"].([]any)
				params[0].(map[string]any)["required"] = false
			})),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.ModifiedEndpoints) != 1 {
					t.Fatalf("expected 1 modified endpoint")
				}
				change := findChange(t, diff.ModifiedEndpoints[0].Changes,
					"requestSchema.parameters[id|path].required")
				if change.Kind != ChangeKindChange {
					t.Errorf("expected change kind, got %q", change.Kind)
				}
			},
		},
		{
			name: "empty prev snapshot returns added-only diff",
			prev: nil,
			curr: snapshot(t, getUserEndpoint()),
			assert: func(t *testing.T, diff Diff) {
				if len(diff.AddedEndpoints) != 1 {
					t.Errorf("expected 1 added when prev is nil, got %+v", diff)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			diff, err := DiffSnapshots(tc.prev, tc.curr)
			if err != nil {
				t.Fatalf("DiffSnapshots returned error: %v", err)
			}
			tc.assert(t, diff)
		})
	}
}

func TestDiffSnapshotsMalformedReturnsError(t *testing.T) {
	t.Parallel()

	_, err := DiffSnapshots(json.RawMessage(`{"endpoints": "not-an-array"}`), nil)
	if err == nil {
		t.Fatal("expected error when prev snapshot is malformed")
	}
}
