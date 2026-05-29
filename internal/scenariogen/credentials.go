package scenariogen

import (
	"encoding/json"
	"strings"
)

const (
	FillLoginUser     = "__FILL_LOGIN_USER__"
	FillLoginPassword = "__FILL_LOGIN_PASSWORD__"
)

// CredentialPair holds login credentials for one role (user/admin).
// Username/Password cover classic login APIs; Body carries extra fields
// such as OAuth code, union_id, phone_code discovered from OpenAPI schema.
type CredentialPair struct {
	Username string         `json:"username,omitempty"`
	Password string         `json:"password,omitempty"`
	Body     map[string]any `json:"body,omitempty"`
}

// LoginCredentialBundle holds user-confirmed login credentials for scenario generation.
type LoginCredentialBundle struct {
	User  *CredentialPair `json:"user,omitempty"`
	Admin *CredentialPair `json:"admin,omitempty"`
}

// CoverageOptions controls optional inputs for coverage generation.
type CoverageOptions struct {
	LoginCredentials *LoginCredentialBundle
}

func (b *LoginCredentialBundle) pair(admin bool) *CredentialPair {
	if b == nil {
		return nil
	}
	if admin {
		return b.Admin
	}
	return b.User
}

// ExtractCredentialsFromRequest reads username/password from a saved case request JSON body.
func ExtractCredentialsFromRequest(raw json.RawMessage) (username, password string, ok bool) {
	if len(raw) == 0 {
		return "", "", false
	}
	var req map[string]any
	if err := json.Unmarshal(raw, &req); err != nil {
		return "", "", false
	}
	body, _ := req["body"].(map[string]any)
	if body == nil {
		return "", "", false
	}
	user := firstString(body, "username", "userName", "user", "email", "login")
	pass := firstString(body, "password", "pass", "pwd")
	if user == "" || pass == "" {
		return "", "", false
	}
	if IsLoginPlaceholder(user) || IsLoginPlaceholder(pass) {
		return "", "", false
	}
	return user, pass, true
}

// ResolveLoginCredentials picks credentials for a login step.
// Priority: saved case body → user-confirmed bundle → placeholders.
func ResolveLoginCredentials(admin bool, bundle *LoginCredentialBundle, caseUser, casePass string) (username, password string, needsConfirm bool) {
	if caseUser != "" && casePass != "" {
		return caseUser, casePass, false
	}
	if pair := bundle.pair(admin); pair != nil {
		user := strings.TrimSpace(pair.Username)
		pass := strings.TrimSpace(pair.Password)
		if user != "" && pass != "" {
			return user, pass, false
		}
	}
	return FillLoginUser, FillLoginPassword, true
}

func IsLoginPlaceholder(s string) bool {
	return strings.HasPrefix(strings.TrimSpace(s), "__FILL_")
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func maskPassword(pass string) string {
	pass = strings.TrimSpace(pass)
	if pass == "" {
		return ""
	}
	runes := []rune(pass)
	if len(runes) <= 2 {
		return "••••"
	}
	return string(runes[:1]) + "••••" + string(runes[len(runes)-1:])
}
