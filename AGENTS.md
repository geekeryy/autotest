# AGENTS.md

## Cursor Cloud specific instructions

### Services overview

| Service | Command | Port | Notes |
|---------|---------|------|-------|
| Go API | `make run-api` | 8080 | Reads `.env` automatically in development mode |
| Vue frontend (dev) | `make web-dev` | 5173 | Vite proxies `/api` → `localhost:8080` |
| PostgreSQL | system service | 5432 | Must be running before API starts |

### Database setup (one-time per fresh VM)

```bash
sudo pg_ctlcluster 16 main start
sudo -u postgres psql -c "CREATE USER autotest WITH PASSWORD 'autotest' CREATEDB;"
sudo -u postgres psql -c "CREATE DATABASE autotest OWNER autotest;"
cp .env.example .env
make migrate
```

### Running the development stack

1. Ensure PostgreSQL is running: `sudo pg_ctlcluster 16 main start`
2. Start API: `make run-api` (blocks; use a separate terminal/tmux)
3. Start frontend: `make web-dev` (blocks; use a separate terminal/tmux)
4. Access UI at `http://localhost:5173`

Default login: `admin` / `admin` (must change password on first login).

### Tests and linting

- Unit tests: `make test` (sets `APP_ENV=test` internally)
- Integration tests: `make test-integration` (requires running PostgreSQL with `autotest` or `autotest_test` DB)
- Go lint: `go vet ./...` (no golangci-lint config in repo)
- Frontend build check: `npm --prefix web/admin run build:dev`

### Non-obvious caveats

- **Cursor 占用 8080**：若 `localhost:8080` 被 IDE 端口转发占用，Vite 代理会 `socket hang up`、登录失败。`make web-dev` 会优先把 `/api` 代理到本机局域网 IP（见控制台 `[vite] API proxy target`）；也可在 `web/admin/.env.development` 设置 `VITE_DEV_API_PROXY=http://<本机IP>:8080`。
- The `make migrate` target uses `psql` directly (not a Go migration library), so `postgresql-client` must be installed.
- The `scripts/build-database-url.sh` helper uses `python3` to URL-encode credentials — Python 3 must be available.
- The API auto-creates the default admin user on first DB migration via SQL INSERT in `migrations/001_schema.sql`; no separate seed step is needed.
- Frontend Vite dev server binds `0.0.0.0` by default (see `web/admin/package.json` dev script).
- There is no ESLint or Prettier configuration in the frontend; no dedicated frontend lint step.
