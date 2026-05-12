package mockset

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceCreateValidates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   CreateInput
		wantErr error
	}{
		{
			name:    "key empty",
			input:   CreateInput{Name: "Colors", Values: []string{"red"}},
			wantErr: ErrInvalidKey,
		},
		{
			name:    "key invalid char",
			input:   CreateInput{Key: "bad key", Name: "Colors", Values: []string{"red"}},
			wantErr: ErrInvalidKey,
		},
		{
			name:    "name empty",
			input:   CreateInput{Key: "colors", Values: []string{"red"}},
			wantErr: ErrInvalidName,
		},
		{
			name:    "values empty",
			input:   CreateInput{Key: "colors", Name: "Colors", Values: []string{}},
			wantErr: ErrEmptyValues,
		},
		{
			name:    "values has empty entry",
			input:   CreateInput{Key: "colors", Name: "Colors", Values: []string{"red", "  "}},
			wantErr: ErrEmptyValueEntry,
		},
		{
			name:    "weights length mismatch",
			input:   CreateInput{Key: "colors", Name: "Colors", Values: []string{"red", "green"}, Weights: []float64{1.0}},
			wantErr: ErrWeightsLengthMismatch,
		},
		{
			name:    "weights negative",
			input:   CreateInput{Key: "colors", Name: "Colors", Values: []string{"red"}, Weights: []float64{-1.0}},
			wantErr: ErrNegativeWeight,
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

func TestServiceCreateAndLookup(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewServiceWithRepository(repo)
	projectID := uuid.New()

	created, err := svc.Create(context.Background(), projectID, CreateInput{
		Key:    "colors",
		Name:   "Colors",
		Values: []string{"red", "green", "blue"},
		// 含两端空格，应被 normalize 去掉
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
	if len(values) != 3 || values[0] != "red" {
		t.Fatalf("unexpected lookup values: %#v", values)
	}
	if len(weights) != 3 || weights[0] != 0.7 {
		t.Fatalf("unexpected lookup weights: %#v", weights)
	}

	// Lookup of unknown key returns ok=false rather than error so callers can
	// surface their own descriptive message.
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
		Key: "colors", Name: "Colors", Values: []string{"red"},
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = svc.Create(context.Background(), projectID, CreateInput{
		Key: "colors", Name: "Other", Values: []string{"yellow"},
	})
	if !errors.Is(err, ErrKeyConflict) {
		t.Fatalf("expected ErrKeyConflict on duplicate key, got %v", err)
	}
}

// TestServiceCreateAfterSoftDeleteAllowsKeyReuse 验证软删除后允许同 key 重建：
// 表上的 unique partial index 仅约束 deleted_at IS NULL 的行。
func TestServiceCreateAfterSoftDeleteAllowsKeyReuse(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository()
	svc := NewServiceWithRepository(repo)
	projectID := uuid.New()

	created, err := svc.Create(context.Background(), projectID, CreateInput{
		Key: "colors", Name: "Colors", Values: []string{"red"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.Delete(context.Background(), projectID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// 重建同 key 应当成功
	if _, err := svc.Create(context.Background(), projectID, CreateInput{
		Key: "colors", Name: "Colors v2", Values: []string{"yellow"},
	}); err != nil {
		t.Fatalf("recreate after delete: %v", err)
	}
}

// fakeRepository is an in-memory stand-in for Repository used by service tests.
// It models the partial unique index by checking for an *active* row with the
// same (projectID, key) and treats soft deletes as removing that constraint.
type fakeRepository struct {
	rows map[uuid.UUID]*ValueSet
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{rows: map[uuid.UUID]*ValueSet{}}
}

func (f *fakeRepository) ListByProject(_ context.Context, projectID uuid.UUID) ([]ValueSet, error) {
	out := []ValueSet{}
	for _, row := range f.rows {
		if row.ProjectID == projectID {
			clone := *row
			out = append(out, clone)
		}
	}
	return out, nil
}

func (f *fakeRepository) GetByKey(_ context.Context, projectID uuid.UUID, key string) (*ValueSet, error) {
	for _, row := range f.rows {
		if row.ProjectID == projectID && row.Key == key {
			clone := *row
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
		Values:      append([]string(nil), input.Values...),
		Weights:     append([]float64(nil), input.Weights...),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	f.rows[row.ID] = row
	clone := *row
	return &clone, nil
}

func (f *fakeRepository) Update(_ context.Context, projectID, setID uuid.UUID, input UpdateInput) (*ValueSet, error) {
	row, ok := f.rows[setID]
	if !ok || row.ProjectID != projectID {
		return nil, ErrNotFound
	}
	row.Name = input.Name
	row.Description = input.Description
	row.Values = append([]string(nil), input.Values...)
	row.Weights = append([]float64(nil), input.Weights...)
	row.UpdatedAt = time.Now().UTC()
	clone := *row
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
