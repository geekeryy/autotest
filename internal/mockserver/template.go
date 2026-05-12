package mockserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"autotest/internal/templating"
)

type mockResponseTemplateContext struct {
	route          MockRoute
	request        *http.Request
	body           []byte
	bodyJSON       any
	bodyJSONParsed bool
	bodyJSONErr    error
}

func renderMockResponseBody(input string, route MockRoute, r *http.Request, body []byte, mockCfg *templating.MockExpanderConfig) (string, error) {
	if input == "" || !strings.Contains(input, "{{") {
		return input, nil
	}

	ctx := &mockResponseTemplateContext{
		route:   route,
		request: r,
		body:    body,
	}
	var resolveErr error
	mockHook := templating.ExpandMock
	if mockCfg != nil {
		// 把 set 解析失败的中文错误也通过 resolveErr 反馈给调用方，
		// 与 request.* 引用失败的处理保持一致：runtime 会回 500 + 详细信息。
		cfg := *mockCfg
		cfg.OnError = func(err error) {
			if resolveErr == nil && err != nil {
				resolveErr = err
			}
		}
		mockHook = templating.NewMockExpander(cfg)
	}
	rendered := templating.Render(input, templating.Resolver{
		// Expand `{{$mock.*}}` first so each occurrence still produces an
		// independent random value, matching the legacy two-pass pipeline
		// (mockdata.Expand → request.* resolution).
		Mock: mockHook,
		Request: func(tok templating.Token) (string, bool) {
			if resolveErr != nil {
				return "", false
			}
			value, err := ctx.resolve(tok.Request.Key)
			if err != nil {
				resolveErr = err
				return "", false
			}
			return mockTemplateValueString(value), true
		},
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	return rendered, nil
}

// resolve looks up a single Mock-Server request reference. `key` is the
// canonical body without the `request.` (or `$req.`) prefix, e.g.
// `method`, `pathvar.id` or `body.user.name`.
func (ctx *mockResponseTemplateContext) resolve(key string) (any, error) {
	expr := "{{request." + key + "}}"
	switch {
	case key == "method":
		return ctx.request.Method, nil
	case key == "path":
		return ctx.request.URL.Path, nil
	case key == "url":
		return ctx.request.URL.RequestURI(), nil
	case key == "body" || key == "bodyRaw":
		return string(ctx.body), nil
	case strings.HasPrefix(key, "pathvar."):
		return ctx.resolvePathVar(strings.TrimPrefix(key, "pathvar."), expr)
	case strings.HasPrefix(key, "path."):
		return ctx.resolvePathVar(strings.TrimPrefix(key, "path."), expr)
	case strings.HasPrefix(key, "query."):
		return ctx.resolveQuery(strings.TrimPrefix(key, "query."), expr)
	case strings.HasPrefix(key, "header."):
		return ctx.resolveHeader(strings.TrimPrefix(key, "header."), expr)
	case strings.HasPrefix(key, "body."):
		return ctx.resolveBodyField(strings.TrimPrefix(key, "body."), expr)
	default:
		return nil, fmt.Errorf("不支持的响应模板引用 %q", expr)
	}
}

func (ctx *mockResponseTemplateContext) resolvePathVar(name, expr string) (string, error) {
	vars, ok := extractPathVars(ctx.route.Path, ctx.request.URL.Path)
	if !ok {
		return "", fmt.Errorf("响应模板引用 %q 无法从路径 %q 提取", expr, ctx.request.URL.Path)
	}
	value, ok := vars[name]
	if !ok {
		return "", fmt.Errorf("响应模板引用 %q 不存在", expr)
	}
	return value, nil
}

func (ctx *mockResponseTemplateContext) resolveQuery(name, expr string) (string, error) {
	values, ok := ctx.request.URL.Query()[name]
	if !ok || len(values) == 0 {
		return "", fmt.Errorf("响应模板引用 %q 不存在", expr)
	}
	return values[0], nil
}

func (ctx *mockResponseTemplateContext) resolveHeader(name, expr string) (string, error) {
	value := ctx.request.Header.Get(name)
	if value == "" {
		return "", fmt.Errorf("响应模板引用 %q 不存在", expr)
	}
	return value, nil
}

func (ctx *mockResponseTemplateContext) resolveBodyField(path, expr string) (any, error) {
	bodyJSON, err := ctx.decodeBodyJSON()
	if err != nil {
		return nil, err
	}
	value, ok := lookupJSON(bodyJSON, path)
	if !ok {
		return nil, fmt.Errorf("响应模板引用 %q 不存在", expr)
	}
	return value, nil
}

func (ctx *mockResponseTemplateContext) decodeBodyJSON() (any, error) {
	if ctx.bodyJSONParsed {
		return ctx.bodyJSON, ctx.bodyJSONErr
	}
	ctx.bodyJSONParsed = true
	if len(bytes.TrimSpace(ctx.body)) == 0 {
		ctx.bodyJSONErr = fmt.Errorf("响应模板引用请求体字段失败: 请求体为空")
		return nil, ctx.bodyJSONErr
	}
	decoder := json.NewDecoder(bytes.NewReader(ctx.body))
	decoder.UseNumber()
	if err := decoder.Decode(&ctx.bodyJSON); err != nil {
		ctx.bodyJSONErr = fmt.Errorf("响应模板引用请求体字段失败: 请求体不是合法 JSON")
		return nil, ctx.bodyJSONErr
	}
	return ctx.bodyJSON, nil
}

func mockTemplateValueString(value any) string {
	switch val := value.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		return string(raw)
	}
}
