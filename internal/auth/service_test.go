package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type fakeAuthRepo struct {
	byUsername map[string]*User
	byID       map[uuid.UUID]*User
}

func (f *fakeAuthRepo) GetUserByUsername(_ context.Context, username string) (*User, error) {
	user, ok := f.byUsername[username]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return user, nil
}

func (f *fakeAuthRepo) GetUser(_ context.Context, id uuid.UUID) (*User, error) {
	user, ok := f.byID[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return user, nil
}

func (f *fakeAuthRepo) EnsureDefaults(context.Context, string, string) error { return nil }
func (f *fakeAuthRepo) UpdatePassword(context.Context, uuid.UUID, string) error { return nil }
func (f *fakeAuthRepo) GetUserByGithubID(context.Context, int64) (*User, error) {
	return nil, pgx.ErrNoRows
}
func (f *fakeAuthRepo) GetUserByExternalIdentity(context.Context, string, string) (*User, error) {
	return nil, pgx.ErrNoRows
}
func (f *fakeAuthRepo) EnsureExternalIdentity(context.Context, uuid.UUID, string, string) error {
	return nil
}
func (f *fakeAuthRepo) AllocateOAuthUsername(context.Context, string, string) (string, error) {
	return "", errors.New("not implemented")
}
func (f *fakeAuthRepo) AllocateGithubUsername(context.Context, string) (string, error) {
	return "", errors.New("not implemented")
}
func (f *fakeAuthRepo) CreateGithubUser(context.Context, GithubUserInput) (*User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) CreateOAuthUser(context.Context, OAuthUserInput) (*User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) UpdateGithubUserProfile(context.Context, uuid.UUID, string, string, []byte) (*User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) UpdateOAuthUserProfile(context.Context, uuid.UUID, string, string, []byte) (*User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) ListUserIDsByPermission(context.Context, string) ([]uuid.UUID, error) {
	return nil, nil
}
func (f *fakeAuthRepo) ListUsers(context.Context) ([]User, error) { return nil, nil }
func (f *fakeAuthRepo) CreateUser(context.Context, CreateUserInput, []byte) (*User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) UpdateUser(context.Context, uuid.UUID, UpdateUserInput, bool, bool, []byte) (*User, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) DeleteUser(context.Context, uuid.UUID) error { return nil }
func (f *fakeAuthRepo) ListRoles(context.Context) ([]Role, error)  { return nil, nil }
func (f *fakeAuthRepo) CreateRole(context.Context, CreateRoleInput) (*Role, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) UpdateRole(context.Context, uuid.UUID, UpdateRoleInput) (*Role, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) DeleteRole(context.Context, uuid.UUID) error { return nil }
func (f *fakeAuthRepo) SetRolePermissions(context.Context, uuid.UUID, []uuid.UUID) (*Role, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAuthRepo) ListPermissions(context.Context) ([]Permission, error) { return nil, nil }
func (f *fakeAuthRepo) CreatePermission(context.Context, CreatePermissionInput) (*Permission, error) {
	return nil, errors.New("not implemented")
}

func TestLoginRejectsInactiveLocalUser(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	repo := &fakeAuthRepo{
		byUsername: map[string]*User{
			"alice": {
				ID:           userID,
				Username:     "alice",
				PasswordHash: string(hash),
				AuthProvider: AuthProviderLocal,
				Active:       false,
			},
		},
		byID: map[uuid.UUID]*User{
			userID: {
				ID:           userID,
				Username:     "alice",
				PasswordHash: string(hash),
				AuthProvider: AuthProviderLocal,
				Active:       false,
			},
		},
	}
	svc := &Service{
		repo:   repo,
		secret: []byte("test-secret"),
		ttl:    time.Hour,
	}

	_, err = svc.Login(context.Background(), LoginInput{Username: "alice", Password: "secret"})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestValidateTokenAllowsInactiveUser(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := &fakeAuthRepo{
		byID: map[uuid.UUID]*User{
			userID: {
				ID:           userID,
				Username:     "gh_alice",
				AuthProvider: AuthProviderGithub,
				Active:       false,
			},
		},
	}
	svc := &Service{
		repo:   repo,
		secret: []byte("test-secret"),
		ttl:    time.Hour,
	}
	token, err := signJWT(svc.secret, repo.byID[userID], svc.ttl)
	if err != nil {
		t.Fatal(err)
	}

	principal, err := svc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if principal.Active {
		t.Fatal("expected inactive principal")
	}
	if principal.UserID != userID {
		t.Fatalf("userID=%s want %s", principal.UserID, userID)
	}
}

func TestGitHubLoginDisabled(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&Service{})
	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
	rec := httptest.NewRecorder()
	handler.githubLogin(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestGitHubStatusDisabled(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&Service{})
	req := httptest.NewRequest(http.MethodGet, "/auth/github/status", nil)
	rec := httptest.NewRecorder()
	handler.githubStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"configured":false`) {
		t.Fatalf("body=%q want configured=false", rec.Body.String())
	}
}
