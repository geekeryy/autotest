package auth

import (
	"context"

	"github.com/google/uuid"
)

// RequestMeta 携带 HTTP 请求上下文，供审计等横切逻辑使用。
type RequestMeta struct {
	ClientIP  string
	UserAgent string
}

// LoginAuditor 记录登录审计事件；由 internal/auditlog 实现并注入。
type LoginAuditor interface {
	RecordLoginAttempt(ctx context.Context, success bool, method string, actorUserID *uuid.UUID, actorUsername, clientIP, userAgent, failureReason, attemptedPassword string) error
}
