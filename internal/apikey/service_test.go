package apikey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"autotest/internal/auth"

	"github.com/google/uuid"
)

// fakeRepo 是 repositoryBackend 的内存实现，便于隔离 DB 校验领域逻辑。
type fakeRepo struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]*record
	byHash   map[string]uuid.UUID
	touched  map[uuid.UUID]time.Time
	createIn []record
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byID:    map[uuid.UUID]*record{},
		byHash:  map[string]uuid.UUID{},
		touched: map[uuid.UUID]time.Time{},
	}
}

func (f *fakeRepo) Create(_ context.Context, rec record) (*record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	now := time.Now()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	if _, exists := f.byHash[rec.TokenHash]; exists {
		return nil, errors.New("token_hash 冲突")
	}
	stored := rec
	f.byID[rec.ID] = &stored
	f.byHash[rec.TokenHash] = rec.ID
	f.createIn = append(f.createIn, stored)
	out := stored
	return &out, nil
}

func (f *fakeRepo) GetByHash(_ context.Context, tokenHash string) (*record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byHash[tokenHash]
	if !ok {
		return nil, ErrTokenInvalid
	}
	rec, ok := f.byID[id]
	if !ok {
		return nil, ErrTokenInvalid
	}
	out := *rec
	return &out, nil
}

func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (*record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := *rec
	return &out, nil
}

func (f *fakeRepo) List(_ context.Context, owner uuid.UUID) ([]record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]record, 0)
	for _, rec := range f.byID {
		if rec.CreatedBy == owner {
			out = append(out, *rec)
		}
	}
	return out, nil
}

func (f *fakeRepo) update(_ context.Context, id uuid.UUID, owner uuid.UUID, patch updatePatch) (*record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.byID[id]
	if !ok || rec.CreatedBy != owner {
		return nil, ErrNotFound
	}
	if patch.Name != nil {
		rec.Name = *patch.Name
	}
	if patch.Enabled != nil {
		rec.Enabled = *patch.Enabled
	}
	switch {
	case patch.ClearExpires:
		rec.ExpiresAt = nil
	case patch.ExpiresAt != nil:
		v := *patch.ExpiresAt
		rec.ExpiresAt = &v
	}
	rec.UpdatedAt = time.Now()
	out := *rec
	return &out, nil
}

func (f *fakeRepo) Rotate(_ context.Context, id uuid.UUID, owner uuid.UUID, hash, prefix, suffix string) (*record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.byID[id]
	if !ok || rec.CreatedBy != owner {
		return nil, ErrNotFound
	}
	delete(f.byHash, rec.TokenHash)
	rec.TokenHash = hash
	rec.TokenPrefix = prefix
	rec.TokenSuffix = suffix
	rec.LastUsedAt = nil
	rec.UpdatedAt = time.Now()
	f.byHash[hash] = id
	out := *rec
	return &out, nil
}

func (f *fakeRepo) SoftDelete(_ context.Context, id uuid.UUID, owner uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.byID[id]
	if !ok || rec.CreatedBy != owner {
		return ErrNotFound
	}
	delete(f.byHash, rec.TokenHash)
	delete(f.byID, id)
	return nil
}

func (f *fakeRepo) TouchLastUsed(_ context.Context, id uuid.UUID, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.touched[id] = at
	if rec, ok := f.byID[id]; ok {
		v := at
		rec.LastUsedAt = &v
	}
	return nil
}

// fakeUserLoader 注入固定的 User 视图，模拟 auth.Repository.GetUser。
type fakeUserLoader struct {
	users map[uuid.UUID]*auth.User
	err   error
}

func (f *fakeUserLoader) GetUser(_ context.Context, id uuid.UUID) (*auth.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("用户不存在")
}

