// Package mockrecord 实现 Mock Server 的录制回放功能。
// 通过正向代理模式拦截真实服务的响应并存储，
// 后续相同请求直接从存储中回放，无需再次访问真实服务。
package mockrecord

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RecordedResponse 存储一次录制的请求-响应对。
type RecordedResponse struct {
	ID           uuid.UUID       `json:"id"`
	ProjectID    uuid.UUID       `json:"projectId"`
	MockServerID uuid.UUID       `json:"mockServerId"`
	MockRouteID  uuid.UUID       `json:"mockRouteId"`
	Method       string          `json:"method"`
	Path         string          `json:"path"`
	QueryHash    string          `json:"queryHash"`
	RequestBody  json.RawMessage `json:"requestBody,omitempty"`
	StatusCode   int             `json:"statusCode"`
	Headers      json.RawMessage `json:"headers,omitempty"`
	Body         json.RawMessage `json:"body"`
	RecordedAt   time.Time       `json:"recordedAt"`
	HitCount     int             `json:"hitCount"`
}

// RecordConfig 录制配置。
type RecordConfig struct {
	// TargetURL 真实服务的基础 URL（如 http://api.example.com）。
	TargetURL string `json:"targetUrl"`
	// MaxRecordings 每个路由最大录制数量，0 表示不限制。
	MaxRecordings int `json:"maxRecordings"`
}

// ReplayMatchResult 回放匹配结果。
type ReplayMatchResult struct {
	// Exact 精确匹配（Method + Path + QueryHash）。
	Exact *RecordedResponse
	// Fuzzy 模糊匹配（Method + Path，忽略 query）。
	Fuzzy *RecordedResponse
}
