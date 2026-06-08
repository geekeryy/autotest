# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Development

```bash
# One-time DB setup (fresh environment)
sudo pg_ctlcluster 16 main start
sudo -u postgres psql -c "CREATE USER autotest WITH PASSWORD 'autotest' CREATEDB;"
sudo -u postgres psql -c "CREATE DATABASE autotest OWNER autotest;"
cp .env.example .env
make migrate

# Start services (each blocks; use separate terminals)
make run-api        # Go API on :8080
make web-dev        # Vue dev server on :5173 (proxies /api → :8080)
```

Default login: `admin` / `admin`.

### Testing & Linting

```bash
make test                          # Unit tests (APP_ENV=test)
make test-integration              # Integration tests (requires PostgreSQL)
go test ./internal/spec/... -v     # Single package, verbose
go test ./internal/spec/... -run TestFunctionName  # Single test
go vet ./...                       # Go linting (no golangci-lint config)
npm --prefix web/admin run build:dev  # Frontend build check (no ESLint/Prettier)
```

### Database

```bash
make migrate        # Apply all migrations/*.sql (requires psql CLI)
```

### Build & Deploy

```bash
make build-mcp              # Build MCP server binary → bin/autotest-mcp
make docker-build           # Multi-arch Docker image
make all-in-one-up          # Single Docker image with embedded frontend
```

## Architecture

**Autotest** is an API automation test platform: import OpenAPI specs → auto-generate test cases → orchestrate multi-step scenarios → run with assertions → optionally use AI assistance throughout.

### Stack

- **Go 1.25** backend (`cmd/api`) — chi router, raw SQL via pgx, no ORM
- **Vue 3 + TypeScript** frontend (`web/admin/`) — Element Plus, Pinia, Vite
- **PostgreSQL 14+** — sole data store; migrations are plain SQL in `migrations/`
- **MCP server** (`cmd/mcp`) — exposes `import_swagger` / `import_swagger_from_url` tools for Cursor/Claude

### Backend Package Layout (`internal/`)

The ~52 packages fall into four concerns:

**Core test platform:**
- `spec/` — OpenAPI import, versioning, endpoint normalization
- `case/` — Test case templates (auto-generated from endpoints or manual)
- `scenario/` — Multi-step workflow definitions (steps: API, SQL, Script, For, Condition)
- `runner/` — Execution engine: parameter interpolation (`{{$steps.X.resp.Y}}`), assertions, chaining
- `report/` — Test run results and history
- `mockserver/`, `mockset/`, `mockrecord/`, `mockgen/` — HTTP mock rules, recording/replay, dynamic templates

**AI subsystem:**
- `aiprovider/` — Multi-LLM abstraction (Anthropic, OpenAI, DeepSeek, Ollama) with semantic routing
- `aitools/` — 60+ built-in tools the assistant can call (read + mutating); tool registry in `aitools/builtin/`
- `aisession/` — Persistent conversation storage, SSE streaming
- `aiassert/` — Three-tier assertion inference: schema rules → history → semantic LLM
- `aifactory/` — Semantic test data generation
- `ainl/` — Natural language → scenario step orchestration
- `genagent/` — Async scenario generation with validation & repair loop
- `scenariogen/` — Full-coverage scenario scaffolding from OpenAPI
- `testprofile/` — Auto-learned project characteristics (response conventions, field enums, endpoint dependencies)

**Auth & platform:**
- `auth/`, `authprovider/` — JWT, RBAC roles, OAuth (GitHub)
- `apikey/`, `auditlog/`, `notification/` — API keys, audit trail, notifications

**Shared utilities:**
- `httpx/` — HTTP client wrapper with retry/timeout
- `templating/` — `{{$mock/ds/steps/env}}` template evaluation
- `assertion/` — Expression evaluator for response assertions
- `paramsource/`, `testdata/` — Parameter and test data sources
- `generator/` — Happy-path request generation from OpenAPI schemas

### Request Flow

```
HTTP request → chi router → JWT middleware → RBAC check → handler
handler → domain service → store (pgx) → PostgreSQL
```

AI assistant requests additionally go through: `aisession` → `aiprovider` router → LLM → `aitools` dispatch → domain services.

### Configuration

Config is loaded from environment variables, then `.env` file (dev/test only). Key vars:

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` | `development` | `development` / `test` / `production` |
| `ADDR` | `:8080` | API listen address |
| `POSTGRES_*` | `autotest/autotest/localhost/5432` | DB connection |
| `JWT_SECRET` | `autotest-dev-secret-change-me` | **Must change in production** |
| `CORS_ALLOWED_ORIGINS` | (empty) | Comma-separated list |
| `AI_MAX_TOOL_HOPS` | `6` | AI tool call loop limit |

Production startup fails if `JWT_SECRET` or `POSTGRES_PASSWORD` match dev defaults.

### Frontend (`web/admin/`)

- `src/api/` — Axios wrappers per domain (mirror backend routes)
- `src/views/` — Page components
- `src/stores/` — Pinia stores
- `src/components/` — Reusable components (includes CodeMirror editor wrappers)

Vite dev server proxies `/api` → `localhost:8080`. If port 8080 is occupied by an IDE tunnel, set `VITE_DEV_API_PROXY=http://<host-ip>:8080` in `web/admin/.env.development`.

### Caveats

- `make migrate` uses `psql` directly — `postgresql-client` must be installed.
- `scripts/build-database-url.sh` uses `python3` for URL-encoding credentials.
- The default `admin` user is seeded by `migrations/001_schema.sql` — no separate seed step.
- No ESLint/Prettier in frontend; no golangci-lint config for backend.
- Design documentation in `docs/design/` covers architecture, AI capabilities, scenario orchestration, mock system, and RBAC in detail.
