package aiassert

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	testcase "autotest/internal/case"
	"autotest/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CasePatcher 持久化测试用例断言。
type CasePatcher interface {
	PatchAssertions(ctx context.Context, caseID uuid.UUID, assertions json.RawMessage) error
}

// InferService 封装断言推断和应用的业务逻辑。
type InferService struct {
	engine  *InferEngine
	patcher CasePatcher
}

// NewInferService 创建断言推断服务。
func NewInferService(engine *InferEngine, patcher CasePatcher) *InferService {
	return &InferService{engine: engine, patcher: patcher}
}

// InferAssertions 推断断言。
func (s *InferService) InferAssertions(ctx context.Context, input InferInput) (*InferOutput, error) {
	return s.engine.InferAssertions(ctx, input)
}

// ApplyAssertions 合并并持久化断言。
func (s *InferService) ApplyAssertions(ctx context.Context, caseID uuid.UUID, assertions []InferredAssertion, replaceMode bool) (json.RawMessage, error) {
	if caseID == uuid.Nil {
		return nil, ErrCaseIDRequired
	}
	if len(assertions) == 0 {
		return nil, ErrAssertionsEmpty
	}
	if s.patcher == nil {
		return nil, errors.New("case 服务未配置")
	}

	tc, err := s.engine.cases.Get(ctx, caseID)
	if err != nil {
		return nil, err
	}

	merged, err := MergeWithExisting(tc.Assertions, assertions, replaceMode)
	if err != nil {
		return nil, err
	}
	if err := s.patcher.PatchAssertions(ctx, caseID, merged); err != nil {
		return nil, err
	}
	return merged, nil
}

// Handler 提供断言推断的 HTTP API。
type Handler struct {
	service *InferService
}

// NewHandler 创建断言推断 HTTP 处理器。
func NewHandler(service *InferService) *Handler {
	return &Handler{service: service}
}

// Register 注册断言推断相关的 HTTP 路由。
func (h *Handler) Register(r chi.Router) {
	r.Post("/cases/{caseID}/infer-assertions", h.inferAssertions)
	r.Post("/cases/{caseID}/apply-assertions", h.applyAssertions)
}

// inferAssertionsRequest 是推断断言的请求体。
type inferAssertionsRequest struct {
	SampleCount int      `json:"sampleCount,omitempty"` // 历史样本数量，默认 10
	Sources     []string `json:"sources,omitempty"`     // 推断来源：schema, history, semantic
}

// inferAssertions 处理 POST /cases/{caseID}/infer-assertions 请求。
func (h *Handler) inferAssertions(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var req inferAssertionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = inferAssertionsRequest{}
	}

	output, err := h.service.InferAssertions(r.Context(), InferInput{
		CaseID:      caseID,
		SampleCount: req.SampleCount,
		Sources:     req.Sources,
	})
	if err != nil {
		if errors.Is(err, ErrCaseIDRequired) {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	httpx.JSON(w, http.StatusOK, output)
}

// applyAssertionsRequest 是应用断言的请求体。
type applyAssertionsRequest struct {
	Assertions  []InferredAssertion `json:"assertions"`
	ReplaceMode *bool               `json:"replaceMode,omitempty"`
	Mode        string              `json:"mode,omitempty"` // replace | append（前端字段）
}

func (r applyAssertionsRequest) isReplace() bool {
	if r.ReplaceMode != nil {
		return *r.ReplaceMode
	}
	return r.Mode == "replace"
}

// applyAssertions 处理 POST /cases/{caseID}/apply-assertions 请求。
func (h *Handler) applyAssertions(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	var req applyAssertionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	merged, err := h.service.ApplyAssertions(r.Context(), caseID, req.Assertions, req.isReplace())
	if err != nil {
		if errors.Is(err, ErrCaseIDRequired) || errors.Is(err, ErrAssertionsEmpty) {
			httpx.Error(w, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, testcase.ErrTestCaseNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"caseId":     caseID,
		"assertions": json.RawMessage(merged),
		"applied":    len(req.Assertions),
		"mode":       modeString(req.isReplace()),
	})
}

// modeString 返回替换模式的描述。
func modeString(replace bool) string {
	if replace {
		return "replace"
	}
	return "append"
}
