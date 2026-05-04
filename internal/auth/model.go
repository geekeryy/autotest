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
)

type User struct {
	ID           uuid.UUID    `json:"id"`
	Username     string       `json:"username"`
	PasswordHash string       `json:"-"`
	DisplayName  string       `json:"displayName"`
	Email        string       `json:"email"`
	Active       bool         `json:"active"`
	Roles        []Role       `json:"roles,omitempty"`
	Permissions  []Permission `json:"permissions,omitempty"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
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
	Username    string      `json:"username"`
	Password    string      `json:"password"`
	DisplayName string      `json:"displayName"`
	Email       string      `json:"email"`
	Active      *bool       `json:"active"`
	RoleIDs     []uuid.UUID `json:"roleIds"`
}

type UpdateUserInput struct {
	Password    string      `json:"password"`
	DisplayName string      `json:"displayName"`
	Email       string      `json:"email"`
	Active      *bool       `json:"active"`
	RoleIDs     []uuid.UUID `json:"roleIds"`
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
	Permissions map[string]struct{}
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
	{PermissionSpecsImport, "导入接口文档", "导入 Swagger/OpenAPI 文档"},
	{PermissionCasesRead, "查看用例", "查看测试用例"},
	{PermissionCasesWrite, "管理用例", "创建和维护测试用例"},
	{PermissionSuitesRead, "查看测试集", "查看测试集"},
	{PermissionSuitesWrite, "管理测试集", "创建测试集并添加用例"},
	{PermissionUsersManage, "管理用户", "用户管理接口权限"},
	{PermissionRolesManage, "管理角色", "角色管理接口权限"},
	{PermissionPermissionsManage, "管理权限", "权限点管理接口权限"},
}
