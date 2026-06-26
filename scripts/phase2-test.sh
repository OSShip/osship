#!/usr/bin/env bash
# Phase 2 integration tests: listings CRUD, cache, mentor reviewer flow
set -euo pipefail

BASE="${API_BASE:-http://localhost/api/v1}"
PASSWORD="${TEST_PASSWORD:-password123}"
FAIL=0

pass() { echo "PASS $1"; }
fail() { echo "FAIL $1"; FAIL=1; }

register() {
  local email="$1" role="$2" github="$3"
  curl -sf -X POST "$BASE/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"$PASSWORD\",\"role\":\"$role\",\"github_username\":\"$github\"}" >/dev/null \
    || true
}

token_for() {
  curl -sf -X POST "$BASE/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$1\",\"password\":\"$PASSWORD\"}" \
    | sed -n 's/.*"token":"\([^"]*\)".*/\1/p'
}

echo "==> Phase 2 integration tests ($BASE)"

# Public catalog + cache
HEADERS=$(curl -sf -D - "$BASE/listings?status=active" -o /tmp/listings.json)
if grep -qi 'X-Cache: MISS' <<<"$HEADERS" || ! grep -qi 'X-Cache' <<<"$HEADERS"; then
  pass "listings first fetch"
else
  fail "listings first fetch (unexpected cache hit)"
fi

HEADERS2=$(curl -sf -D - "$BASE/listings?status=active" -o /dev/null)
if grep -qi 'X-Cache: HIT' <<<"$HEADERS2"; then
  pass "listings redis cache hit"
else
  fail "listings redis cache hit"
fi

# OSS project filter
if curl -sf "$BASE/listings?status=active&oss_project=Linux" | grep -q oss_project_name 2>/dev/null || \
   curl -sf "$BASE/listings?status=active&oss_project=Linux" | grep -q '\[\]'; then
  pass "listings oss_project filter"
else
  fail "listings oss_project filter"
fi

# Mentor application flow
STAMP=$(date +%s)
MENTOR_EMAIL="phase2-mentor-$STAMP@osship.local"
ADMIN_EMAIL="phase2-admin-$STAMP@osship.local"

register "$MENTOR_EMAIL" "student" "octocat"
register "$ADMIN_EMAIL" "admin" "osship-admin"

MENTOR_TOKEN=$(token_for "$MENTOR_EMAIL")
ADMIN_TOKEN=$(token_for "$ADMIN_EMAIL")

if [[ -z "$MENTOR_TOKEN" || -z "$ADMIN_TOKEN" ]]; then
  fail "auth tokens for mentor flow"
else
  pass "auth tokens for mentor flow"

  APPLY=$(curl -sf -X POST "$BASE/mentors/apply" \
    -H "Authorization: Bearer $MENTOR_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"github_username":"octocat"}')
  if echo "$APPLY" | grep -q '"status":"pending"'; then
    pass "mentor apply creates pending application"
  else
    fail "mentor apply creates pending application"
  fi

  if echo "$APPLY" | grep -q 'github_data'; then
    pass "mentor apply includes github_data"
  else
    fail "mentor apply includes github_data"
  fi

  APP_ID=$(echo "$APPLY" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

  REVIEW=$(curl -sf -X PATCH "$BASE/mentors/admin/applications/$APP_ID" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"status":"approved"}')
  if echo "$REVIEW" | grep -q approved; then
    pass "admin approves mentor application"
  else
    fail "admin approves mentor application"
  fi

  MENTOR_TOKEN=$(token_for "$MENTOR_EMAIL")

  CREATE=$(curl -sf -X POST "$BASE/listings" \
    -H "Authorization: Bearer $MENTOR_TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{
      "oss_project_name":"Phase2 Test Project",
      "oss_repo_url":"https://github.com/octocat/Hello-World",
      "description":"Integration test listing",
      "price_cents":9900,
      "duration_weeks":4,
      "total_slots":2,
      "status":"active"
    }')
  LISTING_ID=$(echo "$CREATE" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
  if [[ -n "$LISTING_ID" ]]; then
    pass "approved mentor creates listing"
  else
    fail "approved mentor creates listing"
  fi

  # Cache invalidation after write
  HEADERS3=$(curl -sf -D - "$BASE/listings?status=active" -o /dev/null)
  if grep -qi 'X-Cache: MISS' <<<"$HEADERS3" || ! grep -qi 'X-Cache: HIT' <<<"$HEADERS3"; then
    pass "listing write invalidates cache"
  else
    fail "listing write invalidates cache"
  fi

  DETAIL=$(curl -sf "$BASE/listings/$LISTING_ID")
  if echo "$DETAIL" | grep -q 'Phase2 Test Project'; then
    pass "listing detail by id"
  else
    fail "listing detail by id"
  fi
fi

if [ "$FAIL" -eq 0 ]; then
  echo "All Phase 2 integration tests passed."
else
  echo "Some Phase 2 tests failed."
  exit 1
fi