func newServiceForTests(repo repositoryBackend, users userLoader, now func() time.Time) *Service {
	svc := &Service{repo: repo, users: users, now: now}
	svc.touch = func(uuid.UUID, time.Time) {}
	return svc
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestGenerateTokenShape(t *testing.T) {
	t.Parallel()

	for i := 0; i < 16; i++ {
		token, prefix, suffix, hashHex, err := generateToken()
		if err != nil {
			t.Fatalf("generateToken: %v", err)
		}
		if !strings.HasPrefix(token, "at-") {
			t.Fatalf("token 缺少 at- 前缀: %q", token)
		}
		if strings.ToLower(token) != token {
			t.Fatalf("token 应为全小写: %q", token)
		}
		if strings.Contains(token, "=") {
			t.Fatalf("base32 应去除 padding: %q", token)
		}
		body := strings.TrimPrefix(token, "at-")
		// 32 字节随机 → base32 (无填充) 长度 = ceil(32 * 8 / 5) = 52
		if len(body) != 52 {
			t.Fatalf("base32 body 长度 = %d，期望 52", len(body))
		}
		if prefix != token[:8] {
			t.Fatalf("prefix 应为 token 前 8 字符: got %q want %q", prefix, token[:8])
		}
		if suffix != token[len(token)-4:] {
			t.Fatalf("suffix 应为 token 后 4 字符: got %q want %q", suffix, token[len(token)-4:])
		}
		if hashHex != sha256Hex(token) {
			t.Fatalf("hash 不等于 SHA-256(token)")
		}
	}
}

func TestNormalizeScopes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   []string
		want    []string
		wantErr error
	}{
		{name: "empty defaults to specs:import", input: nil, want: []string{ScopeSpecsImport}},
		{name: "single allowed", input: []string{"specs:import"}, want: []string{ScopeSpecsImport}},
		{name: "duplicates dedup", input: []string{"specs:import", "specs:import"}, want: []string{ScopeSpecsImport}},
		{name: "whitespace trimmed", input: []string{"  specs:import  "}, want: []string{ScopeSpecsImport}},
		{name: "forbidden scope rejected", input: []string{"cases:run"}, wantErr: ErrScopeForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeScopes(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d want %d (%v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("scope[%d]=%q want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestServiceCreatePersistsHashOnly(t *testing.T) {
	t.Parallel()

	creator := uuid.New()
	repo := newFakeRepo()
	users := &fakeUserLoader{users: map[uuid.UUID]*auth.User{
		creator: {ID: creator, Username: "alice", Active: true},
	}}
	svc := newServiceForTests(repo, users, time.Now)

	result, err := svc.Create(context.Background(), creator, CreateInput{Name: "ci-token"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(result.Token, "at-") {
		t.Fatalf("token 未带 at- 前缀: %q", result.Token)
	}
	if len(repo.createIn) != 1 {
		t.Fatalf("期望 1 条入库记录，实际 %d", len(repo.createIn))
	}
	stored := repo.createIn[0]
	if stored.TokenHash != sha256Hex(result.Token) {
		t.Fatalf("库内 hash 不等于 SHA-256(明文 token)")
	}
	if stored.TokenPrefix != result.Token[:8] || stored.TokenSuffix != result.Token[len(result.Token)-4:] {
		t.Fatalf("prefix/suffix 与 token 不一致: %+v", stored)
	}
	if result.APIKey.Mask != stored.TokenPrefix+"…"+stored.TokenSuffix {
		t.Fatalf("mask 渲染异常: %q", result.APIKey.Mask)
	}
	if len(stored.Scopes) != 1 || stored.Scopes[0] != ScopeSpecsImport {
		t.Fatalf("默认 scope 应为 [specs:import]: %#v", stored.Scopes)
	}
	if result.APIKey.Enabled != true {
		t.Fatalf("新建 API Key 应默认启用")
	}
}

func TestServiceCreateRejectsForbiddenScope(t *testing.T) {
	t.Parallel()

	repo := newFakeRepo()
	svc := newServiceForTests(repo, &fakeUserLoader{}, time.Now)
	_, err := svc.Create(context.Background(), uuid.New(), CreateInput{Name: "x", Scopes: []string{"cases:run"}})
	if !errors.Is(err, ErrScopeForbidden) {
		t.Fatalf("expected ErrScopeForbidden, got %v", err)
	}
}

func TestServiceAuthenticate(t *testing.T) {
	creator := uuid.New()
	user := &auth.User{
		ID:       creator,
		Username: "alice",
		Active:   true,
		Permissions: []auth.Permission{
			{Code: auth.PermissionSpecsImport},
		},
	}

	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-time.Hour)

	type wantPrincipal struct {
		source string
		scope  string
		perm   string
	}

	cases := []struct {
		name    string
		mutate  func(rec *record)
		token   func(known string) string
		users   *fakeUserLoader
		wantErr error
		want    *wantPrincipal
	}{
		{
			name:  "happy path",
			token: func(s string) string { return s },
			users: &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: user}},
			want:  &wantPrincipal{source: auth.SourceAPIKey, scope: ScopeSpecsImport, perm: auth.PermissionSpecsImport},
		},
		{
			name:    "disabled returns ErrTokenDisabled",
			mutate:  func(rec *record) { rec.Enabled = false },
			token:   func(s string) string { return s },
			users:   &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: user}},
			wantErr: ErrTokenDisabled,
		},
		{
			name:    "expired returns ErrTokenExpired",
			mutate:  func(rec *record) { rec.ExpiresAt = &past },
			token:   func(s string) string { return s },
			users:   &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: user}},
			wantErr: ErrTokenExpired,
		},
		{
			name:    "soft-deleted returns ErrTokenInvalid",
			token:   func(s string) string { return s },
			users:   &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: user}},
			wantErr: ErrTokenInvalid,
		},
		{
			name:    "unknown token returns ErrTokenInvalid",
			token:   func(string) string { return "at-deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdead" },
			users:   &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: user}},
			wantErr: ErrTokenInvalid,
		},
		{
			name:    "missing prefix returns ErrTokenInvalid",
			token:   func(string) string { return "jwt-style-token" },
			users:   &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: user}},
			wantErr: ErrTokenInvalid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := newServiceForTests(repo, tc.users, func() time.Time { return now })

			created, err := svc.Create(context.Background(), creator, CreateInput{Name: "n", ExpiresAt: &future})
			if err != nil {
				t.Fatalf("seed Create: %v", err)
			}
			rec := repo.byID[created.APIKey.ID]
			if tc.mutate != nil {
				tc.mutate(rec)
			}
			if tc.name == "soft-deleted returns ErrTokenInvalid" {
				if err := repo.SoftDelete(context.Background(), created.APIKey.ID, creator); err != nil {
					t.Fatalf("seed SoftDelete: %v", err)
				}
			}

			principal, err := svc.Authenticate(context.Background(), tc.token(created.Token))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if principal.Source != tc.want.source {
				t.Fatalf("source=%q want %q", principal.Source, tc.want.source)
			}
			if _, ok := principal.Scopes[tc.want.scope]; !ok {
				t.Fatalf("Scopes 缺少 %q: %#v", tc.want.scope, principal.Scopes)
			}
			if _, ok := principal.Permissions[tc.want.perm]; !ok {
				t.Fatalf("Permissions 缺少 %q: %#v", tc.want.perm, principal.Permissions)
			}
		})
	}
}

