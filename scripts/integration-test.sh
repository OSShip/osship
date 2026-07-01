#!/usr/bin/env bash
set -euo pipefail

BASE="${1:-http://localhost}"
API="$BASE/api/v1"
FAIL=0

fail() {
  echo "FAIL $1"
  FAIL=1
}

pass() {
  echo "OK  $1"
}

echo "==> OSShip Phase 1 integration tests"

EMAIL="test-$(date +%s)@osship.test"
PASSWORD="secret123"

REGISTER=$(curl -sf -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\",\"role\":\"student\",\"display_name\":\"Test User\"}") || { fail "register"; REGISTER=""; }

TOKEN=$(echo "$REGISTER" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
USER_ID=$(echo "$REGISTER" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

if [ -n "$TOKEN" ] && [ -n "$USER_ID" ]; then
  pass "register via gateway"
else
  fail "register via gateway"
fi

LOGIN=$(curl -sf -X POST "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}") || { fail "login"; LOGIN=""; }

LOGIN_TOKEN=$(echo "$LOGIN" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
if [ -n "$LOGIN_TOKEN" ]; then
  pass "login via gateway"
else
  fail "login via gateway"
fi

ME=$(curl -sf "$API/auth/me" -H "Authorization: Bearer $LOGIN_TOKEN") || { fail "auth/me"; ME=""; }
if echo "$ME" | grep -q "$EMAIL"; then
  pass "auth/me protected route"
else
  fail "auth/me protected route"
fi

UNAUTH=$(curl -s -o /dev/null -w '%{http_code}' "$API/auth/me")
if [ "$UNAUTH" = "401" ]; then
  pass "auth/me rejects missing token"
else
  fail "auth/me rejects missing token (got $UNAUTH)"
fi

PROFILE=$(curl -sf "$API/users/$USER_ID/profile") || { fail "users profile"; PROFILE=""; }
if echo "$PROFILE" | grep -q "Test User"; then
  pass "users profile via gateway"
else
  fail "users profile via gateway"
fi

PATCH=$(curl -sf -X PATCH "$API/users/me" \
  -H "Authorization: Bearer $LOGIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"bio":"integration test bio"}') || { fail "users patch me"; PATCH=""; }
if echo "$PATCH" | grep -q "integration test bio"; then
  pass "users patch me via gateway"
else
  fail "users patch me via gateway"
fi

OAUTH_STUB=$(curl -sf -H "Accept: application/json" "$API/auth/oauth/github") || { fail "github oauth stub"; OAUTH_STUB=""; }
if echo "$OAUTH_STUB" | grep -q '"stub":true'; then
  pass "github oauth stub"
else
  fail "github oauth stub"
fi

RATE_LIMIT_HIT=0
for i in $(seq 1 12); do
  CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"rl-$i-$(date +%s)@osship.test\",\"password\":\"x\"}")
  if [ "$CODE" = "429" ]; then
    RATE_LIMIT_HIT=1
    break
  fi
done
if [ "$RATE_LIMIT_HIT" -eq 1 ]; then
  pass "rate limit returns 429"
else
  fail "rate limit returns 429"
fi

if [ "$FAIL" -eq 0 ]; then
  echo "All Phase 1 integration tests passed."
else
  echo "Some integration tests failed."
  exit 1
fi

echo ""
./scripts/phase2-test.sh "$BASE"
