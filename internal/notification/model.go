package notification

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const TypeSpecImport = "spec_import"
const TypeUserPending = "user_pending"

const specImportTitle = "OpenAPI/Swagger 导入完成"
const userPendingTitle = "新用户待审核"

// Notification 是用户站内通知的 API 视图。
type Notification struct {
	ID        uuid.UUID       `json:"id"`
	UserID    uuid.UUID       `json:"-"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	Payload   json.RawMessage `json:"payload"`
	ReadAt    *time.Time      `json:"readAt,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

// SpecImportPayload 供前端点击通知后跳转 API 管理页。
type SpecImportPayload struct {
	ProjectID        uuid.UUID `json:"projectId"`
	ServiceID        uuid.UUID `json:"serviceId"`
	SpecID           uuid.UUID `json:"specId"`
	ApiCount         int       `json:"apiCount"`
	ChangedEndpoints int       `json:"changedEndpoints"`
	CreatedEndpoints int       `json:"createdEndpoints"`
	UpdatedEndpoints int       `json:"updatedEndpoints"`
	GeneratedCases   int       `json:"generatedCases"`
}

// UserPendingPayload 供前端点击通知后跳转用户管理页。
type UserPendingPayload struct {
	UserID      uuid.UUID `json:"userId"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
}

// ListResponse 为 GET /notifications 的响应体。
type ListResponse struct {
	Items       []Notification `json:"items"`
	UnreadCount int            `json:"unreadCount"`
}