func TestServiceAuthenticateRejectsInactiveUser(t *testing.T) {
	t.Parallel()

	creator := uuid.New()
	now := time.Now()
	user := &auth.User{ID: creator, Username: "alice", Active: false}
	users := &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: user}}

	repo := newFakeRepo()
	svc := newServiceForTests(repo, users, func() time.Time { return now })
	created, err := svc.Create(context.Background(), creator, CreateInput{Name: "n"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), created.Token); err == nil {
		t.Fatalf("expected error for inactive user")
	}
}

func TestServiceUpdateAndDelete(t *testing.T) {
	t.Parallel()

	creator := uuid.New()
	repo := newFakeRepo()
	users := &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: {ID: creator, Username: "alice", Active: true}}}
	svc := newServiceForTests(repo, users, time.Now)
	created, err := svc.Create(context.Background(), creator, CreateInput{Name: "ci"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	disabled := false
	newName := "renamed"
	updated, err := svc.Update(context.Background(), creator, created.APIKey.ID, UpdateInput{Name: &newName, Enabled: &disabled})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "renamed" || updated.Enabled {
		t.Fatalf("更新结果异常: %+v", updated)
	}

	if err := svc.Delete(context.Background(), creator, created.APIKey.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), created.Token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("软删除后 Authenticate 应返回 ErrTokenInvalid，实际 %v", err)
	}
}

