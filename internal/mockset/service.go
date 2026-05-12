package mockset

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// keyPattern restricts set keys to letters, digits, underscore and hyphen so
// they remain safe to embed inside `{{$mock.set.<key>}}` placeholders.
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// repositoryAPI is the persistence-facing slice of the package; defined here
// so service tests can substitute an in-memory fake without touching pgx.
type repositoryAPI interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]ValueSet, error)
	GetByKey(ctx context.Context, projectID uuid.UUID, key string) (*ValueSet, error)
	Get(ctx context.Context, projectID, setID uuid.UUID) (*ValueSet, error)
	Create(ctx context.Context, projectID uuid.UUID, input CreateInput) (*ValueSet, error)
	Update(ctx context.Context, projectID, setID uuid.UUID, input UpdateInput) (*ValueSet, error)
	Delete(ctx context.Context, projectID, setID uuid.UUID) error
}

// Service applies validation and orchestrates persistence for mock value sets.
type Service struct {
	repo repositoryAPI
}

// NewService constructs a Service backed by the concrete pgx Repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// NewServiceWithRepository lets tests inject a fake repository implementing
// repositoryAPI without depending on pgx.
func NewServiceWithRepository(repo repositoryAPI) *Service {
	return &Service{repo: repo}
}

// List returns every active value set for a project, sorted by key.
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]ValueSet, error) {
	if projectID == uuid.Nil {
		return nil, ErrNotFound
	}
	return s.repo.ListByProject(ctx, projectID)
}

// Create validates the input and inserts a new active record.
func (s *Service) Create(ctx context.Context, projectID uuid.UUID, input CreateInput) (*ValueSet, error) {
	normalizeCreate(&input)
	if err := validateKey(input.Key); err != nil {
		return nil, err
	}
	if err := validateName(input.Name); err != nil {
		return nil, err
	}
	values, err := normalizeValues(input.Values)
	if err != nil {
		return nil, err
	}
	weights, err := normalizeWeights(input.Weights, len(values))
	if err != nil {
		return nil, err
	}
	input.Values = values
	input.Weights = weights
	return s.repo.Create(ctx, projectID, input)
}

// Update mutates an existing active record. key 不可修改。
func (s *Service) Update(ctx context.Context, projectID, setID uuid.UUID, input UpdateInput) (*ValueSet, error) {
	normalizeUpdate(&input)
	if err := validateName(input.Name); err != nil {
		return nil, err
	}
	values, err := normalizeValues(input.Values)
	if err != nil {
		return nil, err
	}
	weights, err := normalizeWeights(input.Weights, len(values))
	if err != nil {
		return nil, err
	}
	input.Values = values
	input.Weights = weights
	return s.repo.Update(ctx, projectID, setID, input)
}

// Delete soft-deletes a record.
func (s *Service) Delete(ctx context.Context, projectID, setID uuid.UUID) error {
	return s.repo.Delete(ctx, projectID, setID)
}

// Lookup returns the values + weights for a (projectID, key). The bool is
// false when the set does not exist or is soft-deleted; templating callers
// surface a descriptive error in that case.
//
// Lookup is the only entry point templating / runner / mockserver use to
// resolve set tokens, keeping the dependency direction one-way: the
// templating package never imports mockset directly.
func (s *Service) Lookup(ctx context.Context, projectID uuid.UUID, key string) (values []string, weights []float64, ok bool) {
	if projectID == uuid.Nil || strings.TrimSpace(key) == "" {
		return nil, nil, false
	}
	set, err := s.repo.GetByKey(ctx, projectID, key)
	if err != nil || set == nil {
		return nil, nil, false
	}
	if len(set.Values) == 0 {
		return nil, nil, false
	}
	return append([]string(nil), set.Values...), append([]float64(nil), set.Weights...), true
}

// SummariesForProject 返回当前项目所有命名值集合的精简摘要，供 AI prompt
// 注入。每项保留 key + name + 前 10 条 values + 是否含 weights + 总数；
// values 完整列表对模型来说既冗余又可能撑爆 prompt 长度，因此截断。
func (s *Service) SummariesForProject(ctx context.Context, projectID uuid.UUID) []SetSummary {
	if projectID == uuid.Nil {
		return nil
	}
	rows, err := s.repo.ListByProject(ctx, projectID)
	if err != nil || len(rows) == 0 {
		return nil
	}
	out := make([]SetSummary, 0, len(rows))
	for _, row := range rows {
		preview := row.Values
		if len(preview) > 10 {
			preview = preview[:10]
		}
		// 拷贝一份避免调用方修改 repo 返回的切片。
		previewCopy := append([]string(nil), preview...)
		out = append(out, SetSummary{
			Key:            row.Key,
			Name:           row.Name,
			ValuesPreview:  previewCopy,
			HasWeights:     len(row.Weights) > 0,
			TotalValueSize: len(row.Values),
		})
	}
	return out
}

// SetSummary 是 SummariesForProject 的返回元素，字段命名与
// `aiprovider.MockSetSummary` 对齐，便于 main.go 通过适配函数桥接两边。
type SetSummary struct {
	Key            string
	Name           string
	ValuesPreview  []string
	HasWeights     bool
	TotalValueSize int
}

func normalizeCreate(input *CreateInput) {
	input.Key = strings.TrimSpace(input.Key)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
}

func normalizeUpdate(input *UpdateInput) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
}

func validateKey(key string) error {
	if key == "" || !keyPattern.MatchString(key) {
		return ErrInvalidKey
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return ErrInvalidName
	}
	return nil
}

// normalizeValues trims whitespace per entry. Empty entries are rejected
// rather than silently dropped so the validation error fires instead of
// surprising the user with a shorter list.
func normalizeValues(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, ErrEmptyValues
	}
	out := make([]string, 0, len(in))
	hasContent := false
	for _, v := range in {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, ErrEmptyValueEntry
		}
		out = append(out, trimmed)
		hasContent = true
	}
	if !hasContent {
		return nil, ErrEmptyValues
	}
	return out, nil
}

// normalizeWeights enforces the contract: empty == uniform random,
// otherwise length must equal valuesLen and every entry must be non-negative.
func normalizeWeights(in []float64, valuesLen int) ([]float64, error) {
	if len(in) == 0 {
		return []float64{}, nil
	}
	if len(in) != valuesLen {
		return nil, ErrWeightsLengthMismatch
	}
	out := make([]float64, len(in))
	for i, w := range in {
		if w < 0 {
			return nil, ErrNegativeWeight
		}
		out[i] = w
	}
	return out, nil
}
