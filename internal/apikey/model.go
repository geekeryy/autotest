// Package apikey 提供 API Key 资源的领域模型、数据访问、业务逻辑与 HTTP 接口。
// API Key 用于 CI/CD 等场景以 Authorization: Bearer at-... 形式调用平台的开放接口；
// 数据库中只保存令牌的 SHA-256 哈希与前后缀掩码，明文仅在创建时一次性返回给前端。
package apikey

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ScopeSpecsImport 是当前 MVP 阶段唯一允许的作用域：调用 OpenAPI/Swagger 导入接口。
const ScopeSpecsImport = "specs:import"

// DefaultScopes 是新建 API Key 时的默认作用域集合。
var DefaultScopes = []string{ScopeSpecsImport}

// allowedScopes 限制可写入的作用域白名单；本期固定只允许 specs:import，
// 后续扩展（如 cases:run）时在此追加即可。
var allowedScopes = map[string]struct{}{
	ScopeSpecsImport: {},
}

// 包内可识别的错误，handler 层据此映射 HTTP 状态码。
var (
	ErrNotFound       = errors.New("API Key 不存在")
	ErrTokenInvalid   = errors.New("API Key 无效")
	ErrTokenDisabled  = errors.New("API Key 已禁用")
	ErrTokenExpired   = errors.New("API Key 已过期")
	ErrScopeForbidden = errors.New("API Key 包含不允许的作用域")
)

// APIKey 是面向 HTTP 接口的 API Key 视图，永远不暴露明文 token 或哈希。
type APIKey struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Mask        string     `json:"mask"`
	Scopes      []string   `json:"scopes"`
	Enabled     bool       `json:"enabled"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	CreatedBy   uuid.UUID  `json:"createdBy"`
	CreatorName string     `json:"creatorName,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// CreateResult 是创建接口的一次性返回值：包含明文 token，前端关闭弹窗后即无法再次获取。
type CreateResult struct {
	APIKey APIKey `json:"apiKey"`
	Token  string `json:"token"`
}

// CreateInput 描述创建 API Key 的请求体。
type CreateInput struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// UpdateInput 描述修改 API Key 的可选字段，nil 表示不变更。
type UpdateInput struct {
	Name      *string    `json:"name,omitempty"`
	Enabled   *bool      `json:"enabled,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// ClearExpiresAt 显式置空过期时间，避免与 ExpiresAt nil 的"不变更"语义混淆。
	ClearExpiresAt bool `json:"clearExpiresAt,omitempty"`
}

// record 是仓储层的内部行结构，handler 层只通过 APIKey 视图访问。
type record struct {
	ID          uuid.UUID
	Name        string
	TokenHash   string
	TokenPrefix string
	TokenSuffix string
	Scopes      []string
	CreatedBy   uuid.UUID
	CreatorName string
	Enabled     bool
	ExpiresAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *record) toAPI() APIKey {
	scopes := append([]string{}, r.Scopes...)
	return APIKey{
		ID:          r.ID,
		Name:        r.Name,
		Mask:        mask(r.TokenPrefix, r.TokenSuffix),
		Scopes:      scopes,
		Enabled:     r.Enabled,
		ExpiresAt:   r.ExpiresAt,
		LastUsedAt:  r.LastUsedAt,
		CreatedBy:   r.CreatedBy,
		CreatorName: r.CreatorName,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func mask(prefix, suffix string) string {
	if prefix == "" && suffix == "" {
		return ""
	}
	return prefix + "…" + suffix
}