func TestServiceRotate(t *testing.T) {
	t.Parallel()

	creator := uuid.New()
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	user := &auth.User{
		ID:       creator,
		Username: "alice",
		Active:   true,
		Permissions: []auth.Permission{
			{Code: auth.PermissionSpecsImport},
		},
	}
	users := &fakeUserLoader{users: map[uuid.UUID]*auth.User{creator: user}}

	repo := newFakeRepo()
	svc := newServiceForTests(repo, users, func() time.Time { return now })

	expires := now.Add(7 * 24 * time.Hour)
	created, err := svc.Create(context.Background(), creator, CreateInput{
		Name:      "ci-token",
		ExpiresAt: &expires,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Authenticate(context.Background(), created.Token); err != nil {
		t.Fatalf("Authenticate before rotate: %v", err)
	}
	// newServiceForTests 把 touch 替换成了 no-op，这里手工调一次仓储 TouchLastUsed
	// 模拟生产环境下 Authenticate 异步刷新出 last_used_at 的状态，便于断言 rotate 是否清空。
	if err := repo.TouchLastUsed(context.Background(), created.APIKey.ID, now); err != nil {
		t.Fatalf("seed TouchLastUsed: %v", err)
	}
	if rec := repo.byID[created.APIKey.ID]; rec == nil || rec.LastUsedAt == nil {
		t.Fatalf("seed: 期望 last_used_at 已被刷新，实际为空")
	}

	rotated, newToken, err := svc.Rotate(context.Background(), creator, created.APIKey.ID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if newToken == created.Token {
		t.Fatalf("Rotate 应生成新 token，实际仍为原 token")
	}
	if !strings.HasPrefix(newToken, "at-") {
		t.Fatalf("新 token 缺少 at- 前缀: %q", newToken)
	}

	if rotated.ID != created.APIKey.ID {
		t.Fatalf("rotate 不应改变 ID: got %s want %s", rotated.ID, created.APIKey.ID)
	}
	if rotated.Name != created.APIKey.Name {
		t.Fatalf("rotate 不应改变 name: got %q want %q", rotated.Name, created.APIKey.Name)
	}
	if !rotated.Enabled {
		t.Fatalf("rotate 不应改变 enabled")
	}
	if rotated.ExpiresAt == nil || !rotated.ExpiresAt.Equal(expires) {
		t.Fatalf("rotate 不应改变 expires_at: got %v want %v", rotated.ExpiresAt, &expires)
	}
	if len(rotated.Scopes) != 1 || rotated.Scopes[0] != ScopeSpecsImport {
		t.Fatalf("rotate 不应改变 scopes: %#v", rotated.Scopes)
	}
	if rotated.LastUsedAt != nil {
		t.Fatalf("rotate 应清空 last_used_at: %v", rotated.LastUsedAt)
	}
	if rotated.Mask != newToken[:8]+"…"+newToken[len(newToken)-4:] {
		t.Fatalf("mask 应根据新 token 重新计算: got %q", rotated.Mask)
	}

	if _, err := svc.Authenticate(context.Background(), created.Token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("rotate 后旧 token 应返回 ErrTokenInvalid，实际 %v", err)
	}

	principal, err := svc.Authenticate(context.Background(), newToken)
	if err != nil {
		t.Fatalf("rotate 后新 token 校验应通过: %v", err)
	}
	if principal.UserID != creator {
		t.Fatalf("principal.UserID 异常: got %s want %s", principal.UserID, creator)
	}

	if _, _, err := svc.Rotate(context.Background(), creator, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rotate 不存在的 id 应返回 ErrNotFound，实际 %v", err)
	}
}

func TestServiceListAndMutateScopedToOwner(t *testing.T) {
	t.Parallel()

	alice := uuid.New()
	bob := uuid.New()
	repo := newFakeRepo()
	users := &fakeUserLoader{users: map[uuid.UUID]*auth.User{
		alice: {ID: alice, Username: "alice", Active: true},
		bob:   {ID: bob, Username: "bob", Active: true},
	}}
	svc := newServiceForTests(repo, users, time.Now)

	aKey, err := svc.Create(context.Background(), alice, CreateInput{Name: "a-ci"})
	if err != nil {
		t.Fatalf("Create alice: %v", err)
	}
	_, err = svc.Create(context.Background(), bob, CreateInput{Name: "b-ci"})
	if err != nil {
		t.Fatalf("Create bob: %v", err)
	}

	aliceList, err := svc.List(context.Background(), alice)
	if err != nil {
		t.Fatalf("List alice: %v", err)
	}
	if len(aliceList) != 1 || aliceList[0].ID != aKey.APIKey.ID {
		t.Fatalf("alice 应只看到 1 条自己的 Key: %#v", aliceList)
	}

	wrongName := "hijack"
	if _, err := svc.Update(context.Background(), bob, aKey.APIKey.ID, UpdateInput{Name: &wrongName}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob 更新 alice 的 Key 应 ErrNotFound，实际 %v", err)
	}
	if _, _, err := svc.Rotate(context.Background(), bob, aKey.APIKey.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob 重置 alice 的 Key 应 ErrNotFound，实际 %v", err)
	}
	if err := svc.Delete(context.Background(), bob, aKey.APIKey.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob 删除 alice 的 Key 应 ErrNotFound，实际 %v", err)
	}
}
