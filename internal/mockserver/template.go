package mockserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var mockResponseTemplateRE = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

type mockResponseTemplateContext struct {
	route          MockRoute
	request        *http.Request
	body           []byte
	bodyJSON       any
	bodyJSONParsed bool
	bodyJSONErr    error
}

func renderMockResponseBody(input string, route MockRoute, r *http.Request, body []byte) (string, error) {
	if input == "" || !strings.Contains(input, "{{") {
		return input, nil
	}

	matches := mockResponseTemplateRE.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	ctx := &mockResponseTemplateContext{
		route:   route,
		request: r,
		body:    body,
	}
	var out strings.Builder
	out.Grow(len(input))
	last := 0
	for _, match := range matches {
		out.WriteString(input[last:match[0]])
		expr := strings.TrimSpace(input[match[2]:match[3]])
		if !strings.HasPrefix(expr, "request.") {
			out.WriteString(input[match[0]:match[1]])
			last = match[1]
			continue
		}
		value, err := ctx.resolve(expr)
		if err != nil {
			return "", err
		}
		out.WriteString(mockTemplateValueString(value))
		last = match[1]
	}
	out.WriteString(input[last:])
	return out.String(), nil
}

func (ctx *mockResponseTemplateContext) resolve(expr string) (any, error) {
	switch {
	case expr == "request.method":
		return ctx.request.Method, nil
	case expr == "request.path":
		return ctx.request.URL.Path, nil
	case expr == "request.url":
		return ctx.request.URL.RequestURI(), nil
	case expr == "request.body" || expr == "request.bodyRaw":
		return string(ctx.body), nil
	case strings.HasPrefix(expr, "request.pathvar."):
		return ctx.resolvePathVar(strings.TrimPrefix(expr, "request.pathvar."), expr)
	case strings.HasPrefix(expr, "request.path."):
		return ctx.resolvePathVar(strings.TrimPrefix(expr, "request.path."), expr)
	case strings.HasPrefix(expr, "request.query."):
		return ctx.resolveQuery(strings.TrimPrefix(expr, "request.query."), expr)
	case strings.HasPrefix(expr, "request.header."):
		return ctx.resolveHeader(strings.TrimPrefix(expr, "request.header."), expr)
	case strings.HasPrefix(expr, "request.body."):
		return ctx.resolveBodyField(strings.TrimPrefix(expr, "request.body."), expr)
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
