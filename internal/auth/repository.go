package auth

import (
	"context"
	"encoding/base64"
	"fmt"

	"autotest/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type Repository struct {
	store.Repository
}

type repositoryBackend interface {
	EnsureDefaults(ctx context.Context, adminUsername, adminPassword string) error
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByGithubID(ctx context.Context, githubID int64) (*User, error)
	AllocateGithubUsername(ctx context.Context, base string) (string, error)
	CreateGithubUser(ctx context.Context, input GithubUserInput) (*User, error)
	UpdateGithubUserProfile(ctx context.Context, id uuid.UUID, displayName, email string, avatarJPEG []byte) (*User, error)
	ListUserIDsByPermission(ctx context.Context, permissionCode string) ([]uuid.UUID, error)
	GetUser(ctx context.Context, id uuid.UUID) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	CreateUser(ctx context.Context, input CreateUserInput, avatarJPEG []byte) (*User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserInput, touchAvatar, clearAvatar bool, avatarJPEG []byte) (*User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	ListRoles(ctx context.Context) ([]Role, error)
	CreateRole(ctx context.Context, input CreateRoleInput) (*Role, error)
	UpdateRole(ctx context.Context, id uuid.UUID, input UpdateRoleInput) (*Role, error)
	DeleteRole(ctx context.Context, id uuid.UUID) error
	SetRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (*Role, error)
	ListPermissions(ctx context.Context) ([]Permission, error)
	CreatePermission(ctx context.Context, input CreatePermissionInput) (*Permission, error)
}

func NewRepository(repo store.Repository) *Repository {
	return &Repository{Repository: repo}
}

const userSelectColumns = `
	id, username, password_hash, auth_provider, github_id,
	display_name, email, active, avatar_jpeg, created_at, updated_at
`

func (r *Repository) EnsureDefaults(ctx context.Context, adminUsername, adminPassword string) error {
	if r.DB == nil {
		return fmt.Errorf("database unavailable")
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin auth defaults: %w", err)
	}
	defer tx.Rollback(ctx)

	permissionIDs := make([]uuid.UUID, 0, len(defaultPermissions))
	for _, permission := range defaultPermissions {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			insert into permissions (code, name, description)
			values ($1, $2, $3)
			on conflict (code)
			do update set name = excluded.name, description = excluded.description, updated_at = now()
			returning id
		`, permission.Code, permission.Name, permission.Description).Scan(&id); err != nil {
			return fmt.Errorf("upsert permission %s: %w", permission.Code, err)
		}
		permissionIDs = append(permissionIDs, id)
	}

	var roleID uuid.UUID
	if err := tx.QueryRow(ctx, `
		insert into roles (code, name, description)
		values ('admin', '系统管理员', '拥有全部后台管理权限')
		on conflict (code)
		do update set name = excluded.name, description = excluded.description, updated_at = now()
		returning id
	`).Scan(&roleID); err != nil {
		return fmt.Errorf("upsert admin role: %w", err)
	}

	for _, permissionID := range permissionIDs {
		if _, err := tx.Exec(ctx, `
			insert into role_permissions (role_id, permission_id)
			values ($1, $2)
			on conflict do nothing
		`, roleID, permissionID); err != nil {
			return fmt.Errorf("attach admin permission: %w", err)
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash default admin password: %w", err)
	}

	var userID uuid.UUID
	// 冲突时必须写入 excluded.password_hash：否则仅首次插入时密码正确，
	// 已存在用户时每次启动生成的新哈希被丢弃，会导致默认 admin/admin123（或 .env 配置）无法登录。
	if err := tx.QueryRow(ctx, `
		insert into users (username, password_hash, display_name, active)
		values ($1, $2, '系统管理员', true)
		on conflict (username)
		do update set
			password_hash = excluded.password_hash,
			active = true,
			updated_at = now()
		returning id
	`, adminUsername, string(hash)).Scan(&userID); err != nil {
		return fmt.Errorf("upsert default admin: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into user_roles (user_id, role_id)
		values ($1, $2)
		on conflict do nothing
	`, userID, roleID); err != nil {
		return fmt.Errorf("attach default admin role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit auth defaults: %w", err)
	}
	return nil
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := r.DB.QueryRow(ctx, `
		select `+userSelectColumns+`
		from users
		where username = $1
	`, username)
	return scanUser(row)
}

func (r *Repository) GetUserByGithubID(ctx context.Context, githubID int64) (*User, error) {
	row := r.DB.QueryRow(ctx, `
		select `+userSelectColumns+`
		from users
		where github_id = $1
	`, githubID)
	return scanUser(row)
}

func (r *Repository) AllocateGithubUsername(ctx context.Context, base string) (string, error) {
	candidate := base
	for i := 0; i < 100; i++ {
		var exists bool
		if err := r.DB.QueryRow(ctx, `select exists(select 1 from users where username = $1)`, candidate).Scan(&exists); err != nil {
			return "", fmt.Errorf("check github username: %w", err)
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s_%d", base, i+1)
	}
	return "", fmt.Errorf("unable to allocate github username for %s", base)
}

func (r *Repository) CreateGithubUser(ctx context.Context, input GithubUserInput) (*User, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create github user: %w", err)
	}
	defer tx.Rollback(ctx)

	var avatarArg any
	if len(input.AvatarJPEG) > 0 {
		avatarArg = input.AvatarJPEG
	}

	user, err := scanUser(tx.QueryRow(ctx, `
		insert into users (username, display_name, email, active, auth_provider, github_id, avatar_jpeg)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning `+userSelectColumns+`
	`, input.Username, input.DisplayName, input.Email, input.Active, AuthProviderGithub, input.GithubID, avatarArg))
	if err != nil {
		return nil, fmt.Errorf("create github user: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create github user: %w", err)
	}
	return r.GetUser(ctx, user.ID)
}

func (r *Repository) UpdateGithubUserProfile(ctx context.Context, id uuid.UUID, displayName, email string, avatarJPEG []byte) (*User, error) {
	if len(avatarJPEG) > 0 {
		if _, err := r.DB.Exec(ctx, `
			update users
			set display_name = $2, email = $3, avatar_jpeg = $4, updated_at = now()
			where id = $1
		`, id, displayName, email, avatarJPEG); err != nil {
			return nil, fmt.Errorf("update github user profile: %w", err)
		}
	} else if _, err := r.DB.Exec(ctx, `
		update users
		set display_name = $2, email = $3, updated_at = now()
		where id = $1
	`, id, displayName, email); err != nil {
		return nil, fmt.Errorf("update github user profile: %w", err)
	}
	return r.GetUser(ctx, id)
}

func (r *Repository) ListUserIDsByPermission(ctx context.Context, permissionCode string) ([]uuid.UUID, error) {
	rows, err := r.DB.Query(ctx, `
		select distinct u.id
		from users u
		join user_roles ur on ur.user_id = u.id
		join role_permissions rp on rp.role_id = ur.role_id
		join permissions p on p.id = rp.permission_id
		where p.code = $1 and u.active = true
	`, permissionCode)
	if err != nil {
		return nil, fmt.Errorf("list user ids by permission: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	user, err := scanUser(r.DB.QueryRow(ctx, `
		select `+userSelectColumns+`
		from users
		where id = $1
	`, id))
	if err != nil {
		return nil, err
	}
	if err := r.loadUserAccess(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (r *Repository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.DB.Query(ctx, `
		select `+userSelectColumns+`
		from users
		order by created_at desc
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		if err := r.loadUserAccess(ctx, user); err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

func (r *Repository) CreateUser(ctx context.Context, input CreateUserInput, avatarJPEG []byte) (*User, error) {
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create user: %w", err)
	}
	defer tx.Rollback(ctx)

	var avatarArg any
	if len(avatarJPEG) > 0 {
		avatarArg = avatarJPEG
	}

	user, err := scanUser(tx.QueryRow(ctx, `
		insert into users (username, password_hash, display_name, email, active, auth_provider, avatar_jpeg)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning `+userSelectColumns+`
	`, input.Username, string(hash), input.DisplayName, input.Email, active, AuthProviderLocal, avatarArg))
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	if err := replaceUserRoles(ctx, tx, user.ID, input.RoleIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create user: %w", err)
	}
	return r.GetUser(ctx, user.ID)
}

func (r *Repository) UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserInput, touchAvatar, clearAvatar bool, avatarJPEG []byte) (*User, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update user: %w", err)
	}
	defer tx.Rollback(ctx)

	active := true
	if input.Active != nil {
		active = *input.Active
	} else {
		if err := tx.QueryRow(ctx, `select active from users where id = $1`, id).Scan(&active); err != nil {
			return nil, fmt.Errorf("load user active: %w", err)
		}
	}

	if input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			update users
			set password_hash = $2, display_name = $3, email = $4, active = $5, updated_at = now()
			where id = $1
		`, id, string(hash), input.DisplayName, input.Email, active); err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
	} else if _, err := tx.Exec(ctx, `
		update users
		set display_name = $2, email = $3, active = $4, updated_at = now()
		where id = $1
	`, id, input.DisplayName, input.Email, active); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	if touchAvatar {
		if clearAvatar {
			if _, err := tx.Exec(ctx, `update users set avatar_jpeg = null, updated_at = now() where id = $1`, id); err != nil {
				return nil, fmt.Errorf("清除头像: %w", err)
			}
		} else {
			if _, err := tx.Exec(ctx, `update users set avatar_jpeg = $2, updated_at = now() where id = $1`, id, avatarJPEG); err != nil {
				return nil, fmt.Errorf("更新头像: %w", err)
			}
		}
	}

	if input.RoleIDs != nil {
		if err := replaceUserRoles(ctx, tx, id, input.RoleIDs); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update user: %w", err)
	}
	return r.GetUser(ctx, id)
}

func (r *Repository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := r.DB.Exec(ctx, `delete from users where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (r *Repository) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.DB.Query(ctx, `
		select id, code, name, description, created_at, updated_at
		from roles
		order by created_at desc
	`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		if err := r.loadRolePermissions(ctx, role); err != nil {
			return nil, err
		}
		roles = append(roles, *role)
	}
	return roles, rows.Err()
}

func (r *Repository) CreateRole(ctx context.Context, input CreateRoleInput) (*Role, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create role: %w", err)
	}
	defer tx.Rollback(ctx)

	role, err := scanRole(tx.QueryRow(ctx, `
		insert into roles (code, name, description)
		values ($1, $2, $3)
		returning id, code, name, description, created_at, updated_at
	`, input.Code, input.Name, input.Description))
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	if err := replaceRolePermissions(ctx, tx, role.ID, input.PermissionIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create role: %w", err)
	}
	return r.GetRole(ctx, role.ID)
}

