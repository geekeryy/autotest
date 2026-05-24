package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAdminUsername = "admin"
	defaultAdminPassword = "admin"
)

var ErrUnauthorized = errors.New("invalid username or password")
var ErrMustChangePassword = errors.New("must change password")
var ErrPasswordUnchanged = errors.New("new password must differ from current password")

// APIKeyAuthenticator 由 internal/apikey 实现，用于在 Authenticate 中间件中
// 桥接 API Key 校验逻辑，避免 internal/auth 反向依赖 apikey 包。
type APIKeyAuthenticator interface {
	Authenticate(ctx context.Context, token string) (*Principal, error)
}

type Settings struct {
	JWTSecret     string
	IsDevelopment bool
}

type Service struct {
	repo                repositoryBackend
	settings            Settings
	secret              []byte
	ttl                 time.Duration
	isDevelopment       bool
	apiKey              APIKeyAuthenticator
	httpClient          *http.Client
	pendingUserNotifier PendingUserNotifier
	authProviders       authProviderResolver
	oauthRegistry       oauthRegistry
}

func NewService(repo *Repository, settings Settings) *Service {
	return &Service{
		repo:          repo,
		settings:      settings,
		secret:        []byte(settings.JWTSecret),
		ttl:           24 * time.Hour,
		isDevelopment: settings.IsDevelopment,
	}
}

// WithAPIKey 注入 API Key 校验实现，由调用方在初始化阶段绑定。
// 未注入时 Authenticate 仅识别 JWT，遇到 at- 前缀 token 直接返回 401。
func (s *Service) WithAPIKey(authn APIKeyAuthenticator) {
	s.apiKey = authn
}

func (s *Service) EnsureDefaultAdmin(ctx context.Context) error {
	return s.repo.EnsureDefaults(ctx, defaultAdminUsername, defaultAdminPassword)
}

func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginResponse, error) {
	user, err := s.repo.GetUserByUsername(ctx, input.Username)
	if err != nil {
		return nil, ErrUnauthorized
	}
	if !user.Active {
		return nil, ErrUnauthorized
	}
	if user.PasswordHash == "" {
		return nil, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrUnauthorized
	}

	user, err = s.repo.GetUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	token, err := signJWT(s.secret, user, s.ttl)
	if err != nil {
		return nil, err
	}
	return &LoginResponse{Token: token, User: user}, nil
}

func (s *Service) CurrentUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.repo.GetUser(ctx, userID)
}

func (s *Service) ValidateToken(ctx context.Context, token string) (*Principal, error) {
	claims, err := parseJWT(s.secret, token)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	permissions := make(map[string]struct{}, len(user.Permissions))
	for _, permission := range user.Permissions {
		permissions[permission.Code] = struct{}{}
	}
	return &Principal{
		UserID:             user.ID,
		Username:           user.Username,
		Active:             user.Active,
		MustChangePassword: user.MustChangePassword,
		Permissions:        permissions,
		Source:             SourceJWT,
	}, nil
}

func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, input ChangePasswordInput) error {
	current := strings.TrimSpace(input.CurrentPassword)
	next := strings.TrimSpace(input.NewPassword)
	if current == "" || next == "" {
		return errors.New("current password and new password are required")
	}
	if len(next) < 6 {
		return errors.New("new password must be at least 6 characters")
	}
	if current == next {
		return ErrPasswordUnchanged
	}

	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return ErrUnauthorized
	}
	if user.PasswordHash == "" {
		return errors.New("password change is not supported for this account")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)); err != nil {
		return ErrUnauthorized
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(ctx, userID, string(hash))
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.repo.ListUsers(ctx)
}

func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	if input.Username == "" {
		return nil, errors.New("username is required")
	}
	if input.Password == "" {
		return nil, errors.New("password is required")
	}
	var avatar []byte
	if strings.TrimSpace(input.AvatarBase64) != "" {
		var err error
		avatar, err = ProcessAvatarBase64(input.AvatarBase64)
		if err != nil {
			return nil, err
		}
	}
	return s.repo.CreateUser(ctx, input, avatar)
}

func (s *Service) UpdateUser(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*User, error) {
	touchAvatar := input.ClearAvatar || strings.TrimSpace(input.AvatarBase64) != ""
	clearAvatar := input.ClearAvatar
	var jpeg []byte
	if touchAvatar && !clearAvatar {
		var err error
		jpeg, err = ProcessAvatarBase64(input.AvatarBase64)
		if err != nil {
			return nil, err
		}
	}
	return s.repo.UpdateUser(ctx, id, input, touchAvatar, clearAvatar, jpeg)
}

func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteUser(ctx, id)
}

func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	return s.repo.ListRoles(ctx)
}

func (s *Service) CreateRole(ctx context.Context, input CreateRoleInput) (*Role, error) {
	if input.Code == "" {
		return nil, errors.New("role code is required")
	}
	if input.Name == "" {
		return nil, errors.New("role name is required")
	}
	return s.repo.CreateRole(ctx, input)
}

func (s *Service) UpdateRole(ctx context.Context, id uuid.UUID, input UpdateRoleInput) (*Role, error) {
	return s.repo.UpdateRole(ctx, id, input)
}

func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteRole(ctx, id)
}

func (s *Service) SetRolePermissions(ctx context.Context, id uuid.UUID, input SetRolePermissionsInput) (*Role, error) {
	return s.repo.SetRolePermissions(ctx, id, input.PermissionIDs)
}

func (s *Service) ListPermissions(ctx context.Context) ([]Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *Service) CreatePermission(ctx context.Context, input CreatePermissionInput) (*Permission, error) {
	if input.Code == "" {
		return nil, errors.New("permission code is required")
	}
	if input.Name == "" {
		return nil, errors.New("permission name is required")
	}
	return s.repo.CreatePermission(ctx, input)
}
