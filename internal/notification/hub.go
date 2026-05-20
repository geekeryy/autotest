package notification

import (
	"sync"

	"github.com/google/uuid"
)

const subscriberBuffer = 8

// StreamEvent 为 GET /notifications/stream 的 SSE 载荷。
type StreamEvent struct {
	Kind         string         `json:"kind"`
	UnreadCount  int            `json:"unreadCount,omitempty"`
	Items        []Notification `json:"items,omitempty"`
	Notification *Notification  `json:"notification,omitempty"`
}

// Hub 按用户 ID 向 SSE 订阅者推送通知事件。
type Hub struct {
	mu   sync.RWMutex
	subs map[uuid.UUID]map[chan StreamEvent]struct{}
}

// NewHub constructs an empty Hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[uuid.UUID]map[chan StreamEvent]struct{})}
}

// Subscribe 注册用户的 SSE 订阅；返回事件通道与取消函数。
func (h *Hub) Subscribe(userID uuid.UUID) (<-chan StreamEvent, func()) {
	ch := make(chan StreamEvent, subscriberBuffer)
	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = make(map[chan StreamEvent]struct{})
	}
	h.subs[userID][ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		if set, ok := h.subs[userID]; ok {
			delete(set, ch)
			if len(set) == 0 {
				delete(h.subs, userID)
			}
		}
		h.mu.Unlock()
		close(ch)
	}
	return ch, unsubscribe
}

// Publish 向指定用户的全部订阅者投递事件；满缓冲时丢弃以免阻塞写入路径。
func (h *Hub) Publish(userID uuid.UUID, event StreamEvent) {
	h.mu.RLock()
	set := h.subs[userID]
	chans := make([]chan StreamEvent, 0, len(set))
	for ch := range set {
		chans = append(chans, ch)
	}
	h.mu.RUnlock()

	for _, ch := range chans {
		select {
		case ch <- event:
		default:
		}
	}
}
