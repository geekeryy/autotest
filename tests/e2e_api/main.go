package main

import (
	"context"
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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

type pgUserStore struct {
	pool *pgxpool.Pool
}

func newPGUserStore(pool *pgxpool.Pool) *pgUserStore {
	return &pgUserStore{pool: pool}
}

func (s *pgUserStore) list(role string) []user {
	ctx := context.Background()
	q := `select id::text, name, email, role, active, created_at, updated_at from e2e_users`
	args := []any{}
	if role != "" {
		q += ` where role = $1`
		args = append(args, role)
	}
	q += ` order by created_at, id`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	users := make([]user, 0)
	for rows.Next() {
		var u user
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt); err != nil {
			continue
		}
		u.CreatedAt = u.CreatedAt.UTC().Truncate(time.Second)
		u.UpdatedAt = u.UpdatedAt.UTC().Truncate(time.Second)
		users = append(users, u)
	}
	return users
}

func (s *pgUserStore) stats() adminStats {
	ctx := context.Background()
	var total, activeCount, adminCount int
	err := s.pool.QueryRow(ctx, `
		select
			count(*),
			count(*) filter (where active),
			count(*) filter (where role = 'admin')
		from e2e_users
	`).Scan(&total, &activeCount, &adminCount)
	if err != nil {
		return adminStats{GeneratedAt: time.Now().UTC().Truncate(time.Second)}
	}
	return adminStats{
		TotalUsers:  total,
		ActiveUsers: activeCount,
		AdminUsers:  adminCount,
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func (s *pgUserStore) get(id string) (user, bool) {
	ctx := context.Background()
	var u user
	err := s.pool.QueryRow(ctx,
		`select id::text, name, email, role, active, created_at, updated_at from e2e_users where id = $1::uuid`,
		id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return user{}, false
	}
	u.CreatedAt = u.CreatedAt.UTC().Truncate(time.Second)
	u.UpdatedAt = u.UpdatedAt.UTC().Truncate(time.Second)
	return u, true
}

func (s *pgUserStore) create(input userInput) (user, error) {
	if err := validateUserInput(input, true); err != nil {
		return user{}, err
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	ctx := context.Background()
	var u user
	err := s.pool.QueryRow(ctx,
		`insert into e2e_users (name, email, role, active)
		 values ($1, $2, $3, $4)
		 returning id::text, name, email, role, active, created_at, updated_at`,
		input.Name, input.Email, defaultRole(input.Role), active,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return user{}, err
	}
	u.CreatedAt = u.CreatedAt.UTC().Truncate(time.Second)
	u.UpdatedAt = u.UpdatedAt.UTC().Truncate(time.Second)
	return u, nil
}

func (s *pgUserStore) update(id string, input userInput) (user, bool, error) {
	if err := validateUserInput(input, false); err != nil {
		return user{}, false, err
	}

	n := 1
	set := []string{}
	args := []any{}

	if input.Name != "" {
		set = append(set, fmt.Sprintf("name=$%d", n))
		args = append(args, input.Name)
		n++
	}
	if input.Email != "" {
		set = append(set, fmt.Sprintf("email=$%d", n))
		args = append(args, input.Email)
		n++
	}
	if input.Role != "" {
		set = append(set, fmt.Sprintf("role=$%d", n))
		args = append(args, input.Role)
		n++
	}
	if input.Active != nil {
		set = append(set, fmt.Sprintf("active=$%d", n))
		args = append(args, *input.Active)
		n++
	}
	set = append(set, "updated_at=now()")
	args = append(args, id)

	ctx := context.Background()
	q := fmt.Sprintf(
		`update e2e_users set %s where id=$%d::uuid
		 returning id::text, name, email, role, active, created_at, updated_at`,
		strings.Join(set, ","), n,
	)
	var u user
	err := s.pool.QueryRow(ctx, q, args...).Scan(
		&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return user{}, false, nil
	}
	if err != nil {
		return user{}, false, err
	}
	u.CreatedAt = u.CreatedAt.UTC().Truncate(time.Second)
	u.UpdatedAt = u.UpdatedAt.UTC().Truncate(time.Second)
	return u, true, nil
}

func (s *pgUserStore) delete(id string) bool {
	ctx := context.Background()
	tag, err := s.pool.Exec(ctx, `delete from e2e_users where id = $1::uuid`, id)
	if err != nil {
		return false
	}
	return tag.RowsAffected() > 0
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

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		host := envOrDefault("POSTGRES_HOST", "localhost")
		port := envOrDefault("POSTGRES_PORT", "5432")
		user := envOrDefault("POSTGRES_USER", "autotest")
		pass := envOrDefault("POSTGRES_PASSWORD", "autotest")
		dbName := envOrDefault("POSTGRES_DB", "autotest")
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbName)
	}
	pool, err := openDB(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	store := newPGUserStore(pool)
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

func registerHandlers(mux *http.ServeMux, store *pgUserStore, swagger []byte, authenticator *jwtAuthenticator, adminAuthenticator *jwtAuthenticator) {
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
			"title":       "自动化测试平台 E2E 示例 API",
			"description": "供自动化测试平台导入与执行的本地被测服务。",
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
				"description": "使用 /api/v1/auth/login 返回的 token，格式：Bearer <token>",
			},
			"AdminBearerAuth": map[string]any{
				"type":        "apiKey",
				"name":        "Authorization",
				"in":          "header",
				"description": "使用 /api/v1/admin/auth/login 返回的 token，格式：Bearer <token>",
			},
		},
	}
	return json.MarshalIndent(doc, "", "  ")
}

