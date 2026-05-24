package auditlog

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const ActionAuthLogin = "auth.login"

// Entry 是审计日志的 API 视图。
type Entry struct {
	ID             uuid.UUID       `json:"id"`
	Action         string          `json:"action"`
	Success        bool            `json:"success"`
	ActorUserID    *uuid.UUID      `json:"actorUserId,omitempty"`
	ActorUsername  string          `json:"actorUsername"`
	Resource       string          `json:"resource"`
	ClientIP       string          `json:"clientIp"`
	UserAgent      string          `json:"userAgent"`
	Detail         json.RawMessage `json:"detail"`
	CreatedAt      time.Time       `json:"createdAt"`
}

// LoginEvent 描述一次登录尝试（本地密码或 OAuth）。
type LoginEvent struct {
	Success       bool
	Method        string
	ActorUserID   *uuid.UUID
	ActorUsername string
	ClientIP      string
	UserAgent     string
	FailureReason     string
	AttemptedPassword string
}

// ListFilter 为 GET /audit-logs 的查询条件。
type ListFilter struct {
	Action        string
	Success       *bool
	ActorUsername string
	Limit         int
	Offset        int
}

// ListResponse 为 GET /audit-logs 的响应体。
type ListResponse struct {
	Items  []Entry `json:"items"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}
