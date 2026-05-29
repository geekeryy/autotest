package scenariogen

import (
	"encoding/json"
	"fmt"
	"strings"

	"autotest/internal/spec"
)

// LoginBodyFieldHint describes one login request body field that may need user input.
type LoginBodyFieldHint struct {
	Name         string `json:"name"`
	Label        string `json:"label,omitempty"`
	Type         string `json:"type"`
	Required     bool   `json:"required"`
	Enum         []any  `json:"enum,omitempty"`
	DefaultValue any    `json:"defaultValue,omitempty"`
	CurrentValue any    `json:"currentValue,omitempty"`
	NeedsInput   bool   `json:"needsInput"`
	Sensitive    bool   `json:"sensitive,omitempty"`
}

// FillFieldPlaceholder returns a stable backend placeholder for a missing login field.
func FillFieldPlaceholder(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return "__FILL_UNKNOWN__"
	}
	return "__FILL_" + field + "__"
}

// ParseLoginBodyFields reads body properties from an endpoint requestSchema.
func ParseLoginBodyFields(requestSchema json.RawMessage) []LoginBodyFieldHint {
	var req map[string]any
	if err := json.Unmarshal(requestSchema, &req); err != nil {
		return nil
	}
	bodySchema, _ := req["body"].(map[string]any)
	if bodySchema == nil {
		return legacyLoginBodyFields()
	}
	requiredSet := map[string]bool{}
	if required, ok := bodySchema["required"].([]any); ok {
		for _, item := range required {
			if name, ok := item.(string); ok {
				requiredSet[name] = true
			}
		}
	}
	props, _ := bodySchema["properties"].(map[string]any)
	if len(props) == 0 {
		return legacyLoginBodyFields()
	}
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sortStrings(names)

	out := make([]LoginBodyFieldHint, 0, len(names))
	for _, name := range names {
		prop, _ := props[name].(map[string]any)
		if prop == nil {
			prop = map[string]any{}
		}
		h := LoginBodyFieldHint{
			Name:     name,
			Label:    fieldLabel(name, prop),
			Type:     schemaType(prop),
			Required: requiredSet[name],
			Sensitive: isSensitiveField(name),
		}
		if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
			h.Enum = enum
		}
		if def, ok := prop["default"]; ok {
			h.DefaultValue = def
		}
		out = append(out, h)
	}
	return out
}

func legacyLoginBodyFields() []LoginBodyFieldHint {
	return []LoginBodyFieldHint{
		{Name: "username", Label: "用户名", Type: "string", Required: true},
		{Name: "password", Label: "密码", Type: "string", Required: true, Sensitive: true},
	}
}

// BuildLoginRequestBody merges saved case body, user credentials, and schema defaults.
// Returns requestOverride JSON, field hints still needing input, and whether user confirm is required.
func BuildLoginRequestBody(ep spec.Endpoint, savedCaseRaw json.RawMessage, creds *LoginCredentialBundle, admin bool) (json.RawMessage, []LoginBodyFieldHint, bool) {
	fields := ParseLoginBodyFields(ep.RequestSchema)
	body := cloneMap(extractRequestBody(savedCaseRaw))
	if body == nil {
		body = map[string]any{}
	}

	for _, f := range fields {
		if hasConcreteValue(body[f.Name]) {
			continue
		}
		if f.DefaultValue != nil {
			body[f.Name] = f.DefaultValue
		}
	}
	if pair := credsPair(creds, admin); pair != nil {
		mergeCredentialPair(body, pair, fields)
	}

	needsConfirm := false
	var pending []LoginBodyFieldHint
	for _, f := range fields {
		h := f
		val, ok := body[f.Name]
		if ok && hasConcreteValue(val) {
			h.CurrentValue = val
			if f.Sensitive {
				h.CurrentValue = maskAnySecret(val)
			}
			continue
		}
		if len(f.Enum) == 1 {
			body[f.Name] = f.Enum[0]
			h.CurrentValue = f.Enum[0]
			continue
		}
		if !f.Required && !isCredentialLikeField(f.Name) {
			continue
		}
		body[f.Name] = FillFieldPlaceholder(f.Name)
		h.CurrentValue = body[f.Name]
		h.NeedsInput = true
		pending = append(pending, h)
		needsConfirm = true
	}

	raw := loginBodyOverride(body)
	return raw, pending, needsConfirm
}

