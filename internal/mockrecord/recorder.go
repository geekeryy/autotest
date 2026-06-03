package mockrecord

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"autotest/internal/urlx"

	"github.com/google/uuid"
)

// Recorder 负责录制真实服务的响应。
type Recorder struct {
	repo *Repository
}

// NewRecorder 创建一个 Recorder。
func NewRecorder(repo *Repository) *Recorder {
	return &Recorder{repo: repo}
}

// RecordAndProxy 将请求转发到真实服务，录制响应并返回。
func (rec *Recorder) RecordAndProxy(
	ctx context.Context,
	projectID, serverID, routeID uuid.UUID,
	targetURL string,
	originalReq *http.Request,
	reqBody []byte,
) (*RecordedResponse, *http.Response, error) {
	if err := urlx.ValidateHTTPServiceURL(targetURL); err != nil {
		return nil, nil, err
	}

	// 构建转发请求
	targetURL = strings.TrimRight(targetURL, "/")
	proxyURL := targetURL + originalReq.URL.RequestURI()

	proxyReq, err := http.NewRequestWithContext(ctx, originalReq.Method, proxyURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, fmt.Errorf("create proxy request: %w", err)
	}

	// 复制原始请求头（排除 hop-by-hop 头）
	for key, values := range originalReq.Header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, v := range values {
			proxyReq.Header.Add(key, v)
		}
	}

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return nil, nil, fmt.Errorf("proxy request to %s: %w", proxyURL, err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read proxy response: %w", err)
	}

	// 构建响应头
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	headersJSON, _ := json.Marshal(headers)

	// 计算 query hash
	queryHash := hashQuery(originalReq.URL.RawQuery)

	// 存储录制数据
	recorded := &RecordedResponse{
		ID:           uuid.New(),
		ProjectID:    projectID,
		MockServerID: serverID,
		MockRouteID:  routeID,
		Method:       originalReq.Method,
		Path:         originalReq.URL.Path,
		QueryHash:    queryHash,
		RequestBody:  reqBody,
		StatusCode:   resp.StatusCode,
		Headers:      headersJSON,
		Body:         respBody,
		RecordedAt:   time.Now(),
		HitCount:     0,
	}

	if err := rec.repo.InsertRecording(ctx, recorded); err != nil {
		// 录制存储失败不影响响应返回
		_ = err
	}

	// 构建返回的 http.Response
	return recorded, &http.Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}, nil
}

// hashQuery 对 query 参数进行规范化后哈希。
// 规范化：按 key 排序，忽略空值。
func hashQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}

	params := strings.Split(rawQuery, "&")
	sort.Strings(params)

	var buf strings.Builder
	for _, p := range params {
		if p != "" {
			buf.WriteString(p)
			buf.WriteByte('&')
		}
	}

	hash := sha256.Sum256([]byte(buf.String()))
	return hex.EncodeToString(hash[:8]) // 取前 8 字节足够
}

// isHopByHopHeader 检查是否为 hop-by-hop 头（不应被代理转发）。
func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailers", "transfer-encoding", "upgrade":
		return true
	}
	return false
}
