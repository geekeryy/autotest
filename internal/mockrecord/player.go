package mockrecord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// Player 负责从录制数据中回放响应。
type Player struct {
	repo *Repository
}

// NewPlayer 创建一个 Player。
func NewPlayer(repo *Repository) *Player {
	return &Player{repo: repo}
}

// Replay 从录制数据中查找匹配的响应并返回。
// 匹配策略：精确匹配（Method + Path + QueryHash）优先，降级到模糊匹配（Method + Path）。
func (p *Player) Replay(
	ctx context.Context,
	serverID, routeID uuid.UUID,
	r *http.Request,
	reqBody []byte,
) (*http.Response, *RecordedResponse, error) {
	queryHash := hashQuery(r.URL.RawQuery)

	// 精确匹配（限定在当前路由的录制数据内）
	recorded, err := p.repo.FindExactMatch(ctx, serverID, routeID, r.Method, r.URL.Path, queryHash)
	if err != nil {
		return nil, nil, fmt.Errorf("exact match lookup: %w", err)
	}

	// 降级到模糊匹配
	if recorded == nil {
		recorded, err = p.repo.FindFuzzyMatch(ctx, serverID, routeID, r.Method, r.URL.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("fuzzy match lookup: %w", err)
		}
	}

	if recorded == nil {
		return nil, nil, nil // 无匹配
	}

	// 更新命中次数
	go func() {
		_ = p.repo.IncrementHitCount(context.Background(), recorded.ID)
	}()

	// 构建 http.Response
	headers := make(http.Header)
	if len(recorded.Headers) > 0 {
		var headerMap map[string]string
		if err := json.Unmarshal(recorded.Headers, &headerMap); err == nil {
			for k, v := range headerMap {
				headers.Set(k, v)
			}
		}
	}

	return &http.Response{
		StatusCode: recorded.StatusCode,
		Header:     headers,
		Body:       io.NopCloser(bytes.NewReader(recorded.Body)),
	}, recorded, nil
}

// ListRecordings 列出路由的录制数据。
func (p *Player) ListRecordings(ctx context.Context, serverID, routeID uuid.UUID) ([]RecordedResponse, error) {
	return p.repo.ListByRoute(ctx, serverID, routeID)
}

// DeleteRecording 删除一条录制数据。
func (p *Player) DeleteRecording(ctx context.Context, id uuid.UUID) error {
	return p.repo.DeleteByID(ctx, id)
}

// ClearRecordings 清空路由的所有录制数据。
func (p *Player) ClearRecordings(ctx context.Context, serverID, routeID uuid.UUID) error {
	return p.repo.ClearByRoute(ctx, serverID, routeID)
}
