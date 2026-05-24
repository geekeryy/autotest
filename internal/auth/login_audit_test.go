package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type recordingLoginAuditor struct {
	calls []loginAuditCall
}

type loginAuditCall struct {
	success           bool
	method            string
	actorUserID       *uuid.UUID
	actorUsername     string
	clientIP          string
	userAgent         string
	failureReason     string
	attemptedPassword string
}

func (r *recordingLoginAuditor) RecordLoginAttempt(_ context.Context, success bool, method string, actorUserID *uuid.UUID, actorUsername, clientIP, userAgent, failureReason, attemptedPassword string) error {
	r.calls = append(r.calls, loginAuditCall{
		success:           success,
		method:            method,
		actorUserID:       actorUserID,
		actorUsername:     actorUsername,
		clientIP:          clientIP,
		userAgent:         userAgent,
		failureReason:     failureReason,
		attemptedPassword: attemptedPassword,
	})
	return nil
}

func TestLoginRecordsAuditOnFailure(t *testing.T) {
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
				Active:       true,
			},
		},
		byID: map[uuid.UUID]*User{
			userID: {
				ID:           userID,
				Username:     "alice",
				PasswordHash: string(hash),
				AuthProvider: AuthProviderLocal,
				Active:       true,
			},
		},
	}
	auditor := &recordingLoginAuditor{}
	svc := &Service{
		repo:         repo,
		secret:       []byte("test-secret"),
		ttl:          time.Hour,
		loginAuditor: auditor,
	}

	_, err = svc.Login(context.Background(), LoginInput{Username: "alice", Password: "wrong"}, RequestMeta{ClientIP: "10.0.0.1", UserAgent: "test"})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	if len(auditor.calls) != 1 {
		t.Fatalf("audit calls=%d want 1", len(auditor.calls))
	}
	call := auditor.calls[0]
	if call.success || call.method != "local" || call.failureReason != "invalid_password" {
		t.Fatalf("unexpected audit call: %+v", call)
	}
	if call.actorUserID == nil || *call.actorUserID != userID {
		t.Fatalf("actorUserID=%v want %s", call.actorUserID, userID)
	}
	if call.attemptedPassword != "wrong" {
		t.Fatalf("attemptedPassword=%q want wrong", call.attemptedPassword)
	}
}

func TestLoginRecordsAuditOnUnknownUser(t *testing.T) {
	t.Parallel()

	auditor := &recordingLoginAuditor{}
	svc := &Service{
		repo:         &fakeAuthRepo{byUsername: map[string]*User{}},
		secret:       []byte("test-secret"),
		ttl:          time.Hour,
		loginAuditor: auditor,
	}

	_, err := svc.Login(context.Background(), LoginInput{Username: "nobody", Password: "x"}, RequestMeta{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	if len(auditor.calls) != 1 || auditor.calls[0].failureReason != "user_not_found" {
		t.Fatalf("calls=%+v", auditor.calls)
	}
	if auditor.calls[0].attemptedPassword != "x" {
		t.Fatalf("attemptedPassword=%q want x", auditor.calls[0].attemptedPassword)
	}
}

func TestLoginRecordsAuditOnSuccess(t *testing.T) {
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
				Active:       true,
			},
		},
		byID: map[uuid.UUID]*User{
			userID: {
				ID:           userID,
				Username:     "alice",
				PasswordHash: string(hash),
				AuthProvider: AuthProviderLocal,
				Active:       true,
			},
		},
	}
	auditor := &recordingLoginAuditor{}
	svc := &Service{
		repo:         repo,
		secret:       []byte("test-secret"),
		ttl:          time.Hour,
		loginAuditor: auditor,
	}

	resp, err := svc.Login(context.Background(), LoginInput{Username: "alice", Password: "secret"}, RequestMeta{ClientIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || resp.Token == "" {
		t.Fatal("expected token")
	}
	if len(auditor.calls) != 1 || !auditor.calls[0].success {
		t.Fatalf("calls=%+v", auditor.calls)
	}
}
