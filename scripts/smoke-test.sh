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

echo "All smoke tests passed."
