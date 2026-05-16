package aitools

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestResolveProjectID_NoCallerStamped(t *testing.T) {
	t.Parallel()
	_, err := ResolveProjectID(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error when caller is missing, got nil")
	}
	if !strings.Contains(err.Error(), "未携带会话身份") {
		t.Errorf("expected Chinese identity error, got %v", err)
	}
}

func TestResolveProjectID_EmptyArgFallsBackToSession(t *testing.T) {
	t.Parallel()
	sessionProject := uuid.New()
	ctx := WithCaller(context.Background(), CallerContext{
		UserID:    uuid.New(),
		ProjectID: sessionProject,
	})
	got, err := ResolveProjectID(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sessionProject {
		t.Errorf("expected session project %s, got %s", sessionProject, got)
	}
}

func TestResolveProjectID_RejectsCrossProject(t *testing.T) {
	t.Parallel()
	sessionProject := uuid.New()
	otherProject := uuid.New()
	ctx := WithCaller(context.Background(), CallerContext{ProjectID: sessionProject})
	_, err := ResolveProjectID(ctx, otherProject.String())
	if err == nil {
		t.Fatalf("expected cross-project error, got nil")
	}
	if !strings.Contains(err.Error(), "权限隔离") {
		t.Errorf("expected 权限隔离 error, got %v", err)
	}
}

func TestResolveProjectID_AcceptsMatchingProject(t *testing.T) {
	t.Parallel()
	sessionProject := uuid.New()
	ctx := WithCaller(context.Background(), CallerContext{ProjectID: sessionProject})
	got, err := ResolveProjectID(ctx, sessionProject.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sessionProject {
		t.Errorf("expected %s, got %s", sessionProject, got)
	}
}

func TestResolveProjectID_RejectsMalformedUUID(t *testing.T) {
	t.Parallel()
	ctx := WithCaller(context.Background(), CallerContext{ProjectID: uuid.New()})
	_, err := ResolveProjectID(ctx, "not-a-uuid")
	if err == nil {
		t.Fatalf("expected uuid parse error, got nil")
	}
}

func TestRequireProjectAccess_NoCaller(t *testing.T) {
	t.Parallel()
	err := RequireProjectAccess(context.Background(), uuid.New())
	if err == nil {
		t.Fatalf("expected error when caller missing")
	}
}

func TestRequireProjectAccess_RejectsMismatch(t *testing.T) {
	t.Parallel()
	sessionProject := uuid.New()
	ctx := WithCaller(context.Background(), CallerContext{ProjectID: sessionProject})
	err := RequireProjectAccess(ctx, uuid.New())
	if err == nil || !strings.Contains(err.Error(), "权限隔离") {
		t.Fatalf("expected isolation error, got %v", err)
	}
}

func TestRequireProjectAccess_NilArg(t *testing.T) {
	t.Parallel()
	ctx := WithCaller(context.Background(), CallerContext{ProjectID: uuid.New()})
	err := RequireProjectAccess(ctx, uuid.Nil)
	if err == nil || !strings.Contains(err.Error(), "projectId 不能为空") {
		t.Fatalf("expected nil projectID error, got %v", err)
	}
}