func mergeCredentialPair(body map[string]any, pair *CredentialPair, fields []LoginBodyFieldHint) {
	if pair == nil {
		return
	}
	userKeys := []string{"username", "userName", "user", "email", "login"}
	passKeys := []string{"password", "pass", "pwd"}
	if strings.TrimSpace(pair.Username) != "" {
		for _, f := range fields {
			if containsKey(userKeys, strings.ToLower(f.Name)) {
				body[f.Name] = pair.Username
			}
		}
	}
	if strings.TrimSpace(pair.Password) != "" {
		for _, f := range fields {
			if containsKey(passKeys, strings.ToLower(f.Name)) {
				body[f.Name] = pair.Password
			}
		}
	}
	if len(pair.Body) == 0 {
		return
	}
	for k, v := range pair.Body {
		if hasConcreteValue(v) {
			body[k] = v
		}
	}
}

func credsPair(creds *LoginCredentialBundle, admin bool) *CredentialPair {
	if creds == nil {
		return nil
	}
	return creds.pair(admin)
}

func extractRequestBody(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var req map[string]any
	if json.Unmarshal(raw, &req) != nil {
		return nil
	}
	body, _ := req["body"].(map[string]any)
	return body
}

func loginBodyOverride(body map[string]any) json.RawMessage {
	if len(body) == 0 {
		return nil
	}
	return jsonRequestOverride(map[string]any{"body": body})
}

func hasConcreteValue(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case string:
		s := strings.TrimSpace(val)
		return s != "" && !IsLoginPlaceholder(s)
	default:
		return true
	}
}

func isCredentialLikeField(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch lower {
	case "username", "user", "email", "login", "password", "pass", "pwd",
		"code", "authcode", "authorizationcode", "phonecode", "phone_code", "unionid", "union_id", "openid", "token":
		return true
	}
	return strings.Contains(lower, "code") || strings.Contains(lower, "token") || strings.Contains(lower, "secret")
}

func isSensitiveField(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(lower, "pass") || strings.Contains(lower, "secret") ||
		strings.Contains(lower, "token") || lower == "code"
}

func fieldLabel(name string, prop map[string]any) string {
	if desc, ok := prop["description"].(string); ok {
		desc = strings.TrimSpace(desc)
		if desc != "" {
			return desc
		}
	}
	if title, ok := prop["title"].(string); ok {
		title = strings.TrimSpace(title)
		if title != "" {
			return title
		}
	}
	return name
}

func schemaType(prop map[string]any) string {
	if t, ok := prop["type"].(string); ok && t != "" {
		return t
	}
	return "string"
}

func containsKey(keys []string, name string) bool {
	for _, k := range keys {
		if k == name {
			return true
		}
	}
	return false
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func maskAnySecret(v any) string {
	s, ok := v.(string)
	if !ok {
		return "••••"
	}
	return maskPassword(s)
}

func sortStrings(items []string) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j] < items[j-1]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// CredentialValueForField resolves a login body field from user-provided credentials.
func CredentialValueForField(field string, creds *LoginCredentialBundle, admin bool) (string, bool) {
	if creds == nil {
		return "", false
	}
	pair := creds.pair(admin)
	if pair == nil {
		return "", false
	}
	lower := strings.ToLower(strings.TrimSpace(field))
	switch lower {
	case "username", "user", "email", "login":
		if pair.Username != "" {
			return pair.Username, true
		}
	case "password", "pass", "pwd":
		if pair.Password != "" {
			return pair.Password, true
		}
	}
	if pair.Body != nil {
		if v, ok := pair.Body[field]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s, true
			}
			return fmt.Sprint(v), true
		}
	}
	return "", false
}
