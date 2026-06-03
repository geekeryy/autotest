// Package adapter 提供 aiprovider.Service 到 ainl.ChatProvider 接口的适配器。
// 独立成包以避免 ainl -> aiprovider 的循环导入。
package adapter

import (
	"context"
	"encoding/json"

	"autotest/internal/aiprovider"

	"github.com/google/uuid"
)

// ChatProviderAdapter 将 aiprovider.Service 适配为 ainl.ChatProvider 接口。
type ChatProviderAdapter struct {
	AI *aiprovider.Service
}

// Chat 发送请求到 LLM。
func (a *ChatProviderAdapter) Chat(ctx context.Context, projectID uuid.UUID, req any) (any, error) {
	// 将 any 类型的请求转换为 aiprovider.ChatRequest
	var chatReq aiprovider.ChatRequest

	switch r := req.(type) {
	case aiprovider.ChatRequest:
		chatReq = r
	default:
		// 尝试通过 JSON 序列化/反序列化转换
		b, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &chatReq); err != nil {
			return nil, err
		}
	}

	return a.AI.Chat(ctx, projectID, chatReq)
}

// ResolveDefaultChatProvider 获取平台默认的 AI 提供商。
func (a *ChatProviderAdapter) ResolveDefaultChatProvider(ctx context.Context) (uuid.UUID, string, error) {
	return a.AI.ResolveDefaultChatProvider(ctx)
}
