package notification

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateAndPublishNotifiesSubscriber(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := &fakeRepo{projectName: "P", serviceName: "S"}
	svc := NewService(nil)
	svc.repo = repo

	ch, unsubscribe := svc.Subscribe(userID)
	defer unsubscribe()

	if err := svc.createAndPublish(context.Background(), createInput{
		UserID: userID,
		Type:   TypeSpecImport,
		Title:  specImportTitle,
		Body:   "test",
	}); err != nil {
		t.Fatalf("createAndPublish: %v", err)
	}

	select {
	case evt := <-ch:
		if evt.Kind != "notification" || evt.Notification == nil {
			t.Fatalf("event = %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for publish")
	}
}
