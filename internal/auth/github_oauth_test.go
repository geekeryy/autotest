package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestOAuthStateRoundTrip(t *testing.T) {
	t.Parallel()

	svc := &Service{secret: []byte("test-secret")}
	state, err := svc.OAuthStateForTest(time.Now().Add(5*time.Minute), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.verifyOAuthState(state); err != nil {
		t.Fatalf("verifyOAuthState: %v", err)
	}
}

func TestOAuthStateExpired(t *testing.T) {
	t.Parallel()

	svc := &Service{secret: []byte("test-secret")}
	state, err := svc.OAuthStateForTest(time.Now().Add(-time.Minute), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.verifyOAuthState(state); err == nil {
		t.Fatal("expected expired state error")
	}
}

func TestGithubUsernameSanitize(t *testing.T) {
	t.Parallel()

	if got := githubUsername("Hello.World"); got != "gh_hello.world" {
		t.Fatalf("got %q", got)
	}
	if got := githubUsername(""); got != "gh_user" {
		t.Fatalf("got %q", got)
	}
}

func TestOAuthUsernamePrefixes(t *testing.T) {
	t.Parallel()

	if got := oauthUsernamePrefix("gitlab"); got != "gl_" {
		t.Fatalf("got %q", got)
	}
	if got := oauthUsernamePrefix("google"); got != "gg_" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveGithubUserCreatesInactiveUser(t *testing.T) {
	t.Parallel()

	repo := &oauthMemRepo{}
	svc := &Service{
		repo:   repo,
		secret: []byte("test-secret"),
		ttl:    time.Hour,
	}
	user, created, err := svc.resolveGithubUser(context.Background(), &githubUserProfile{
		ID:    99,
		Login: "newbie",
		Name:  "Newbie",
		Email: "newbie@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if user.Active {
		t.Fatal("expected inactive github user")
	}
	if user.AuthProvider != AuthProviderGithub {
		t.Fatalf("authProvider=%q", user.AuthProvider)
	}
	if user.GithubID == nil || *user.GithubID != 99 {
		t.Fatalf("githubID=%v", user.GithubID)
	}

	user2, created2, err := svc.resolveGithubUser(context.Background(), &githubUserProfile{
		ID:    99,
		Login: "newbie",
		Name:  "Newbie",
		Email: "newbie@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("expected idempotent lookup")
	}
	if user2.ID != user.ID {
		t.Fatal("expected same user on second login")
	}
}

func TestOAuthCallbackErrorQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err  error
		want string
	}{
		{ErrInvalidOAuthState, "invalid_state"},
		{ErrOAuthExchangeFailed, "exchange_failed"},
		{fmt.Errorf("wrap: %w", ErrOAuthProfileFetchFailed), "profile_failed"},
		{ErrRegistrationDisabled, "registration_disabled"},
		{errors.New("other"), "oauth_failed"},
	}
	for _, tc := range cases {
		if got := OAuthCallbackErrorQuery(tc.err); got != tc.want {
			t.Fatalf("err=%v got %q want %q", tc.err, got, tc.want)
		}
	}
}

func TestResolveGithubUserRetriesAfterDuplicateGithubID(t *testing.T) {
	t.Parallel()

	repo := &oauthRaceRepo{}
	svc := &Service{repo: repo, secret: []byte("test-secret"), ttl: time.Hour}
	profile := &githubUserProfile{ID: 42, Login: "race", Name: "Race", Email: "race@example.com"}

	user, created, err := svc.resolveGithubUser(context.Background(), profile)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected created=false after duplicate retry")
	}
	if user == nil || user.GithubID == nil || *user.GithubID != 42 {
		t.Fatalf("user=%+v", user)
	}
}

type oauthMemRepo struct {
	byGithub   map[int64]*User
	byExternal map[string]*User
}

func (r *oauthMemRepo) externalKey(providerType, externalID string) string {
	return providerType + ":" + externalID
}

func (r *oauthMemRepo) ensureMaps() {
	if r.byGithub == nil {
		r.byGithub = map[int64]*User{}
	}
	if r.byExternal == nil {
		r.byExternal = map[string]*User{}
	}
}

func (r *oauthMemRepo) EnsureDefaults(context.Context, string, string) error { return nil }
func (r *oauthMemRepo) UpdatePassword(context.Context, uuid.UUID, string) error { return nil }

func (r *oauthMemRepo) GetUserByGithubID(_ context.Context, githubID int64) (*User, error) {
	r.ensureMaps()
	user, ok := r.byGithub[githubID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return user, nil
}

func (r *oauthMemRepo) GetUserByExternalIdentity(_ context.Context, providerType, externalID string) (*User, error) {
	r.ensureMaps()
	user, ok := r.byExternal[r.externalKey(providerType, externalID)]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return user, nil
}

func (r *oauthMemRepo) EnsureExternalIdentity(_ context.Context, userID uuid.UUID, providerType, externalID string) error {
	r.ensureMaps()
	r.byExternal[r.externalKey(providerType, externalID)] = &User{ID: userID}
	return nil
}

func (r *oauthMemRepo) AllocateOAuthUsername(_ context.Context, prefix, base string) (string, error) {
	return prefix + base, nil
}

func (r *oauthMemRepo) AllocateGithubUsername(_ context.Context, base string) (string, error) {
	return base, nil
}

func (r *oauthMemRepo) CreateGithubUser(ctx context.Context, input GithubUserInput) (*User, error) {
	return r.CreateOAuthUser(ctx, OAuthUserInput{
		Username:     input.Username,
		DisplayName:  input.DisplayName,
		Email:        input.Email,
		AuthProvider: AuthProviderGithub,
		GithubID:     &input.GithubID,
		Active:       input.Active,
		AvatarJPEG:   input.AvatarJPEG,
		ProviderType: AuthProviderGithub,
		ExternalID:   fmt.Sprintf("%d", input.GithubID),
	})
}

func (r *oauthMemRepo) CreateOAuthUser(_ context.Context, input OAuthUserInput) (*User, error) {
	r.ensureMaps()
	user := &User{
		ID:           uuid.New(),
		Username:     input.Username,
		DisplayName:  input.DisplayName,
		Email:        input.Email,
		Active:       input.Active,
		AuthProvider: input.AuthProvider,
		GithubID:     input.GithubID,
	}
	if input.GithubID != nil {
		r.byGithub[*input.GithubID] = user
	}
	r.byExternal[r.externalKey(externalType(input), input.ExternalID)] = user
	return user, nil
}

func (r *oauthMemRepo) UpdateGithubUserProfile(ctx context.Context, id uuid.UUID, displayName, email string, avatar []byte) (*User, error) {
	return r.UpdateOAuthUserProfile(ctx, id, displayName, email, avatar)
}

func (r *oauthMemRepo) UpdateOAuthUserProfile(_ context.Context, id uuid.UUID, displayName, email string, _ []byte) (*User, error) {
	for _, user := range r.byExternal {
		if user.ID == id {
			user.DisplayName = displayName
			user.Email = email
			return user, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *oauthMemRepo) GetUserByUsername(context.Context, string) (*User, error) {
	return nil, pgx.ErrNoRows
}
func (r *oauthMemRepo) GetUser(context.Context, uuid.UUID) (*User, error) { return nil, pgx.ErrNoRows }
func (r *oauthMemRepo) ListUserIDsByPermission(context.Context, string) ([]uuid.UUID, error) {
	return nil, nil
}
func (r *oauthMemRepo) ListUsers(context.Context) ([]User, error) { return nil, nil }
func (r *oauthMemRepo) CreateUser(context.Context, CreateUserInput, []byte) (*User, error) {
	return nil, pgx.ErrNoRows
}
func (r *oauthMemRepo) UpdateUser(context.Context, uuid.UUID, UpdateUserInput, bool, bool, []byte) (*User, error) {
	return nil, pgx.ErrNoRows
}
func (r *oauthMemRepo) DeleteUser(context.Context, uuid.UUID) error { return nil }
func (r *oauthMemRepo) ListRoles(context.Context) ([]Role, error)  { return nil, nil }
func (r *oauthMemRepo) CreateRole(context.Context, CreateRoleInput) (*Role, error) {
	return nil, pgx.ErrNoRows
}
func (r *oauthMemRepo) UpdateRole(context.Context, uuid.UUID, UpdateRoleInput) (*Role, error) {
	return nil, pgx.ErrNoRows
}
func (r *oauthMemRepo) DeleteRole(context.Context, uuid.UUID) error { return nil }
func (r *oauthMemRepo) SetRolePermissions(context.Context, uuid.UUID, []uuid.UUID) (*Role, error) {
	return nil, pgx.ErrNoRows
}
func (r *oauthMemRepo) ListPermissions(context.Context) ([]Permission, error) { return nil, nil }
func (r *oauthMemRepo) CreatePermission(context.Context, CreatePermissionInput) (*Permission, error) {
	return nil, pgx.ErrNoRows
}

type oauthRaceRepo struct {
	oauthMemRepo
}

func (r *oauthRaceRepo) CreateOAuthUser(ctx context.Context, input OAuthUserInput) (*User, error) {
	r.ensureMaps()
	existing := &User{
		ID:           uuid.New(),
		Username:     input.Username,
		DisplayName:  input.DisplayName,
		Email:        input.Email,
		Active:       input.Active,
		AuthProvider: input.AuthProvider,
		GithubID:     input.GithubID,
	}
	if input.GithubID != nil {
		r.byGithub[*input.GithubID] = existing
	}
	r.byExternal[r.externalKey(externalType(input), input.ExternalID)] = existing
	return nil, fmt.Errorf(`duplicate key value violates unique constraint "users_github_id_key"`)
}
