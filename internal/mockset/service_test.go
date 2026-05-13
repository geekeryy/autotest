package mockset

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func mustMarshalJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(b)
}

func TestServiceCreateValidates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   CreateInput
		wantErr error
	}{
		{
			name:    "key empty",
			input:   CreateInput{Name: "Colors", Values: []json.RawMessage{mustMarshalJSON(t, "red")}},
			wantErr: ErrInvalidKey,
		},
		{
			name:    "key invalid char",
			input:   CreateInput{Key: "bad key", Name: "Colors", Values: []json.RawMessage{mustMarshalJSON(t, "red")}},
			wantErr: ErrInvalidKey,
		},
		{
			name:    "name empty",
			input:   CreateInput{Key: "colors", Values: []json.RawMessage{mustMarshalJSON(t, "red")}},
			wantErr: ErrInvalidName,
		},
		{
			name:    "values empty",
			input:   CreateInput{Key: "colors", Name: "Colors", Values: []json.RawMessage{}},
			wantErr: ErrEmptyValues,
		},
		{
			name:    "values has empty entry",
			input:   CreateInput{Key: "colors", Name: "Colors", Values: []json.RawMessage{mustMarshalJSON(t, "red"), json.RawMessage("  ")}},
			wantErr: ErrEmptyValueEntry,
		},
		{
			name:    "weights length mismatch",
			input: CreateInput{
				Key:     "colors",
				Name:    "Colors",
				Values:  []json.RawMessage{mustMarshalJSON(t, "red"), mustMarshalJSON(t, "green")},
				Weights: []float64{1.0},
			},
			wantErr: ErrWeightsLengthMismatch,
		},
		{
			name: "weights negative",
			input: CreateInput{
				Key:     "colors",
				Name:    "Colors",
				Values:  []json.RawMessage{mustMarshalJSON(t, "red")},
				Weights: []float64{-1.0},
			},
			wantErr: ErrNegativeWeight,
		},
		{
			name: "invalid json fragment",
			input: CreateInput{
				Key:    "colors",
				Name:   "Colors",
				Values: []json.RawMessage{json.RawMessage(`{"a":`)},
			},
			wantErr: ErrValueNotValidJSON,
		},
		{
			name: "depth exceeded",
			input: CreateInput{
				Key:    "deep",
				Name:   "Deep",
				Values: []json.RawMessage{json.RawMessage(nestedBracketArray(12))},
			},
			wantErr: ErrValueDepthExceeded,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := NewServiceWithRepository(newFakeRepository())
			_, err := svc.Create(context.Background(), uuid.New(), tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// nestedBracketArray 返回 JSON：depth 层嵌套数组包裹标量 0，例如 depth=2 → [[0]]。
func nestedBracketArray(depth int) string {
	return strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth)
}

func TestServiceCreateMixedJSONValues(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewServiceWithRepository(repo)
	projectID := uuid.New()

	mixed := []json.RawMessage{
		mustMarshalJSON(t, "red"),
		json.RawMessage(`42`),
		json.RawMessage(`true`),
		json.RawMessage(`null`),
		mustMarshalJSON(t, map[string]any{"k": 1}),
		mustMarshalJSON(t, []any{1, "two"}),
	}

	created, err := svc.Create(context.Background(), projectID, CreateInput{
		Key:     "mix",
		Name:    "Mixed",
		Values:  mixed,
		Weights: []float64{1, 1, 1, 1, 1, 1},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(created.Values) != 6 {
		t.Fatalf("len values: %d", len(created.Values))
	}
	// 规范化为紧凑 JSON
	if string(created.Values[4]) != `{"k":1}` {
		t.Fatalf("object compact: %s", created.Values[4])
	}

	values, weights, ok := svc.Lookup(context.Background(), projectID, "mix")
	if !ok || len(values) != 6 {
		t.Fatalf("lookup")
	}
	if len(weights) != 6 {
		t.Fatalf("weights")
	}
}

func TestServiceCreateAndLookup(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewServiceWithRepository(repo)
	projectID := uuid.New()

	created, err := svc.Create(context.Background(), projectID, CreateInput{
		Key:  "colors",
		Name: "Colors",
		Values: []json.RawMessage{
			mustMarshalJSON(t, "red"),
			mustMarshalJSON(t, "green"),
			mustMarshalJSON(t, "blue"),
		},
		Weights: []float64{0.7, 0.2, 0.1},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Key != "colors" || len(created.Values) != 3 {
		t.Fatalf("unexpected created record: %#v", created)
	}

	values, weights, ok := svc.Lookup(context.Background(), projectID, "colors")
	if !ok {
		t.Fatalf("expected lookup to succeed")
	}
	if len(values) != 3 || string(values[0]) != `"red"` {
		t.Fatalf("unexpected lookup values: %#v", values)
	}
	if len(weights) != 3 || weights[0] != 0.7 {
		t.Fatalf("unexpected lookup weights: %#v", weights)
	}

	if _, _, ok := svc.Lookup(context.Background(), projectID, "missing"); ok {
		t.Fatalf("expected miss for unknown key")
	}
}

func TestServiceCreateRejectsDuplicateKey(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewServiceWithRepository(repo)
	projectID := uuid.New()

	_, err := svc.Create(context.Background(), projectID, CreateInput{
		Key:    "colors",
		Name:   "Colors",
		Values: []json.RawMessage{mustMarshalJSON(t, "red")},
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = svc.Create(context.Background(), projectID, CreateInput{
		Key:    "colors",
		Name:   "Other",
		Values: []json.RawMessage{mustMarshalJSON(t, "yellow")},
	})
	if !errors.Is(err, ErrKeyConflict) {
		t.Fatalf("expected ErrKeyConflict on duplicate key, got %v", err)
	}
}

func TestServiceCreateAfterSoftDeleteAllowsKeyReuse(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewServiceWithRepository(repo)
	projectID := uuid.New()

	created, err := svc.Create(context.Background(), projectID, CreateInput{
		Key:    "colors",
		Name:   "Colors",
		Values: []json.RawMessage{mustMarshalJSON(t, "red")},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Delete(context.Background(), projectID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Create(context.Background(), projectID, CreateInput{
		Key:    "colors",
		Name:   "Colors v2",
		Values: []json.RawMessage{mustMarshalJSON(t, "yellow")},
	}); err != nil {
		t.Fatalf("recreate after delete: %v", err)
	}
}

type fakeRepository struct {
	rows map[uuid.UUID]*ValueSet
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{rows: map[uuid.UUID]*ValueSet{}}
}

func cloneRawMessages(in []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, len(in))
	for i, r := range in {
		out[i] = append(json.RawMessage(nil), r...)
	}
	return out
}

func (f *fakeRepository) ListByProject(_ context.Context, projectID uuid.UUID) ([]ValueSet, error) {
	out := []ValueSet{}
	for _, row := range f.rows {
		if row.ProjectID == projectID {
			clone := *row
			clone.Values = cloneRawMessages(row.Values)
			out = append(out, clone)
		}
	}
	return out, nil
}

func (f *fakeRepository) GetByKey(_ context.Context, projectID uuid.UUID, key string) (*ValueSet, error) {
	for _, row := range f.rows {
		if row.ProjectID == projectID && row.Key == key {
			clone := *row
			clone.Values = cloneRawMessages(row.Values)
			return &clone, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepository) Get(_ context.Context, projectID, setID uuid.UUID) (*ValueSet, error) {
	row, ok := f.rows[setID]
	if !ok || row.ProjectID != projectID {
		return nil, ErrNotFound
	}
	clone := *row
	clone.Values = cloneRawMessages(row.Values)
	return &clone, nil
}

func (f *fakeRepository) Create(_ context.Context, projectID uuid.UUID, input CreateInput) (*ValueSet, error) {
	for _, row := range f.rows {
		if row.ProjectID == projectID && row.Key == input.Key {
			return nil, ErrKeyConflict
		}
	}
	now := time.Now().UTC()
	row := &ValueSet{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Key:         input.Key,
		Name:        input.Name,
		Description: input.Description,
		Values:      cloneRawMessages(input.Values),
		Weights:     append([]float64(nil), input.Weights...),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	f.rows[row.ID] = row
	clone := *row
	clone.Values = cloneRawMessages(row.Values)
	return &clone, nil
}

func (f *fakeRepository) Update(_ context.Context, projectID, setID uuid.UUID, input UpdateInput) (*ValueSet, error) {
	row, ok := f.rows[setID]
	if !ok || row.ProjectID != projectID {
		return nil, ErrNotFound
	}
	row.Name = input.Name
	row.Description = input.Description
	row.Values = cloneRawMessages(input.Values)
	row.Weights = append([]float64(nil), input.Weights...)
	row.UpdatedAt = time.Now().UTC()
	clone := *row
	clone.Values = cloneRawMessages(row.Values)
	return &clone, nil
}

func (f *fakeRepository) Delete(_ context.Context, projectID, setID uuid.UUID) error {
	row, ok := f.rows[setID]
	if !ok || row.ProjectID != projectID {
		return ErrNotFound
	}
	delete(f.rows, setID)
	return nil
}
