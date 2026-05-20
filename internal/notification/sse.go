package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// writeStreamEvent 将单条 SSE 帧写入响应并 flush。
func writeStreamEvent(w http.ResponseWriter, flusher http.Flusher, mu *sync.Mutex, event StreamEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal sse event: %w", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, err := fmt.Fprintf(w, "event: %s\n", event.Kind); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// startNotificationSSE 设置 SSE 响应头并返回 flusher。
func startNotificationSSE(w http.ResponseWriter) (http.Flusher, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming unsupported by current writer")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return flusher, nil
}

// runNotificationHeartbeat 在 ctx 存活期间周期性发送 SSE 注释心跳。
func runNotificationHeartbeat(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, mu *sync.Mutex, done <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				_, _ = fmt.Fprint(w, ": ping\n\n")
				flusher.Flush()
				mu.Unlock()
			}
		}
	}()
}