func (r *Repository) GetRole(ctx context.Context, id uuid.UUID) (*Role, error) {
	role, err := scanRole(r.DB.QueryRow(ctx, `
		select id, code, name, description, created_at, updated_at
		from roles
		where id = $1
	`, id))
	if err != nil {
		return nil, err
	}
	if err := r.loadRolePermissions(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

func (r *Repository) UpdateRole(ctx context.Context, id uuid.UUID, input UpdateRoleInput) (*Role, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update role: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		update roles
		set name = $2, description = $3, updated_at = now()
		where id = $1
	`, id, input.Name, input.Description); err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}
	if input.PermissionIDs != nil {
		if err := replaceRolePermissions(ctx, tx, id, input.PermissionIDs); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update role: %w", err)
	}
	return r.GetRole(ctx, id)
}

func (r *Repository) DeleteRole(ctx context.Context, id uuid.UUID) error {
	_, err := r.DB.Exec(ctx, `delete from roles where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

func (r *Repository) SetRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) (*Role, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin set role permissions: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := replaceRolePermissions(ctx, tx, roleID, permissionIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit set role permissions: %w", err)
	}
	return r.GetRole(ctx, roleID)
}

func (r *Repository) ListPermissions(ctx context.Context) ([]Permission, error) {
	rows, err := r.DB.Query(ctx, `
		select id, code, name, description, created_at, updated_at
		from permissions
		order by code asc
	`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, *permission)
	}
	return permissions, rows.Err()
}

func (r *Repository) CreatePermission(ctx context.Context, input CreatePermissionInput) (*Permission, error) {
	permission, err := scanPermission(r.DB.QueryRow(ctx, `
		insert into permissions (code, name, description)
		values ($1, $2, $3)
		returning id, code, name, description, created_at, updated_at
	`, input.Code, input.Name, input.Description))
	if err != nil {
		return nil, fmt.Errorf("create permission: %w", err)
	}
	return permission, nil
}

func (r *Repository) loadUserAccess(ctx context.Context, user *User) error {
	roles, err := r.userRoles(ctx, user.ID)
	if err != nil {
		return err
	}
	permissions, err := r.userPermissions(ctx, user.ID)
	if err != nil {
		return err
	}
	user.Roles = roles
	user.Permissions = permissions
	return nil
}

func (r *Repository) userRoles(ctx context.Context, userID uuid.UUID) ([]Role, error) {
	rows, err := r.DB.Query(ctx, `
		select r.id, r.code, r.name, r.description, r.created_at, r.updated_at
		from roles r
		join user_roles ur on ur.role_id = r.id
		where ur.user_id = $1
		order by r.code asc
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user roles: %w", err)
	}
	defer rows.Close()

	var roles []Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *role)
	}
	return roles, rows.Err()
}

func (r *Repository) userPermissions(ctx context.Context, userID uuid.UUID) ([]Permission, error) {
	rows, err := r.DB.Query(ctx, `
		select distinct p.id, p.code, p.name, p.description, p.created_at, p.updated_at
		from permissions p
		join role_permissions rp on rp.permission_id = p.id
		join user_roles ur on ur.role_id = rp.role_id
		where ur.user_id = $1
		order by p.code asc
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user permissions: %w", err)
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, *permission)
	}
	return permissions, rows.Err()
}