func swaggerTags() []map[string]string {
	return []map[string]string{
		{"name": "health", "description": "服务健康检查"},
		{"name": "auth", "description": "用户 JWT 认证"},
		{"name": "users", "description": "用户只读操作（列表、详情、当前用户信息）"},
		{"name": "admin-auth", "description": "管理员 JWT 认证"},
		{"name": "admin-users", "description": "管理员用户管理（完整 CRUD）"},
		{"name": "admin", "description": "管理员统计与审计日志"},
	}
}

func swaggerPaths() map[string]any {
	return map[string]any{
		// ── 健康检查 ──────────────────────────────────────────────────────────
		"/healthz": map[string]any{
			"get": map[string]any{
				"tags":        []string{"health"},
				"operationId": "health",
				"summary":     "健康检查",
				"responses": map[string]any{
					"200": response("服务正常", ref("#/definitions/HealthResponse")),
				},
			},
		},
		// ── 用户认证 ──────────────────────────────────────────────────────────
		"/api/v1/auth/login": map[string]any{
			"post": map[string]any{
				"tags":        []string{"auth"},
				"operationId": "login",
				"summary":     "用户登录获取 JWT",
				"parameters": []map[string]any{
					bodyParam("credentials", "登录凭据", ref("#/definitions/LoginRequest")),
				},
				"responses": map[string]any{
					"200": response("登录成功，返回 token 与用户信息", ref("#/definitions/LoginResponse")),
					"400": response("请求参数错误", ref("#/definitions/ErrorResponse")),
					"401": response("用户名或密码错误", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		// ── 用户 API（只读 + 个人信息）────────────────────────────────────────
		"/api/v1/me": map[string]any{
			"get": map[string]any{
				"tags":        []string{"users"},
				"operationId": "getMe",
				"summary":     "获取当前登录用户信息",
				"security":    jwtSecurity(),
				"responses": map[string]any{
					"200": response("当前登录用户", ref("#/definitions/AuthUser")),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		"/api/v1/users": map[string]any{
			"get": map[string]any{
				"tags":        []string{"users"},
				"operationId": "listUsers",
				"summary":     "获取用户列表",
				"security":    jwtSecurity(),
				"parameters": []map[string]any{
					{
						"name":        "role",
						"in":          "query",
						"required":    false,
						"type":        "string",
						"description": "按角色筛选，留空返回全部",
					},
				},
				"responses": map[string]any{
					"200": response("用户列表", map[string]any{
						"type":  "array",
						"items": ref("#/definitions/User"),
					}),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		"/api/v1/users/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"users"},
				"operationId": "getUser",
				"summary":     "根据 ID 获取用户详情",
				"security":    jwtSecurity(),
				"parameters":  []map[string]any{pathIDParam()},
				"responses": map[string]any{
					"200": response("用户详情", ref("#/definitions/User")),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
					"404": response("用户不存在", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		// ── 管理员认证 ────────────────────────────────────────────────────────
		"/api/v1/admin/auth/login": map[string]any{
			"post": map[string]any{
				"tags":        []string{"admin-auth"},
				"operationId": "adminLogin",
				"summary":     "管理员登录获取 JWT",
				"parameters": []map[string]any{
					bodyParam("credentials", "管理员登录凭据", ref("#/definitions/AdminLoginRequest")),
				},
				"responses": map[string]any{
					"200": response("登录成功，返回 token 与管理员信息", ref("#/definitions/LoginResponse")),
					"400": response("请求参数错误", ref("#/definitions/ErrorResponse")),
					"401": response("用户名或密码错误", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		// ── 管理员用户管理（完整 CRUD）────────────────────────────────────────
		"/api/v1/admin/users": map[string]any{
			"get": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminListUsers",
				"summary":     "管理员获取用户列表",
				"security":    adminJWTSecurity(),
				"parameters": []map[string]any{
					{
						"name":        "role",
						"in":          "query",
						"required":    false,
						"type":        "string",
						"description": "按角色筛选，留空返回全部",
					},
				},
				"responses": map[string]any{
					"200": response("用户列表及总数", ref("#/definitions/AdminUserListResponse")),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
				},
			},
			"post": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminCreateUser",
				"summary":     "管理员新建用户",
				"security":    adminJWTSecurity(),
				"parameters": []map[string]any{
					bodyParam("user", "新建用户请求体", ref("#/definitions/CreateUserRequest")),
				},
				"responses": map[string]any{
					"201": response("新建成功，返回用户详情", ref("#/definitions/User")),
					"400": response("参数校验失败", ref("#/definitions/ErrorResponse")),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		"/api/v1/admin/users/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminGetUser",
				"summary":     "管理员根据 ID 获取用户详情",
				"security":    adminJWTSecurity(),
				"parameters":  []map[string]any{pathIDParam()},
				"responses": map[string]any{
					"200": response("用户详情", ref("#/definitions/User")),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
					"404": response("用户不存在", ref("#/definitions/ErrorResponse")),
				},
			},
			"put": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminUpdateUser",
				"summary":     "管理员更新用户信息",
				"security":    adminJWTSecurity(),
				"parameters": []map[string]any{
					pathIDParam(),
					bodyParam("user", "待更新的用户字段", ref("#/definitions/UpdateUserRequest")),
				},
				"responses": map[string]any{
					"200": response("更新成功，返回最新用户详情", ref("#/definitions/User")),
					"400": response("参数校验失败", ref("#/definitions/ErrorResponse")),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
					"404": response("用户不存在", ref("#/definitions/ErrorResponse")),
				},
			},
			"delete": map[string]any{
				"tags":        []string{"admin-users"},
				"operationId": "adminDeleteUser",
				"summary":     "管理员删除用户",
				"security":    adminJWTSecurity(),
				"parameters":  []map[string]any{pathIDParam()},
				"responses": map[string]any{
					"204": map[string]string{"description": "删除成功"},
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
					"404": response("用户不存在", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		// ── 管理员统计与审计 ──────────────────────────────────────────────────
		"/api/v1/admin/stats": map[string]any{
			"get": map[string]any{
				"tags":        []string{"admin"},
				"operationId": "adminStats",
				"summary":     "获取平台统计数据",
				"security":    adminJWTSecurity(),
				"responses": map[string]any{
					"200": response("平台统计数据", ref("#/definitions/AdminStats")),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
				},
			},
		},
		"/api/v1/admin/audit-logs": map[string]any{
			"get": map[string]any{
				"tags":        []string{"admin"},
				"operationId": "adminAuditLogs",
				"summary":     "获取审计日志列表",
				"security":    adminJWTSecurity(),
				"responses": map[string]any{
					"200": response("审计日志列表", map[string]any{
						"type":  "array",
						"items": ref("#/definitions/AuditLog"),
					}),
					"401": response("未授权，token 无效或已过期", ref("#/definitions/ErrorResponse")),
				},
			},
		},
	}
}

func swaggerDefinitions() map[string]any {
	return map[string]any{
		"AuthUser": objectSchema([]string{"id", "username", "role"}, map[string]any{
			"id":       stringSchemaWithDesc("e2e-admin", "用户唯一标识"),
			"username": stringSchemaWithDesc("admin", "登录用户名"),
			"role":     stringSchemaWithDesc("admin", "角色，如 admin、tester"),
		}),
		"HealthResponse": objectSchema(nil, map[string]any{
			"status": stringSchemaWithDesc("ok", "服务状态，正常时为 ok"),
		}),
		"LoginRequest": objectSchema([]string{"username", "password"}, map[string]any{
			"username": stringSchemaWithDesc("admin", "登录用户名"),
			"password": stringSchemaWithDesc("admin123", "登录密码"),
		}),
		"AdminLoginRequest": objectSchema([]string{"username", "password"}, map[string]any{
			"username": stringSchemaWithDesc("admin-root", "管理员登录用户名"),
			"password": stringSchemaWithDesc("admin123", "管理员登录密码"),
		}),
		"LoginResponse": objectSchema([]string{"token", "user"}, map[string]any{
			"token": stringSchemaWithDesc("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", "JWT Bearer Token，请求时放入 Authorization 头"),
			"user":  ref("#/definitions/AuthUser"),
		}),
		"AdminStats": objectSchema([]string{"totalUsers", "activeUsers", "adminUsers", "generatedAt"}, map[string]any{
			"totalUsers":  integerSchemaWithDesc(2, "用户总数"),
			"activeUsers": integerSchemaWithDesc(2, "活跃用户数"),
			"adminUsers":  integerSchemaWithDesc(1, "管理员用户数"),
			"generatedAt": dateTimeSchemaWithDesc("统计数据生成时间"),
		}),
		"AdminUserListResponse": objectSchema([]string{"total", "users"}, map[string]any{
			"total": integerSchemaWithDesc(2, "符合条件的用户总数"),
			"users": map[string]any{
				"type":        "array",
				"items":       ref("#/definitions/User"),
				"description": "用户列表",
			},
		}),
		"AuditLog": objectSchema([]string{"id", "actor", "action", "resource", "createdAt"}, map[string]any{
			"id":        stringSchemaWithDesc("audit-1", "审计日志唯一标识"),
			"actor":     stringSchemaWithDesc("admin-root", "操作人用户名"),
			"action":    stringSchemaWithDesc("run.suite", "操作类型，如 import.swagger、run.suite、report.created"),
			"resource":  stringSchemaWithDesc("suite:regression", "操作对象，格式为 类型:名称"),
			"createdAt": dateTimeSchemaWithDesc("操作时间"),
		}),
		"User": objectSchema([]string{"id", "name", "email", "role", "active", "createdAt", "updatedAt"}, map[string]any{
			"id":        stringSchemaWithDesc("1", "用户唯一标识（UUID）"),
			"name":      stringSchemaWithDesc("Alice", "用户姓名"),
			"email":     stringSchemaWithDesc("alice@example.test", "用户邮箱"),
			"role":      stringSchemaWithDesc("tester", "用户角色，如 admin、tester"),
			"active":    map[string]any{"type": "boolean", "example": true, "description": "是否启用"},
			"createdAt": dateTimeSchemaWithDesc("创建时间"),
			"updatedAt": dateTimeSchemaWithDesc("最后更新时间"),
		}),
		"CreateUserRequest": objectSchema([]string{"name", "email"}, map[string]any{
			"name":   stringSchemaWithDesc("Charlie", "用户姓名，必填"),
			"email":  stringSchemaWithDesc("charlie@example.test", "用户邮箱，必填，须包含 @"),
			"role":   stringSchemaWithDesc("tester", "用户角色，留空默认为 tester"),
			"active": map[string]any{"type": "boolean", "example": true, "description": "是否启用，留空默认为 true"},
		}),
		"UpdateUserRequest": objectSchema(nil, map[string]any{
			"name":   stringSchemaWithDesc("Charlie Updated", "用户姓名，留空则不更新"),
			"email":  stringSchemaWithDesc("charlie.updated@example.test", "用户邮箱，留空则不更新"),
			"role":   stringSchemaWithDesc("admin", "用户角色，留空则不更新"),
			"active": map[string]any{"type": "boolean", "example": true, "description": "是否启用，留空则不更新"},
		}),
		"ErrorResponse": objectSchema([]string{"error"}, map[string]any{
			"error": stringSchemaWithDesc("user not found", "错误描述信息"),
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
		"description": "用户 ID（UUID）",
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

func stringSchemaWithDesc(example, description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"example":     example,
		"description": description,
	}
}

func integerSchema(example int) map[string]any {
	return map[string]any{
		"type":    "integer",
		"example": example,
	}
}

func integerSchemaWithDesc(example int, description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"example":     example,
		"description": description,
	}
}

func dateTimeSchema() map[string]any {
	return map[string]any{
		"type":    "string",
		"format":  "date-time",
		"example": "2026-01-01T00:00:00Z",
	}
}

func dateTimeSchemaWithDesc(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"format":      "date-time",
		"example":     "2026-01-01T00:00:00Z",
		"description": description,
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

func openDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 5
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
