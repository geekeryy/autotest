package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"autotest/internal/spec"

	"github.com/google/uuid"
)

type fakeRepo struct {
	createInput   createInput
	createErr     error
	listItems     []Notification
	listErr       error
	unreadCount   int
	unreadErr     error
	markReadErr   error
	markAllErr     error
	deleteAllErr   error
	projectName   string
	serviceName   string
	lookupErr     error
	markReadCalls []uuid.UUID
}

func (f *fakeRepo) Create(_ context.Context, input createInput) (*Notification, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createInput = input
	id := uuid.New()
	return &Notification{ID: id, UserID: input.UserID, Type: input.Type, Title: input.Title, Body: input.Body, Payload: input.Payload}, nil
}

func (f *fakeRepo) ListByUser(_ context.Context, _ uuid.UUID, _ int) ([]Notification, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listItems, nil
}

func (f *fakeRepo) CountUnread(_ context.Context, _ uuid.UUID) (int, error) {
	if f.unreadErr != nil {
		return 0, f.unreadErr
	}
	return f.unreadCount, nil
}

func (f *fakeRepo) MarkRead(_ context.Context, _, id uuid.UUID) error {
	f.markReadCalls = append(f.markReadCalls, id)
	return f.markReadErr
}

func (f *fakeRepo) MarkAllRead(context.Context, uuid.UUID) error {
	return f.markAllErr
}

func (f *fakeRepo) DeleteAllByUser(context.Context, uuid.UUID) error {
	return f.deleteAllErr
}

func (f *fakeRepo) LookupProjectServiceNames(context.Context, uuid.UUID, uuid.UUID) (string, string, error) {
	if f.lookupErr != nil {
		return "", "", f.lookupErr
	}
	return f.projectName, f.serviceName, nil
}

func TestServiceCreateSpecImportNotification(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	projectID := uuid.New()
	serviceID := uuid.New()
	specID := uuid.New()
	repo := &fakeRepo{projectName: "Demo", serviceName: "Orders"}
	svc := NewService(nil)
	svc.repo = repo

	summary := &spec.ImportSummary{
		SpecID:           specID,
		ApiCount:         12,
		CreatedEndpoints: 3,
		UpdatedEndpoints: 2,
		GeneratedCases:   5,
	}
	if err := svc.CreateSpecImportNotification(context.Background(), userID, projectID, serviceID, summary); err != nil {
		t.Fatalf("CreateSpecImportNotification: %v", err)
	}
	if repo.createInput.UserID != userID {
		t.Fatalf("userID = %v", repo.createInput.UserID)
	}
	if repo.createInput.Type != TypeSpecImport || repo.createInput.Title != specImportTitle {
		t.Fatalf("type/title = %q / %q", repo.createInput.Type, repo.createInput.Title)
	}
	wantBody := "项目「Demo」服务「Orders」已通过 API Key 导入：本次变动 5 个接口（新增 3、更新 2），生成模板 5 条。"
	if repo.createInput.Body != wantBody {
		t.Fatalf("body = %q", repo.createInput.Body)
	}
	var payload SpecImportPayload
	if err := json.Unmarshal(repo.createInput.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.ProjectID != projectID || payload.ServiceID != serviceID || payload.SpecID != specID {
		t.Fatalf("payload ids mismatch: %+v", payload)
	}
	if payload.ChangedEndpoints != 5 {
		t.Fatalf("changedEndpoints = %d", payload.ChangedEndpoints)
	}
}

func TestServiceCreateSpecImportNotificationNilSummary(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	if err := svc.CreateSpecImportNotification(context.Background(), uuid.New(), uuid.New(), uuid.New(), nil); err == nil {
		t.Fatal("expected error for nil summary")
	}
}

func TestServiceList(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	repo := &fakeRepo{
		listItems:   []Notification{{ID: id, Title: "t"}},
		unreadCount: 2,
	}
	svc := NewService(nil)
	svc.repo = repo

	resp, err := svc.List(context.Background(), uuid.New(), 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Items) != 1 || resp.UnreadCount != 2 {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestServiceMarkReadPropagatesNotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{markReadErr: ErrNotFound}
	svc := NewService(nil)
	svc.repo = repo

	err := svc.MarkRead(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestServiceClearAllPublishesEmptySnapshot(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := &fakeRepo{}
	svc := NewService(nil)
	svc.repo = repo

	ch, unsubscribe := svc.Subscribe(userID)
	defer unsubscribe()

	if err := svc.ClearAll(context.Background(), userID); err != nil {
		t.Fatalf("ClearAll: %v", err)
	}

	select {
	case evt := <-ch:
		if evt.Kind != "snapshot" {
			t.Fatalf("kind = %q", evt.Kind)
		}
		if evt.UnreadCount != 0 {
			t.Fatalf("unreadCount = %d", evt.UnreadCount)
		}
		if len(evt.Items) != 0 {
			t.Fatalf("items = %+v", evt.Items)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for snapshot")
	}
}
