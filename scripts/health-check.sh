#!/usr/bin/env bash
set -euo pipefail

BASE="${1:-http://localhost}"
FAIL=0

check() {
  local name="$1"
  local cmd="$2"
  if eval "$cmd"; then
    echo "OK  $name"
  else
    echo "FAIL $name"
    FAIL=1
  fi
}

echo "==> OSShip Phase 0 health checks"

check "nginx" "curl -sf '$BASE/' | grep -q OSShip"
check "gateway via nginx" "curl -sf '$BASE/api/' | grep -q gateway-placeholder"
check "grafana via nginx" "curl -sf -o /dev/null -w '%{http_code}' '$BASE/grafana/' | grep -qE '200|301|302'"
check "postgres schemas" "docker compose exec -T postgres psql -U osship -d osship -tAc \"SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name IN ('general','payments','metrics')\" | grep -q 3"
check "redis" "docker compose exec -T redis redis-cli ping | grep -q PONG"
check "kafka" "docker compose exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list >/dev/null"
check "prometheus" "docker compose exec -T prometheus wget -q -O- http://localhost:9090/-/healthy | grep -q Prometheus"

if [ "$FAIL" -eq 0 ]; then
  echo "All Phase 0 health checks passed."
else
  echo "Some checks failed."
  exit 1
fi
