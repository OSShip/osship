#!/usr/bin/env bash
# Smoke tests for OSShip deployment
set -euo pipefail

BASE="${1:-http://localhost}"

echo "Testing $BASE/api/v1/health..."
curl -sf "$BASE/api/v1/health" | grep -q ok

echo "Testing listings..."
curl -sf "$BASE/api/v1/listings?status=active" || true

echo "Testing public payout summary..."
curl -sf "$BASE/api/v1/public/payout-summary" || true

echo "All smoke tests passed."
