package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"autotest/internal/logx"
	"time"

	"autotest/internal/auth"

	"github.com/google/uuid"
)

// repositoryBackend 是 Service 依赖的仓储行为子集，通过接口隔离便于单元测试注入 fake。
type repositoryBackend interface {
	Create(ctx context.Context, rec record) (*record, error)
	GetByHash(ctx context.Context, tokenHash string) (*record, error)
	Get(ctx context.Context, id uuid.UUID) (*record, error)
	List(ctx context.Context, createdBy uuid.UUID) ([]record, error)
	update(ctx context.Context, id uuid.UUID, owner uuid.UUID, patch updatePatch) (*record, error)
	Rotate(ctx context.Context, id uuid.UUID, owner uuid.UUID, hash, prefix, suffix string) (*record, error)
	SoftDelete(ctx context.Context, id uuid.UUID, owner uuid.UUID) error
	TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error
}

// userLoader 由 *auth.Repository 实现，用于在校验通过后加载用户的权限快照。
type userLoader interface {
	GetUser(ctx context.Context, id uuid.UUID) (*auth.User, error)
}

// Service 提供 API Key 的领域操作，包括令牌生成、校验与 CRUD。
type Service struct {
	repo  repositoryBackend
	users userLoader
	now   func() time.Time
	touch func(id uuid.UUID, at time.Time)
}

// NewService 使用具体仓储构造 Service；touch 默认通过 goroutine 异步刷新 last_used_at。
func NewService(repo *Repository, users *auth.Repository) *Service {
	s := &Service{
		repo:  repo,
		users: users,
		now:   time.Now,
	}
	s.touch = s.defaultTouch
	return s
}

func (s *Service) defaultTouch(id uuid.UUID, at time.Time) {
	go func() {
		if err := s.repo.TouchLastUsed(context.Background(), id, at); err != nil {
			logx.Warn("apikey: 刷新最近使用时间失败", "err", err)
		}
	}()
}

// Create 生成一组新的 API Key：明文 token 在返回值中一次性下发，仅以 SHA-256 哈希入库。
// createdBy 必须传入当前登录用户 ID，用于审计与「上次使用时间」绑定。
func (s *Service) Create(ctx context.Context, createdBy uuid.UUID, input CreateInput) (*CreateResult, error) {
	if createdBy == uuid.Nil {
		return nil, errors.New("createdBy 不能为空")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("API Key 名称不能为空")
	}
	scopes, err := normalizeScopes(input.Scopes)
	if err != nil {
		return nil, err
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(s.now()) {
		return nil, errors.New("过期时间必须晚于当前时间")
	}

	token, prefix, suffix, hashHex, err := generateToken()
	if err != nil {
		return nil, err
	}

	rec := record{
		Name:        name,
		TokenHash:   hashHex,
		TokenPrefix: prefix,
		TokenSuffix: suffix,
		Scopes:      scopes,
		CreatedBy:   createdBy,
		Enabled:     true,
		ExpiresAt:   input.ExpiresAt,
	}
	saved, err := s.repo.Create(ctx, rec)
	if err != nil {
		return nil, err
	}
	return &CreateResult{APIKey: saved.toAPI(), Token: token}, nil
}

// List 返回当前用户创建的、未软删的 API Key（不含明文，仅含 mask 视图）。
func (s *Service) List(ctx context.Context, owner uuid.UUID) ([]APIKey, error) {
	rows, err := s.repo.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]APIKey, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toAPI())
	}
	return out, nil
}

// Update 修改 API Key 的名称、启用状态或过期时间；nil 字段保留原值，
// ClearExpiresAt=true 时显式将 expires_at 置空（与 ExpiresAt nil 的"不变更"语义区分）。
// 仅允许更新 owner 本人创建的 Key，否则返回 ErrNotFound。
func (s *Service) Update(ctx context.Context, owner uuid.UUID, id uuid.UUID, input UpdateInput) (*APIKey, error) {
	patch := updatePatch{
		Enabled:      input.Enabled,
		ExpiresAt:    input.ExpiresAt,
		ClearExpires: input.ClearExpiresAt,
	}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			return nil, errors.New("API Key 名称不能为空")
		}
		patch.Name = &trimmed
	}
	if input.ExpiresAt != nil && !input.ClearExpiresAt && !input.ExpiresAt.After(s.now()) {
		return nil, errors.New("过期时间必须晚于当前时间")
	}

	saved, err := s.repo.update(ctx, id, owner, patch)
	if err != nil {
		return nil, err
	}
	api := saved.toAPI()
	return &api, nil
}

