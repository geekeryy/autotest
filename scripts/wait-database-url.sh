#!/usr/bin/env bash
# Wait until PostgreSQL is reachable (uses psql with PG* from POSTGRES_*).
set -euo pipefail

label="${1:-PostgreSQL}"

: "${PGHOST:?POSTGRES_HOST is required}"
: "${PGPORT:?POSTGRES_PORT is required}"
: "${PGUSER:?POSTGRES_USER is required}"
: "${PGPASSWORD:?POSTGRES_PASSWORD is required}"
: "${PGDATABASE:?POSTGRES_DB is required}"

echo "等待 ${label} ${PGUSER}@${PGHOST}:${PGPORT}/${PGDATABASE} 就绪..."
for i in $(seq 1 15); do
  if psql -c 'SELECT 1' >/dev/null 2>&1; then
    echo "${label} 已就绪。"
    exit 0
  fi
  sleep 2
done

echo "${label} 未在预期时间内就绪：${PGUSER}@${PGHOST}:${PGPORT}/${PGDATABASE}" >&2
exit 1
