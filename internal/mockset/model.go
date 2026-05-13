// Package mockset 维护项目级「命名值集合（Mock Value Sets）」。
//
// 每个 ValueSet 由 (projectID, key) 唯一标识，承载一组 JSON 候选值（字符串、
// 数字、布尔、null、对象、数组）与可选权重。Runner / mockserver 在渲染
// `{{$mock.set.<key>}}` 系列模板时会通过 Service.Lookup 拿到 values +
// weights，并按 random / index / sequential 三种模式取值。
package mockset

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// 包级错误，handler 层映射为可读的 HTTP 状态码。
var (
	// ErrNotFound 表示给定 (project, id) 找不到匹配的命名值集合。
	ErrNotFound = errors.New("未找到命名值集合")
	// ErrKeyConflict 表示同一项目内已经存在相同 key 的活跃记录。
	ErrKeyConflict = errors.New("项目内已存在相同 key 的命名值集合")
	// ErrInvalidKey 表示 key 为空或包含非法字符。
	ErrInvalidKey = errors.New("命名值集合 key 必须匹配 ^[A-Za-z0-9_-]+$")
	// ErrInvalidName 表示 name 字段为空。
	ErrInvalidName = errors.New("命名值集合名称不能为空")
	// ErrEmptyValues 表示 values 列表为空。
	ErrEmptyValues = errors.New("命名值集合 values 至少包含一项")
	// ErrEmptyValueEntry 表示 values 中存在空项或仅空白项。
	ErrEmptyValueEntry = errors.New("命名值集合 values 不能包含空项")
	// ErrValueNotValidJSON 表示某项无法解析为合法 JSON。
	ErrValueNotValidJSON = errors.New("命名值集合某项不是合法 JSON")
	// ErrValueExtraJSON 表示某项在首尾以外含有多余 JSON。
	ErrValueExtraJSON = errors.New("命名值集合某项含有多余 JSON 内容")
	// ErrValueDepthExceeded 表示对象/数组嵌套超过上限。
	ErrValueDepthExceeded = errors.New("命名值集合某项嵌套过深（最多 12 层）")
	// ErrValueTooManyNodes 表示对象/数组节点总数超过上限。
	ErrValueTooManyNodes = errors.New("命名值集合某项过大（最多 256 个节点）")
	// ErrWeightsLengthMismatch 表示 weights 长度与 values 不一致。
	ErrWeightsLengthMismatch = errors.New("命名值集合 weights 长度必须与 values 一致")
	// ErrNegativeWeight 表示 weights 中存在负数。
	ErrNegativeWeight = errors.New("命名值集合权重不能为负数")
)

// ValueSet 是 mock_value_sets 表的领域模型，也是 API 响应体。
type ValueSet struct {
	ID          uuid.UUID           `json:"id"`
	ProjectID   uuid.UUID           `json:"projectId"`
	Key         string              `json:"key"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Values      []json.RawMessage   `json:"values"`
	Weights     []float64           `json:"weights,omitempty"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

// CreateInput 是 POST /projects/{id}/mock-value-sets 的请求体。
type CreateInput struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Values      []json.RawMessage `json:"values"`
	Weights     []float64         `json:"weights"`
}

// UpdateInput 是 PUT /projects/{id}/mock-value-sets/{setID} 的请求体。
//
// key 不可修改：变更 key 等于新建一个集合，否则将影响已经引用旧 key 的
// 模板。需要重命名时请新建一个集合并迁移引用。
type UpdateInput struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Values      []json.RawMessage `json:"values"`
	Weights     []float64         `json:"weights"`
}
