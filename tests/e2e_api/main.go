package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type user struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type userInput struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Active *bool  `json:"active"`
}

type loginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string   `json:"token"`
	User  authUser `json:"user"`
}

type authUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type adminStats struct {
	TotalUsers  int       `json:"totalUsers"`
	ActiveUsers int       `json:"activeUsers"`
	AdminUsers  int       `json:"adminUsers"`
	GeneratedAt time.Time `json:"generatedAt"`
}

type auditLog struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	CreatedAt time.Time `json:"createdAt"`
}

type tokenClaims struct {
	Subject  string `json:"sub"`
	Username string `json:"username"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

type jwtAuthenticator struct {
	secret   []byte
	ttl      time.Duration
	username string
	password string
	user     authUser
}

type userStore struct {
	mu     sync.RWMutex
	nextID int
	users  map[string]user
}

func newUserStore() *userStore {
	store := &userStore{
		nextID: 3,
		users:  make(map[string]user),
	}
	now := time.Now().UTC().Truncate(time.Second)
	store.users["1"] = user{
		ID:        "1",
		Name:      "Alice",
		Email:     "alice@example.test",
		Role:      "admin",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	store.users["2"] = user{
		ID:        "2",
		Name:      "Bob",
		Email:     "bob@example.test",
		Role:      "tester",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return store
}

func (s *userStore) list(role string) []user {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]user, 0, len(s.users))
	for _, item := range s.users {
		if role != "" && item.Role != role {
			continue
		}
		users = append(users, item)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})
	return users
}

func (s *userStore) stats() adminStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := adminStats{
		TotalUsers:  len(s.users),
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
	}
	for _, item := range s.users {
		if item.Active {
			stats.ActiveUsers++
		}
		if item.Role == "admin" {
			stats.AdminUsers++
		}
	}
	return stats
}

func (s *userStore) get(id string) (user, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.users[id]
	return item, ok
}

func (s *userStore) create(input userInput) (user, error) {
	if err := validateUserInput(input, true); err != nil {
		return user{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Truncate(time.Second)
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	item := user{
		ID:        fmt.Sprintf("%d", s.nextID),
		Name:      input.Name,
		Email:     input.Email,
		Role:      defaultRole(input.Role),
		Active:    active,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.nextID++
	s.users[item.ID] = item
	return item, nil
}

func (s *userStore) update(id string, input userInput) (user, bool, error) {
	if err := validateUserInput(input, false); err != nil {
		return user{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.users[id]
	if !ok {
		return user{}, false, nil
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.Email != "" {
		item.Email = input.Email
	}
	if input.Role != "" {
		item.Role = input.Role
	}
	if input.Active != nil {
		item.Active = *input.Active
	}
	item.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	s.users[id] = item
	return item, true, nil
}

func (s *userStore) delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return false
	}
	delete(s.users, id)
	return true
}

func validateUserInput(input userInput, requireNameAndEmail bool) error {
	if requireNameAndEmail && strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if requireNameAndEmail && strings.TrimSpace(input.Email) == "" {
		return errors.New("email is required")
	}
	if input.Email != "" && !strings.Contains(input.Email, "@") {
		return errors.New("email must contain @")
	}
	return nil
}

func defaultRole(role string) string {
	if strings.TrimSpace(role) == "" {
		return "tester"
	}
	return role
}

func main() {
	addr := flag.String("addr", ":18080", "HTTP listen address")
	baseURL := flag.String("base-url", "", "public base URL used in swagger, for example http://localhost:18080")
	swaggerFile := flag.String("swagger-file", "tests/e2e_api/swagger.json", "path to write generated swagger JSON")
	writeSwaggerOnly := flag.Bool("write-swagger-only", false, "write swagger file and exit without starting the server")
	flag.Parse()

	publicURL := *baseURL
	if publicURL == "" {
		publicURL = baseURLFromAddr(*addr)
	}

	swagger, err := buildSwagger(publicURL)
	if err != nil {
		log.Fatalf("build swagger: %v", err)
	}
	if *swaggerFile != "" {
		if err := writeSwaggerFile(*swaggerFile, swagger); err != nil {
			log.Fatalf("write swagger file: %v", err)
		}
		log.Printf("swagger written to %s", *swaggerFile)
	}
	if *writeSwaggerOnly {
		return
	}

	store := newUserStore()
	authenticator := newJWTAuthenticator()
	adminAuthenticator := newAdminJWTAuthenticator()
	mux := http.NewServeMux()
	registerHandlers(mux, store, swagger, authenticator, adminAuthenticator)

	log.Printf("e2e api listening on %s", *addr)
	log.Printf("swagger endpoint: %s/swagger.json", strings.TrimRight(publicURL, "/"))
	if err := http.ListenAndServe(*addr, authenticateRequests(mux, authenticator, adminAuthenticator)); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func registerHandlers(mux *http.ServeMux, store *userStore, swagger []byte, authenticator *jwtAuthenticator, adminAuthenticator *jwtAuthenticator) {
	// ── 公共端点 ──────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(swagger)
	})

	// ── 用户认证 ──────────────────────────────────────────────────────────────
	mux.HandleFunc("POST /api/v1/auth/login", authenticator.Login)

	// ── 用户 API（只读 + 个人信息）──────────────────────────────────────────
	mux.HandleFunc("GET /api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, authenticator.user)
	})
	mux.HandleFunc("GET /api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.list(r.URL.Query().Get("role")))
	})
	mux.HandleFunc("GET /api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		item, ok := store.get(r.PathValue("id"))
		if !ok {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	// ── 管理员认证 ────────────────────────────────────────────────────────────
	mux.HandleFunc("POST /api/v1/admin/auth/login", adminAuthenticator.Login)

	// ── 管理员用户管理（完整 CRUD）────────────────────────────────────────────
	mux.HandleFunc("GET /api/v1/admin/users", func(w http.ResponseWriter, r *http.Request) {
		users := store.list(r.URL.Query().Get("role"))
		writeJSON(w, http.StatusOK, map[string]any{
			"total": len(users),
			"users": users,
		})
	})
	mux.HandleFunc("POST /api/v1/admin/users", func(w http.ResponseWriter, r *http.Request) {
		var input userInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		item, err := store.create(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})
	mux.HandleFunc("GET /api/v1/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		item, ok := store.get(r.PathValue("id"))
		if !ok {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("PUT /api/v1/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		var input userInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		item, ok, err := store.update(r.PathValue("id"), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !store.delete(r.PathValue("id")) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// ── 管理员统计与审计 ──────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/v1/admin/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.stats())
	})
	mux.HandleFunc("GET /api/v1/admin/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, adminAuditLogs())
	})
}

func newJWTAuthenticator() *jwtAuthenticator {
	return &jwtAuthenticator{
		secret:   []byte("autotest-e2e-api-secret"),
		ttl:      24 * time.Hour,
		username: "admin",
		password: "admin123",
		user: authUser{
			ID:       "e2e-admin",
			Username: "admin",
			Role:     "admin",
		},
	}
}

func newAdminJWTAuthenticator() *jwtAuthenticator {
	return &jwtAuthenticator{
		secret:   []byte("autotest-e2e-admin-api-secret"),
		ttl:      24 * time.Hour,
		username: "admin-root",
		password: "admin123",
		user: authUser{
			ID:       "e2e-admin-root",
			Username: "admin-root",
			Role:     "super_admin",
		},
	}
}

func adminAuditLogs() []auditLog {
	now := time.Now().UTC().Truncate(time.Second)
	return []auditLog{
		{
			ID:        "audit-1",
			Actor:     "admin-root",
			Action:    "import.swagger",
			Resource:  "service:user-api",
			CreatedAt: now.Add(-20 * time.Minute),
		},
		{
			ID:        "audit-2",
			Actor:     "admin-root",
			Action:    "run.suite",
			Resource:  "suite:regression",
			CreatedAt: now.Add(-10 * time.Minute),
		},
		{
			ID:        "audit-3",
			Actor:     "system",
			Action:    "report.created",
			Resource:  "report:latest",
			CreatedAt: now.Add(-5 * time.Minute),
		},
	}
}

func (a *jwtAuthenticator) Login(w http.ResponseWriter, r *http.Request) {
	var input loginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if input.Username != a.username || input.Password != a.password {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := a.sign()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{Token: token, User: a.user})
}

func authenticateRequests(next http.Handler, authenticator *jwtAuthenticator, adminAuthenticator *jwtAuthenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requiresJWT(r) {
			next.ServeHTTP(w, r)
			return
		}

		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		selectedAuthenticator := authenticator
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
			selectedAuthenticator = adminAuthenticator
		}
		if _, err := selectedAuthenticator.parse(token); err != nil {
			if selectedAuthenticator == adminAuthenticator {
				if _, uerr := authenticator.parse(token); uerr == nil {
					writeError(w, http.StatusUnauthorized, "user API token cannot access admin routes; use POST /api/v1/admin/auth/login (e.g. admin-root / admin123)")
					return
				}
			}
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requiresJWT(r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		return false
	}
	return !(r.Method == http.MethodPost && (r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/admin/auth/login"))
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (a *jwtAuthenticator) sign() (string, error) {
	return generateJWT(a.secret, a.user, a.ttl)
}

func generateJWT(secret []byte, user authUser, ttl time.Duration) (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := tokenClaims{
		Subject:  user.ID,
		Username: user.Username,
		IssuedAt: now.Unix(),
		Expires:  now.Add(ttl).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	payload := encodedHeader + "." + encodedClaims
	return payload + "." + signPayload(secret, payload), nil
}

func (a *jwtAuthenticator) parse(token string) (*tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid bearer token")
	}

	payload := parts[0] + "." + parts[1]
	expected := signPayload(a.secret, payload)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("invalid bearer token")
	}

	rawClaims, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid bearer token")
	}
	var claims tokenClaims
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		return nil, errors.New("invalid bearer token")
	}
	if claims.Expires < time.Now().Unix() {
		return nil, errors.New("token expired")
	}
	if claims.Subject != a.user.ID || claims.Username != a.user.Username {
		return nil, errors.New("invalid bearer token")
	}
	return &claims, nil
}

func signPayload(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func writeSwaggerFile(path string, swagger []byte) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, swagger, 0o644)
}

func baseURLFromAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://localhost:18080"
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func buildSwagger(baseURL string) ([]byte, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "http"
	}
	host := parsed.Host
	if host == "" {
		host = "localhost:18080"
	}

	doc := map[string]any{
		"swagger": "2.0",
		"info": map[string]string{
			"title":       "Autotest E2E Demo API",
			"description": "A local target API service for importing into the automation test platform.",
			"version":     "1.0.0",
		},
		"host":        host,
		"basePath":    "/",
		"schemes":     []string{scheme},
		"consumes":    []string{"application/json"},
		"produces":    []string{"application/json"},
		"tags":        swaggerTags(),
		"paths":       swaggerPaths(),
		"definitions": swaggerDefinitions(),
		"securityDefinitions": map[string]any{
			"BearerAuth": map[string]any{
				"type":        "apiKey",
				"name":        "Authorization",
				"in":          "header",
				"description": "Use the value returned by /api/v1/auth/login as: Bearer <token>",
			},
			"AdminBearerAuth": map[string]any{
				"type":        "apiKey",
				"name":        "Authorization",
				"in":          "header",
				"description": "Use the value returned by /api/v1/admin/auth/login as: Bearer <token>",
			},
		},
	}
	return json.MarshalIndent(doc, "", "  ")
}

func swaggerTags() []map[string]string {
	return []map[string]string{
		{"name": "health", "description": "Service health checks"},
		{"name": "auth", "description": "User JWT authentication"},
		{"name": "users", "description": "User read-only operations (list, detail, personal info)"},
		{"name": "admin-auth", "description": "Admin JWT authentication"},
		{"name": "admin-users", "description": "Admin user management (full CRUD)"},
		{"name": "admin", "description": "Admin statistics and audit logs"},
	}
}

func swaggerPaths() map[string]any {
	return map[string]any{
		// ── 健康检查 ──────────────────────────────────────────────────────────
		"/healthz": map[string]any{
			"get": map[string]any{
				"tags":        []string{"health"},
				"operationId": "health",
				"summary":     "Health check",
				"responses": map[string]any{
					"200": response("service is healthy", ref("#/definitions/HealthResponse")),
				},
			},
		},
		// ── 用户认证 ──────────────────────────────────────────────────────────
		"/api/v1/auth/login": map[string]any{
			"post": map[string]any{
				"tags":        []string{"auth"},
				"operationId": "login",
				"summary":     "Login and get JWT",
				"parameters": []map[string]any{
					bodyParam("credentials", "Login credentials", ref("#/definitions/LoginRequest")),
				},
				"responses": map[string]any{
					"200": response("login response", ref("#/definitions/LoginResponse")),
					"400": response("invalid request", ref("#/definitions/ErrorResponse")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		// ── 用户 API（只读 + 个人信息）────────────────────────────────────────
		"/api/v1/me": map[string]any{
			"get": map[string]any{
				"tags":        []string{"users"},
				"operationId": "getMe",
				"summary":     "Get current authenticated user",
				"security":    jwtSecurity(),
				"responses": map[string]any{
					"200": response("current user", ref("#/definitions/AuthUser")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		"/api/v1/users": map[string]any{
			"get": map[string]any{
				"tags":        []string{"users"},
				"operationId": "listUsers",
				"summary":     "List users",
				"security":    jwtSecurity(),
				"parameters": []map[string]any{
					{
						"name":        "role",
						"in":          "query",
						"required":    false,
						"type":        "string",
						"description": "Optional role filter",
					},
				},
				"responses": map[string]any{
					"200": response("users", map[string]any{
						"type":  "array",
						"items": ref("#/definitions/User"),
					}),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		"/api/v1/users/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"users"},
				"operationId": "getUser",
				"summary":     "Get user by ID",
				"security":    jwtSecurity(),
				"parameters":  []map[string]any{pathIDParam()},
				"responses": map[string]any{
					"200": response("user", ref("#/definitions/User")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
					"404": response("not found", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		// ── 管理员认证 ────────────────────────────────────────────────────────
		"/api/v1/admin/auth/login": map[string]any{
			"post": map[string]any{
				"tags":        []string{"admin-auth"},
				"operationId": "adminLogin",
				"summary":     "Login and get admin JWT",
				"parameters": []map[string]any{
					bodyParam("credentials", "Admin login credentials", ref("#/definitions/AdminLoginRequest")),
				},
				"responses": map[string]any{
					"200": response("login response", ref("#/definitions/LoginResponse")),
					"400": response("invalid request", ref("#/definitions/ErrorResponse")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		// ── 管理员用户管理（完整 CRUD）────────────────────────────────────────
		"/api/v1/admin/users": map[string]any{
			"get": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminListUsers",
				"summary":     "List users (admin)",
				"security":    adminJWTSecurity(),
				"parameters": []map[string]any{
					{
						"name":        "role",
						"in":          "query",
						"required":    false,
						"type":        "string",
						"description": "Optional role filter",
					},
				},
				"responses": map[string]any{
					"200": response("users with total count", ref("#/definitions/AdminUserListResponse")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
				},
			},
			"post": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminCreateUser",
				"summary":     "Create user (admin)",
				"security":    adminJWTSecurity(),
				"parameters": []map[string]any{
					bodyParam("user", "User payload", ref("#/definitions/CreateUserRequest")),
				},
				"responses": map[string]any{
					"201": response("created user", ref("#/definitions/User")),
					"400": response("validation error", ref("#/definitions/ErrorResponse")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		"/api/v1/admin/users/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminGetUser",
				"summary":     "Get user by ID (admin)",
				"security":    adminJWTSecurity(),
				"parameters":  []map[string]any{pathIDParam()},
				"responses": map[string]any{
					"200": response("user", ref("#/definitions/User")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
					"404": response("not found", ref("#/definitions/ErrorResponse")),
				},
			},
			"put": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminUpdateUser",
				"summary":     "Update user (admin)",
				"security":    adminJWTSecurity(),
				"parameters": []map[string]any{
					pathIDParam(),
					bodyParam("user", "User fields to update", ref("#/definitions/UpdateUserRequest")),
				},
				"responses": map[string]any{
					"200": response("updated user", ref("#/definitions/User")),
					"400": response("validation error", ref("#/definitions/ErrorResponse")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
					"404": response("not found", ref("#/definitions/ErrorResponse")),
				},
			},
			"delete": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminDeleteUser",
				"summary":     "Delete user (admin)",
				"security":    adminJWTSecurity(),
				"parameters":  []map[string]any{pathIDParam()},
				"responses": map[string]any{
					"204": map[string]string{"description": "deleted"},
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
					"404": response("not found", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		// ── 管理员统计与审计 ──────────────────────────────────────────────────
		"/api/v1/admin/stats": map[string]any{
			"get": map[string]any{
				"tags":        []string{"admin"},
				"operationId": "adminStats",
				"summary":     "Get platform statistics (admin)",
				"security":    adminJWTSecurity(),
				"responses": map[string]any{
					"200": response("stats", ref("#/definitions/AdminStats")),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		"/api/v1/admin/audit-logs": map[string]any{
			"get": map[string]any{
				"tags":        []string{"admin"},
				"operationId": "adminAuditLogs",
				"summary":     "List audit logs (admin)",
				"security":    adminJWTSecurity(),
				"responses": map[string]any{
					"200": response("audit logs", map[string]any{
						"type":  "array",
						"items": ref("#/definitions/AuditLog"),
					}),
					"401": response("unauthorized", ref("#/definitions/ErrorResponse")),
				},
			},
		},
	}
}

func swaggerDefinitions() map[string]any {
	return map[string]any{
		"AuthUser": objectSchema([]string{"id", "username", "role"}, map[string]any{
			"id":       stringSchema("e2e-admin"),
			"username": stringSchema("admin"),
			"role":     stringSchema("admin"),
		}),
		"HealthResponse": objectSchema(nil, map[string]any{
			"status": stringSchema("ok"),
		}),
		"LoginRequest": objectSchema([]string{"username", "password"}, map[string]any{
			"username": stringSchema("admin"),
			"password": stringSchema("admin123"),
		}),
		"AdminLoginRequest": objectSchema([]string{"username", "password"}, map[string]any{
			"username": stringSchema("admin-root"),
			"password": stringSchema("admin123"),
		}),
		"LoginResponse": objectSchema([]string{"token", "user"}, map[string]any{
			"token": stringSchema("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."),
			"user":  ref("#/definitions/AuthUser"),
		}),
		"AdminStats": objectSchema([]string{"totalUsers", "activeUsers", "adminUsers", "generatedAt"}, map[string]any{
			"totalUsers":  integerSchema(2),
			"activeUsers": integerSchema(2),
			"adminUsers":  integerSchema(1),
			"generatedAt": dateTimeSchema(),
		}),
		"AdminUserListResponse": objectSchema([]string{"total", "users"}, map[string]any{
			"total": integerSchema(2),
			"users": map[string]any{
				"type":  "array",
				"items": ref("#/definitions/User"),
			},
		}),
		"AuditLog": objectSchema([]string{"id", "actor", "action", "resource", "createdAt"}, map[string]any{
			"id":        stringSchema("audit-1"),
			"actor":     stringSchema("admin-root"),
			"action":    stringSchema("run.suite"),
			"resource":  stringSchema("suite:regression"),
			"createdAt": dateTimeSchema(),
		}),
		"User": objectSchema([]string{"id", "name", "email", "role", "active", "createdAt", "updatedAt"}, map[string]any{
			"id":        stringSchema("1"),
			"name":      stringSchema("Alice"),
			"email":     stringSchema("alice@example.test"),
			"role":      stringSchema("tester"),
			"active":    map[string]any{"type": "boolean", "example": true},
			"createdAt": dateTimeSchema(),
			"updatedAt": dateTimeSchema(),
		}),
		"CreateUserRequest": objectSchema([]string{"name", "email"}, map[string]any{
			"name":   stringSchema("Charlie"),
			"email":  stringSchema("charlie@example.test"),
			"role":   stringSchema("tester"),
			"active": map[string]any{"type": "boolean", "example": true},
		}),
		"UpdateUserRequest": objectSchema(nil, map[string]any{
			"name":   stringSchema("Charlie Updated"),
			"email":  stringSchema("charlie.updated@example.test"),
			"role":   stringSchema("admin"),
			"active": map[string]any{"type": "boolean", "example": true},
		}),
		"ErrorResponse": objectSchema([]string{"error"}, map[string]any{
			"error": stringSchema("user not found"),
		}),
	}
}

func ref(path string) map[string]any {
	return map[string]any{"$ref": path}
}

func jwtSecurity() []map[string][]string {
	return []map[string][]string{
		{"BearerAuth": []string{}},
	}
}

func adminJWTSecurity() []map[string][]string {
	return []map[string][]string{
		{"AdminBearerAuth": []string{}},
	}
}

func response(description string, schema map[string]any) map[string]any {
	return map[string]any{
		"description": description,
		"schema":      schema,
	}
}

func bodyParam(name, description string, schema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "body",
		"required":    true,
		"description": description,
		"schema":      schema,
	}
}

func pathIDParam() map[string]any {
	return map[string]any{
		"name":        "id",
		"in":          "path",
		"required":    true,
		"type":        "string",
		"description": "User ID",
	}
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(example string) map[string]any {
	return map[string]any{
		"type":    "string",
		"example": example,
	}
}

func integerSchema(example int) map[string]any {
	return map[string]any{
		"type":    "integer",
		"example": example,
	}
}

func dateTimeSchema() map[string]any {
	return map[string]any{
		"type":    "string",
		"format":  "date-time",
		"example": "2026-01-01T00:00:00Z",
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
