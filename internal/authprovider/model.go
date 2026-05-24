package authprovider

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	TypeGitHub = "github"
	TypeGitLab = "gitlab"
	TypeGoogle = "google"

	ProviderTypeGithub = TypeGitHub
	ProviderTypeGitlab = TypeGitLab
	ProviderTypeGoogle = TypeGoogle

	SourceDB = "db"
)

var (
	ErrProviderNotFound    = errors.New("auth provider not found")
	ErrProviderTypeInvalid = errors.New("auth provider type is invalid")
	ErrProviderDisabled    = errors.New("auth provider is disabled")
	ErrEmptyClientSecret   = errors.New("client secret is required")
)

// ConfigFieldMeta 描述 extra_config 动态表单字段。
type ConfigFieldMeta struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	DefaultValue string `json:"defaultValue,omitempty"`
	Required     bool   `json:"required,omitempty"`
}

// ProviderTypeMeta 供管理端创建表单使用。
type ProviderTypeMeta struct {
	Type              string            `json:"type"`
	DisplayName       string            `json:"displayName"`
	DefaultScopes     []string          `json:"defaultScopes"`
	ExtraConfigFields []ConfigFieldMeta `json:"extraConfigFields,omitempty"`
}

// Provider 是 API 对外暴露的登录方式配置（不含明文 secret）。
type Provider struct {
	ID                  uuid.UUID       `json:"id"`
	Slug                string          `json:"slug"`
	ProviderType        string          `json:"providerType"`
	Name                string          `json:"name"`
	Enabled             bool            `json:"enabled"`
	ClientID            string          `json:"clientId"`
	ClientSecretMasked  string          `json:"clientSecretMasked"`
	ClientSecretPresent bool            `json:"clientSecretPresent"`
	Scopes              []string        `json:"scopes,omitempty"`
	ExtraConfig         json.RawMessage `json:"extraConfig,omitempty"`
	AutoRegister        bool            `json:"autoRegister"`
	DefaultActive       bool            `json:"defaultActive"`
	SortOrder               int             `json:"sortOrder"`
	CallbackURL             string          `json:"callbackUrl,omitempty"`
	TrustedFrontendOrigins  []string        `json:"trustedFrontendOrigins,omitempty"`
	CreatedAt               time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

// LoginProvider 是登录页公开列表项（无 secret）。
type LoginProvider struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	ProviderType string `json:"providerType"`
	CallbackURL  string `json:"callbackUrl,omitempty"`
	Source       string `json:"source,omitempty"`
}

// LoginProviderItem 与 LoginProvider 同义，供 auth 包引用。
type LoginProviderItem = LoginProvider

// CreateInput POST /auth-providers
type CreateInput struct {
	ProviderType  string          `json:"providerType"`
	Slug          string          `json:"slug"`
	Name          string          `json:"name"`
	ClientID      string          `json:"clientId"`
	ClientSecret  string          `json:"clientSecret"`
	Scopes        []string        `json:"scopes"`
	ExtraConfig   json.RawMessage `json:"extraConfig"`
	AutoRegister  *bool           `json:"autoRegister"`
	DefaultActive *bool           `json:"defaultActive"`
	Enabled       *bool           `json:"enabled"`
	SortOrder              *int     `json:"sortOrder"`
	CallbackURL            string   `json:"callbackUrl"`
	TrustedFrontendOrigins []string `json:"trustedFrontendOrigins"`
}

// UpdateInput PUT /auth-providers/{id}
type UpdateInput struct {
	Slug          string          `json:"slug"`
	Name          string          `json:"name"`
	ClientID      string          `json:"clientId"`
	ClientSecret  *string         `json:"clientSecret"`
	Scopes        []string        `json:"scopes"`
	ExtraConfig   json.RawMessage `json:"extraConfig"`
	AutoRegister  *bool           `json:"autoRegister"`
	DefaultActive *bool           `json:"defaultActive"`
	Enabled       *bool           `json:"enabled"`
	SortOrder              *int     `json:"sortOrder"`
	CallbackURL            string   `json:"callbackUrl"`
	TrustedFrontendOrigins []string `json:"trustedFrontendOrigins"`
}

// TestResult POST /auth-providers/{id}/test
type TestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	AuthURL string `json:"authUrl,omitempty"`
}

// ResolvedProvider 是 OAuth 运行时解析后的配置。
type ResolvedProvider struct {
	ID            string
	Slug          string
	ProviderType  string
	Name          string
	ClientID      string
	ClientSecret  string
	Scopes        []string
	ExtraConfig   map[string]string
	CallbackURL            string
	TrustedFrontendOrigins []string
	AutoRegister           bool
	DefaultActive          bool
	Source                 string // "db"
}

// ExternalProfile 是 IdP 用户资料。
type ExternalProfile struct {
	ExternalID  string
	Login       string
	DisplayName string
	Email       string
	AvatarURL   string
}

// ProviderRow is the DB row for auth_providers (exported for auth package list queries).
type ProviderRow struct {
	ID            uuid.UUID
	Slug          string
	ProviderType  string
	Name          string
	Enabled       bool
	ClientID      string
	ClientSecret  string
	Scopes        []string
	ExtraConfig   []byte
	AutoRegister  bool
	DefaultActive bool
	SortOrder              int
	CallbackURL            string
	TrustedFrontendOrigins []string
	CreatedAt              time.Time
	UpdatedAt     time.Time
}

type providerRow = ProviderRow

func (r *providerRow) toAPI() *Provider {
	out := &Provider{
		ID:                  r.ID,
		Slug:                r.Slug,
		ProviderType:        r.ProviderType,
		Name:                r.Name,
		Enabled:             r.Enabled,
		ClientID:            r.ClientID,
		ClientSecretMasked:  maskKey(r.ClientSecret),
		ClientSecretPresent: r.ClientSecret != "",
		Scopes:              r.Scopes,
		AutoRegister:        r.AutoRegister,
		DefaultActive:       r.DefaultActive,
		SortOrder:              r.SortOrder,
		CallbackURL:            r.CallbackURL,
		TrustedFrontendOrigins: append([]string(nil), r.TrustedFrontendOrigins...),
		CreatedAt:              r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
	}
	if len(r.ExtraConfig) > 0 {
		out.ExtraConfig = json.RawMessage(r.ExtraConfig)
	}
	return out
}

func maskKey(key string) string {
	if key == "" {
		return ""
	}
	runes := []rune(key)
	if len(runes) <= 4 {
		return "••••"
	}
	if len(runes) <= 10 {
		return "••••" + string(runes[len(runes)-2:])
	}
	return string(runes[:3]) + "••••" + string(runes[len(runes)-4:])
}