func (r *Repository) loadRolePermissions(ctx context.Context, role *Role) error {
	rows, err := r.DB.Query(ctx, `
		select p.id, p.code, p.name, p.description, p.created_at, p.updated_at
		from permissions p
		join role_permissions rp on rp.permission_id = p.id
		where rp.role_id = $1
		order by p.code asc
	`, role.ID)
	if err != nil {
		return fmt.Errorf("list role permissions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return err
		}
		role.Permissions = append(role.Permissions, *permission)
	}
	return rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*User, error) {
	var user User
	var passwordHash *string
	var githubID *int64
	var avatar []byte
	if err := row.Scan(
		&user.ID, &user.Username, &passwordHash, &user.AuthProvider, &githubID,
		&user.DisplayName, &user.Email, &user.Active, &avatar, &user.CreatedAt, &user.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	if passwordHash != nil {
		user.PasswordHash = *passwordHash
	}
	user.GithubID = githubID
	user.avatarJPEG = avatar
	if user.AuthProvider == "" {
		user.AuthProvider = AuthProviderLocal
	}
	applyAvatarURL(&user)
	return &user, nil
}

func applyAvatarURL(u *User) {
	if len(u.avatarJPEG) == 0 {
		u.AvatarURL = ""
		return
	}
	u.AvatarURL = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(u.avatarJPEG)
}

func scanRole(row scanner) (*Role, error) {
	var role Role
	if err := row.Scan(&role.ID, &role.Code, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan role: %w", err)
	}
	return &role, nil
}

func scanPermission(row scanner) (*Permission, error) {
	var permission Permission
	if err := row.Scan(&permission.ID, &permission.Code, &permission.Name, &permission.Description, &permission.CreatedAt, &permission.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan permission: %w", err)
	}
	return &permission, nil
}

func replaceUserRoles(ctx context.Context, tx pgx.Tx, userID uuid.UUID, roleIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `delete from user_roles where user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear user roles: %w", err)
	}
	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx, `
			insert into user_roles (user_id, role_id)
			values ($1, $2)
			on conflict do nothing
		`, userID, roleID); err != nil {
			return fmt.Errorf("attach user role: %w", err)
		}
	}
	return nil
}

func replaceRolePermissions(ctx context.Context, tx pgx.Tx, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `delete from role_permissions where role_id = $1`, roleID); err != nil {
		return fmt.Errorf("clear role permissions: %w", err)
	}
	for _, permissionID := range permissionIDs {
		if _, err := tx.Exec(ctx, `
			insert into role_permissions (role_id, permission_id)
			values ($1, $2)
			on conflict do nothing
		`, roleID, permissionID); err != nil {
			return fmt.Errorf("attach role permission: %w", err)
		}
	}
	return nil
}
