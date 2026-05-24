#!/bin/sh
set -eu

POSTGRES_USER="${POSTGRES_USER:-autotest}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-autotest}"
POSTGRES_HOST="${POSTGRES_HOST:-postgres}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_DB="${POSTGRES_DB:-autotest}"

export PGUSER="$POSTGRES_USER"
export PGPASSWORD="$POSTGRES_PASSWORD"
export PGHOST="$POSTGRES_HOST"
export PGPORT="$POSTGRES_PORT"
export PGDATABASE="$POSTGRES_DB"

wait_for_db() {
  echo "等待 PostgreSQL 就绪..."
  i=1
  last_err=""
  while [ "$i" -le 30 ]; do
    last_err=$(psql -c 'SELECT 1' 2>&1) && {
      echo "PostgreSQL 已就绪。"
      return 0
    }
    sleep 2
    i=$((i + 1))
  done
  echo "PostgreSQL 连接失败：${PGUSER}@${PGHOST}:${PGPORT}/${PGDATABASE}" >&2
  echo "最近错误：$last_err" >&2
  echo "若刚修改过 POSTGRES_USER/POSTGRES_PASSWORD，请执行：docker compose down -v && docker compose up -d" >&2
  exit 1
}

run_migrations() {
  for file in /app/migrations/*.sql; do
    [ -f "$file" ] || continue
    echo "执行迁移 $file"
    if grep -qE '^--[[:space:]]+\+goose[[:space:]]+Up' "$file"; then
      sql=$(awk '/^--[[:space:]]+\+goose[[:space:]]+Up[[:space:]]*$/{in_up=1; next} /^--[[:space:]]+\+goose[[:space:]]+Down[[:space:]]*$/{in_up=0} in_up {print}' "$file")
    else
      echo "警告: $file 缺少 -- +goose Up 标记，将执行完整文件"
      sql=$(cat "$file")
    fi
    if [ -z "$(printf '%s' "$sql" | tr -d '[:space:]')" ]; then
      echo "错误: $file 未产生可执行 SQL（请检查 -- +goose Up 段）" >&2
      exit 1
    fi
    printf '%s\n' "$sql" | psql -v ON_ERROR_STOP=1
  done
}

wait_for_db
run_migrations
exec "$@"