// Delete 软删除一条 API Key，删除后即使保留 token 也无法再通过校验。
// 仅允许删除 owner 本人创建的 Key，否则返回 ErrNotFound。
func (s *Service) Delete(ctx context.Context, owner uuid.UUID, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, id, owner)
}

// Rotate 为指定 API Key 重新生成一组 token，并使用同一次 UPDATE 替换 hash / 前后缀掩码、
// 清空 last_used_at；name / scopes / enabled / expires_at 保持不变。
// 明文 token 仅作为返回值一次性下发，库内仅留 SHA-256 哈希，原 token 即刻失效。
// 找不到记录、已软删或非本人创建时返回 ErrNotFound（由 handler 转 404）。
func (s *Service) Rotate(ctx context.Context, owner uuid.UUID, id uuid.UUID) (*APIKey, string, error) {
	if id == uuid.Nil {
		return nil, "", ErrNotFound
	}
	token, prefix, suffix, hashHex, err := generateToken()
	if err != nil {
		return nil, "", err
	}
	saved, err := s.repo.Rotate(ctx, id, owner, hashHex, prefix, suffix)
	if err != nil {
		return nil, "", err
	}
	api := saved.toAPI()
	return &api, token, nil
}

// Authenticate 实现 auth.APIKeyAuthenticator：校验明文 token，构造 Principal。
// 校验失败按四种情况返回差异化错误，便于上层日志区分；不主动暴露记录存在性。
func (s *Service) Authenticate(ctx context.Context, token string) (*auth.Principal, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, auth.APIKeyPrefix) {
		return nil, ErrTokenInvalid
	}
	hashHex := hashToken(token)
	rec, err := s.repo.GetByHash(ctx, hashHex)
	if err != nil {
		return nil, err
	}
	if !rec.Enabled {
		return nil, ErrTokenDisabled
	}
	if rec.ExpiresAt != nil && !rec.ExpiresAt.After(s.now()) {
		return nil, ErrTokenExpired
	}

	user, err := s.users.GetUser(ctx, rec.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("加载 API Key 所属用户失败: %w", err)
	}
	if !user.Active {
		return nil, errors.New("API Key 所属用户已停用")
	}

	permissions := make(map[string]struct{}, len(user.Permissions))
	for _, permission := range user.Permissions {
		permissions[permission.Code] = struct{}{}
	}
	scopes := make(map[string]struct{}, len(rec.Scopes))
	for _, scope := range rec.Scopes {
		scopes[scope] = struct{}{}
	}

	if s.touch != nil {
		s.touch(rec.ID, s.now())
	}

	return &auth.Principal{
		UserID:      user.ID,
		Username:    user.Username,
		Permissions: permissions,
		Source:      auth.SourceAPIKey,
		Scopes:      scopes,
	}, nil
}

// normalizeScopes 校验入参 scopes：空切片回退默认值，未识别 scope 返回 ErrScopeForbidden。
func normalizeScopes(input []string) ([]string, error) {
	if len(input) == 0 {
		return append([]string{}, DefaultScopes...), nil
	}
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, raw := range input {
		scope := strings.TrimSpace(raw)
		if scope == "" {
			continue
		}
		if _, ok := allowedScopes[scope]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrScopeForbidden, scope)
		}
		if _, dup := seen[scope]; dup {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	if len(out) == 0 {
		return append([]string{}, DefaultScopes...), nil
	}
	return out, nil
}

// generateToken 返回 (明文 token, prefix, suffix, sha256 hex)。
// 32 字节随机串经 base32（无填充、小写）后拼接 `at-` 前缀，prefix/suffix 用于列表展示掩码。
func generateToken() (string, string, string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", "", fmt.Errorf("生成 API Key 随机串失败: %w", err)
	}
	body := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf))
	token := auth.APIKeyPrefix + body
	prefix := token[:8]
	suffix := token[len(token)-4:]
	return token, prefix, suffix, hashToken(token), nil
}

// hashToken 对完整明文 token 取 SHA-256 hex；与库内 token_hash 严格对齐。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
