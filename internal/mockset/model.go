// Package mockset 维护项目级「命名值集合（Mock Value Sets）」。
//
// 每个 ValueSet 由 (projectID, key) 唯一标识，承载一组离散字符串候选值与
// 可选权重。Runner / mockserver 在渲染 `{{$mock.set.<key>}}` 系列模板时
// 会通过 Service.Lookup 拿到 values + weights，并按 random / index /
// sequential 三种模式取值。
package mockset

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// 包级错误，handler 层映射为可读的 HTTP 状态码。
var (
	// ErrNotFound 表示给定 (project, id) 找不到匹配的命名值集合。
	ErrNotFound = errors.New("mock value set not found")
	// ErrKeyConflict 表示同一项目内已经存在相同 key 的活跃记录。
	ErrKeyConflict = errors.New("mock value set key already exists in project")
	// ErrInvalidKey 表示 key 为空或包含非法字符。
	ErrInvalidKey = errors.New("mock value set key must match ^[A-Za-z0-9_-]+$")
	// ErrInvalidName 表示 name 字段为空。
	ErrInvalidName = errors.New("mock value set name is required")
	// ErrEmptyValues 表示 values 列表为空或所有项均为空字符串。
	ErrEmptyValues = errors.New("mock value set values must contain at least one non-empty entry")
	// ErrEmptyValueEntry 表示 values 中存在空字符串项。
	ErrEmptyValueEntry = errors.New("mock value set values must not contain empty entries")
	// ErrWeightsLengthMismatch 表示 weights 长度与 values 不一致。
	ErrWeightsLengthMismatch = errors.New("mock value set weights length must match values length")
	// ErrNegativeWeight 表示 weights 中存在负数。
	ErrNegativeWeight = errors.New("mock value set weights must be non-negative")
)

// ValueSet 是 mock_value_sets 表的领域模型，也是 API 响应体。
type ValueSet struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"projectId"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Values      []string  `json:"values"`
	Weights     []float64 `json:"weights,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CreateInput 是 POST /projects/{id}/mock-value-sets 的请求体。
type CreateInput struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Values      []string  `json:"values"`
	Weights     []float64 `json:"weights"`
}

// UpdateInput 是 PUT /projects/{id}/mock-value-sets/{setID} 的请求体。
//
// key 不可修改：变更 key 等于新建一个集合，否则将影响已经引用旧 key 的
// 模板。需要重命名时请新建一个集合并迁移引用。
type UpdateInput struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Values      []string  `json:"values"`
	Weights     []float64 `json:"weights"`
}
