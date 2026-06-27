#!/usr/bin/env bash
# Smoke tests for OSShip deployment
set -euo pipefail

BASE="${1:-http://localhost}"

echo "Testing $BASE/api/v1/health..."
curl -sf "$BASE/api/v1/health" | grep -q ok

echo "Testing listings catalog..."
curl -sf "$BASE/api/v1/listings?status=active" | grep -q '\['

echo "Testing listings cache..."
curl -sf -D - "$BASE/api/v1/listings?status=active" -o /dev/null | grep -qi 'X-Cache'

echo "Testing public payout summary..."
curl -sf "$BASE/api/v1/public/payout-summary" | grep -q transaction_count

if docker compose ps gateway --status running -q 2>/dev/null | grep -q .; then
  echo "Testing internal service health endpoints..."
  for port in 8081 8082 8083 8084 8085 8086 8087 8088; do
    docker compose exec -T gateway wget -q -O - "http://localhost:$port/health" 2>/dev/null | grep -q ok || true
  done
fi

if docker compose ps prometheus --status running -q 2>/dev/null | grep -q .; then
  echo "Testing Prometheus targets..."
  curl -sf "http://localhost:9090/api/v1/query?query=up" | grep -q '"status":"success"' || \
    docker compose exec -T prometheus wget -q -O - 'http://localhost:9090/api/v1/query?query=up' | grep -q '"status":"success"'
fi

echo "All smoke tests passed."
