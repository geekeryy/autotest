package auth

import (
	"time"

	"github.com/google/uuid"
)

const (
	PermissionProjectsRead      = "projects:read"
	PermissionProjectsWrite     = "projects:write"
	PermissionServicesWrite     = "services:write"
	PermissionSpecsImport       = "specs:import"
	PermissionCasesRead         = "cases:read"
	PermissionCasesWrite        = "cases:write"
	PermissionSuitesRead        = "suites:read"
	PermissionSuitesWrite       = "suites:write"
	PermissionUsersManage       = "users:manage"
	PermissionRolesManage       = "roles:manage"
	PermissionPermissionsManage = "permissions:manage"
	PermissionAPIKeysManage     = "apikeys:manage"
)

// 认证来源：JWT 走原有用户名密码登录流程，APIKey 走 internal/apikey 校验。
const (
	SourceJWT    = "jwt"
	SourceAPIKey = "apikey"
)

const (
	AuthProviderLocal  = "local"
	AuthProviderGithub = "github"
)

type User struct {
	ID           uuid.UUID    `json:"id"`
	Username     string       `json:"username"`
	PasswordHash string       `json:"-"`
	AuthProvider string       `json:"authProvider"`
	GithubID     *int64       `json:"githubId,omitempty"`
	DisplayName  string       `json:"displayName"`
	Email        string       `json:"email"`
	Active       bool         `json:"active"`
	AvatarURL    string       `json:"avatarUrl,omitempty"`
	Roles        []Role       `json:"roles,omitempty"`
	Permissions  []Permission `json:"permissions,omitempty"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
	// avatarJPEG 仅包内使用，由 repository 扫描并生成 AvatarURL（data URL，便于 Bearer 鉴权下展示）。
	avatarJPEG []byte `json:"-"`
}

type GithubUserInput struct {
	Username    string
	DisplayName string
	Email       string
	GithubID    int64
	Active      bool
	AvatarJPEG  []byte
}

type Role struct {
	ID          uuid.UUID    `json:"id"`
	Code        string       `json:"code"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type Permission struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

type CreateUserInput struct {
	Username     string      `json:"username"`
	Password     string      `json:"password"`
	DisplayName  string      `json:"displayName"`
	Email        string      `json:"email"`
	Active       *bool       `json:"active"`
	RoleIDs      []uuid.UUID `json:"roleIds"`
	AvatarBase64 string      `json:"avatarBase64,omitempty"`
}

type UpdateUserInput struct {
	Password     string      `json:"password"`
	DisplayName  string      `json:"displayName"`
	Email        string      `json:"email"`
	Active       *bool       `json:"active"`
	RoleIDs      []uuid.UUID `json:"roleIds"`
	AvatarBase64 string      `json:"avatarBase64,omitempty"`
	ClearAvatar  bool        `json:"clearAvatar,omitempty"`
}

type CreateRoleInput struct {
	Code          string      `json:"code"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	PermissionIDs []uuid.UUID `json:"permissionIds"`
}

type UpdateRoleInput struct {
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	PermissionIDs []uuid.UUID `json:"permissionIds"`
}

type CreatePermissionInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SetRolePermissionsInput struct {
	PermissionIDs []uuid.UUID `json:"permissionIds"`
}

type Principal struct {
	UserID      uuid.UUID
	Username    string
	Active      bool
	Permissions map[string]struct{}
	// Source 区分调用来源：SourceJWT 或 SourceAPIKey；默认空值视为 SourceJWT。
	Source string
	// Scopes 仅在 Source 为 SourceAPIKey 时由 internal/apikey 注入，
	// 用于在路由层判断 API Key 是否被允许访问当前接口。
	Scopes map[string]struct{}
}

type defaultPermission struct {
	Code        string
	Name        string
	Description string
}

var defaultPermissions = []defaultPermission{
	{PermissionProjectsRead, "查看项目", "查看项目、服务和环境"},
	{PermissionProjectsWrite, "管理项目", "创建项目、服务和环境"},
	{PermissionServicesWrite, "管理服务环境", "管理服务与环境配置"},
	{PermissionSpecsImport, "API文档导入", "在 API管理中导入 OpenAPI/Swagger 文档"},
	{PermissionCasesRead, "查看API管理", "查看 API管理页面和接口请求模板"},
	{PermissionCasesWrite, "管理接口模板", "在 API管理中创建与维护接口请求模板"},
	{PermissionSuitesRead, "查看历史测试集", "查看已废弃的历史测试集数据"},
	{PermissionSuitesWrite, "管理历史测试集", "维护已废弃的历史测试集数据"},
	{PermissionUsersManage, "管理用户", "用户管理接口权限"},
	{PermissionRolesManage, "管理角色", "角色管理接口权限"},
	{PermissionPermissionsManage, "管理权限", "权限点管理接口权限"},
	{PermissionAPIKeysManage, "管理API Key", "创建、禁用、删除 API Key（用于 CI/CD 调用平台开放接口）"},
}
