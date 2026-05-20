package notification

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHubPublishDeliveredToSubscriber(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	userID := uuid.New()
	ch, unsubscribe := hub.Subscribe(userID)
	defer unsubscribe()

	want := StreamEvent{Kind: "notification", UnreadCount: 3}
	hub.Publish(userID, want)

	select {
	case got := <-ch:
		if got.Kind != want.Kind || got.UnreadCount != want.UnreadCount {
			t.Fatalf("event = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestHubPublishDoesNotBlockOtherUsers(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	userA := uuid.New()
	userB := uuid.New()
	chA, unsubA := hub.Subscribe(userA)
	defer unsubA()

	hub.Publish(userB, StreamEvent{Kind: "notification", UnreadCount: 1})

	select {
	case <-chA:
		t.Fatal("user A should not receive user B event")
	case <-time.After(50 * time.Millisecond):
	}
}
