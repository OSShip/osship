#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! docker compose ps postgres --status running -q 2>/dev/null | grep -q .; then
  echo "Postgres is not running. Start the stack with: make up"
  exit 1
fi

run_sql() {
  local file="$1"
  echo "==> Applying $(basename "$file")..."
  if docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U osship -d osship < "$file"; then
    echo "    applied"
  else
    echo "    skipped (likely already applied on first boot)"
  fi
}

run_sql "$ROOT/migrations/init/001_schemas.sql"
run_sql "$ROOT/migrations/general/001_init.up.sql"
run_sql "$ROOT/migrations/general/002_stripe_connect.up.sql"
run_sql "$ROOT/migrations/general/003_password_salt.up.sql"
run_sql "$ROOT/migrations/general/004_session_is_active.up.sql"
run_sql "$ROOT/migrations/payments/001_init.up.sql"
run_sql "$ROOT/migrations/payments/002_outbox.up.sql"
run_sql "$ROOT/migrations/metrics/001_init.up.sql"

echo "Migration step complete."
