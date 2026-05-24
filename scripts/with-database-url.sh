#!/usr/bin/env bash
# Load .env, normalize POSTGRES_*, derive DATABASE_URL / PG*, then run a command.
#
#   ./scripts/with-database-url.sh -- make migrate
#   ./scripts/with-database-url.sh compose -- docker compose ... up -d
#
# compose 模式会将 POSTGRES_HOST 设为 postgres（供 compose 网络内 API 使用）。

set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

compose_mode=""
if [ "${1:-}" = "compose" ]; then
  compose_mode=compose
  shift
fi

eval "$(./scripts/build-database-url.sh "$compose_mode")"

if [ "${1:-}" = "--" ]; then
  shift
fi

exec "$@"
