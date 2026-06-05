#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
DATA_DIR="${DATA_DIR:-$ROOT_DIR/.wsl-data}"
export PATH="/usr/local/go/bin:$PATH"
export GOPATH="${GOPATH:-/tmp/quotanet-go/gopath}"
export GOCACHE="${GOCACHE:-/tmp/quotanet-go/gocache}"

SERVER_HOST="${SERVER_HOST:-127.0.0.1}"
SERVER_PORT="${SERVER_PORT:-8080}"
DATABASE_HOST="${DATABASE_HOST:-127.0.0.1}"
DATABASE_PORT="${DATABASE_PORT:-5432}"
DATABASE_USER="${DATABASE_USER:-sub2api}"
DATABASE_PASSWORD="${DATABASE_PASSWORD:-sub2api}"
DATABASE_DBNAME="${DATABASE_DBNAME:-sub2api}"
DATABASE_SSLMODE="${DATABASE_SSLMODE:-disable}"
REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
REDIS_DB="${REDIS_DB:-0}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@sub2api.local}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123456}"
JWT_SECRET="${JWT_SECRET:-quotanet-local-dev-jwt-secret-please-change-32}"
TOTP_ENCRYPTION_KEY="${TOTP_ENCRYPTION_KEY:-00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff}"

require_cmd() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "Missing required command: $name" >&2
    exit 1
  fi
}

check_deps() {
  mkdir -p "$GOPATH" "$GOCACHE"
  require_cmd go
  require_cmd node
  if ! command -v pnpm >/dev/null 2>&1; then
    require_cmd corepack
    corepack enable
    corepack prepare pnpm@latest --activate
  fi
  require_cmd pnpm
  require_cmd psql
  require_cmd redis-server
  require_cmd redis-cli
}

ensure_redis() {
  if redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping >/dev/null 2>&1; then
    echo "Redis is already running at $REDIS_HOST:$REDIS_PORT"
    return
  fi

  echo "Starting local Redis at $REDIS_HOST:$REDIS_PORT"
  redis-server --daemonize yes --bind "$REDIS_HOST" --port "$REDIS_PORT"
}

check_postgres() {
  echo "Checking PostgreSQL at $DATABASE_HOST:$DATABASE_PORT database=$DATABASE_DBNAME user=$DATABASE_USER"
  PGPASSWORD="$DATABASE_PASSWORD" psql \
    -h "$DATABASE_HOST" \
    -p "$DATABASE_PORT" \
    -U "$DATABASE_USER" \
    -d postgres \
    -v ON_ERROR_STOP=1 \
    -c "SELECT 1;" >/dev/null
}

install_frontend() {
  echo "Installing frontend dependencies"
  (cd "$FRONTEND_DIR" && pnpm install --frozen-lockfile)
}

test_backend_protocol() {
  echo "Running QuotaNet protocol package tests"
  (cd "$BACKEND_DIR" && go test ./internal/quotanet/protocol)
}

run_backend() {
  mkdir -p "$DATA_DIR"
  echo "Starting backend at http://$SERVER_HOST:$SERVER_PORT"
  echo "Data dir: $DATA_DIR"
  cd "$BACKEND_DIR"
  DATA_DIR="$DATA_DIR" \
  AUTO_SETUP=true \
  TZ="${TZ:-Asia/Shanghai}" \
  SERVER_HOST="$SERVER_HOST" \
  SERVER_PORT="$SERVER_PORT" \
  SERVER_MODE="${SERVER_MODE:-debug}" \
  RUN_MODE="${RUN_MODE:-standard}" \
  DATABASE_HOST="$DATABASE_HOST" \
  DATABASE_PORT="$DATABASE_PORT" \
  DATABASE_USER="$DATABASE_USER" \
  DATABASE_PASSWORD="$DATABASE_PASSWORD" \
  DATABASE_DBNAME="$DATABASE_DBNAME" \
  DATABASE_SSLMODE="$DATABASE_SSLMODE" \
  REDIS_HOST="$REDIS_HOST" \
  REDIS_PORT="$REDIS_PORT" \
  REDIS_PASSWORD="$REDIS_PASSWORD" \
  REDIS_DB="$REDIS_DB" \
  ADMIN_EMAIL="$ADMIN_EMAIL" \
  ADMIN_PASSWORD="$ADMIN_PASSWORD" \
  JWT_SECRET="$JWT_SECRET" \
  TOTP_ENCRYPTION_KEY="$TOTP_ENCRYPTION_KEY" \
  go run ./cmd/server
}

usage() {
  cat <<'EOF'
Usage:
  deploy/wsl-dev-run.sh check
  deploy/wsl-dev-run.sh frontend
  deploy/wsl-dev-run.sh test-protocol
  deploy/wsl-dev-run.sh backend
  deploy/wsl-dev-run.sh all

Environment overrides:
  DATABASE_HOST=127.0.0.1 DATABASE_USER=sub2api DATABASE_PASSWORD=sub2api DATABASE_DBNAME=sub2api
  REDIS_HOST=127.0.0.1 REDIS_PORT=6379
  SERVER_HOST=127.0.0.1 SERVER_PORT=8080
  DATA_DIR=/path/to/data

The script expects Go, psql, redis-server, redis-cli, Node.js and pnpm to be available inside WSL.
EOF
}

cmd="${1:-all}"
case "$cmd" in
  check)
    check_deps
    ensure_redis
    check_postgres
    ;;
  frontend)
    check_deps
    install_frontend
    ;;
  test-protocol)
    check_deps
    test_backend_protocol
    ;;
  backend)
    check_deps
    ensure_redis
    check_postgres
    run_backend
    ;;
  all)
    check_deps
    ensure_redis
    check_postgres
    install_frontend
    test_backend_protocol
    run_backend
    ;;
  *)
    usage
    exit 1
    ;;
esac
