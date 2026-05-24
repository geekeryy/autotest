#!/usr/bin/env bash
# Build DATABASE_URL and psql PG* exports from POSTGRES_* (Makefile / compose tooling).
# Usage: eval "$(./scripts/build-database-url.sh)"
#        eval "$(./scripts/build-database-url.sh compose)"  # POSTGRES_HOST=postgres

set -euo pipefail

POSTGRES_USER="${POSTGRES_USER:-autotest}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-autotest}"
POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_DB="${POSTGRES_DB:-autotest}"
POSTGRES_SSLMODE="${POSTGRES_SSLMODE:-disable}"

if [ "${1:-}" = "compose" ]; then
  POSTGRES_HOST=postgres
fi

python3 - "$POSTGRES_USER" "$POSTGRES_PASSWORD" "$POSTGRES_HOST" "$POSTGRES_PORT" "$POSTGRES_DB" "$POSTGRES_SSLMODE" <<'PY'
import sys
from urllib.parse import quote, urlunparse

user, password, host, port, db, sslmode = sys.argv[1:7]

def sh_escape(s: str) -> str:
    return s.replace("'", "'\"'\"'")

def emit(name, value):
    print(f"export {name}='{sh_escape(str(value))}'")

user_q = quote(user, safe="")
pass_q = quote(password, safe="")
netloc = f"{user_q}:{pass_q}@{host}:{port}"
query = f"sslmode={quote(sslmode, safe='')}" if sslmode else ""
database_url = urlunparse(("postgres", netloc, f"/{db}", "", query, ""))

emit("POSTGRES_USER", user)
emit("POSTGRES_PASSWORD", password)
emit("POSTGRES_HOST", host)
emit("POSTGRES_PORT", port)
emit("POSTGRES_DB", db)
emit("POSTGRES_SSLMODE", sslmode)
emit("DATABASE_URL", database_url)
emit("PGUSER", user)
emit("PGPASSWORD", password)
emit("PGHOST", host)
emit("PGPORT", port)
emit("PGDATABASE", db)
PY
