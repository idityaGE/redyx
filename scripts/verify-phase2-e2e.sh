#!/usr/bin/env bash
#
# Phase 2 End-to-End Verification Script
# Tests the complete auth → user → community flow through Envoy (port 8080)
#
# Prerequisites:
#   docker compose up -d --build
#   Wait for all services to be healthy: docker compose ps
#
# Usage:
#   ./scripts/verify-phase2-e2e.sh
#
set -euo pipefail

ENVOY_URL="${ENVOY_URL:-http://localhost:8080}"
# Keep generated names short to fit validation limits (username max 20, community max 21)
EPOCH_SUFFIX=$(date +%s | tail -c 7)
TEST_EMAIL="e2e_${EPOCH_SUFFIX}@test.com"
TEST_USERNAME="e2e${EPOCH_SUFFIX}"
TEST_PASSWORD="TestPassword123!"
PASSED=0
FAILED=0
TOTAL=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_test() {
  TOTAL=$((TOTAL + 1))
  printf "${CYAN}[TEST %02d]${NC} %s ... " "$TOTAL" "$1"
}

pass() {
  PASSED=$((PASSED + 1))
  printf "${GREEN}PASS${NC}\n"
}

fail() {
  FAILED=$((FAILED + 1))
  printf "${RED}FAIL${NC} — %s\n" "$1"
}

separator() {
  echo ""
  printf "${YELLOW}═══ %s ═══${NC}\n" "$1"
  echo ""
}

# Helper to extract JSON field using python3
json_field() {
  python3 -c "import sys,json; data=json.load(sys.stdin); print(data.get('$1',''))" 2>/dev/null
}

# ─────────────────────────────────────────────────────────────
# 0. Service Health Check
# ─────────────────────────────────────────────────────────────
separator "SERVICE HEALTH"

log_test "Docker Compose services are running"
SERVICES_UP=$(docker compose ps --format "{{.Name}}" 2>/dev/null | wc -l)
if [ "$SERVICES_UP" -ge 6 ]; then
  pass
else
  fail "Expected >= 6 services, got $SERVICES_UP"
  echo "Run: docker compose up -d --build"
  exit 1
fi

log_test "Envoy proxy responds on $ENVOY_URL"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$ENVOY_URL/api/v1/health" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" != "000" ]; then
  pass
else
  fail "Envoy not responding (HTTP $HTTP_CODE)"
  exit 1
fi

# ─────────────────────────────────────────────────────────────
# 1. Auth Flow: Register → OTP → Login
# ─────────────────────────────────────────────────────────────
separator "AUTH FLOW"

log_test "Register new user ($TEST_USERNAME)"
REGISTER_RESP=$(curl -s -X POST "$ENVOY_URL/api/v1/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$TEST_EMAIL\",\"username\":\"$TEST_USERNAME\",\"password\":\"$TEST_PASSWORD\"}" 2>/dev/null)
REGISTER_OK=$(echo "$REGISTER_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('userId') or d.get('requiresVerification') else 'no')" 2>/dev/null || echo "no")
if [ "$REGISTER_OK" = "yes" ]; then
  pass
else
  fail "Unexpected response: $REGISTER_RESP"
fi

log_test "Extract OTP from auth-service logs"
sleep 1
OTP_CODE=$(docker compose logs auth-service --tail=30 2>/dev/null | grep -oP '"code":"(\d{6})"' | tail -1 | grep -oP '\d{6}' || echo "")
if [ -z "$OTP_CODE" ]; then
  # Fallback: look for "OTP code generated" line
  OTP_CODE=$(docker compose logs auth-service --tail=30 2>/dev/null | grep -i "otp" | grep -oP '\d{6}' | tail -1 || echo "")
fi
if [ -n "$OTP_CODE" ]; then
  pass
  echo "         OTP: $OTP_CODE"
else
  fail "Could not find OTP in logs — check: docker compose logs auth-service --tail=30"
  OTP_CODE="000000"
fi

log_test "Verify OTP"
VERIFY_RESP=$(curl -s -X POST "$ENVOY_URL/api/v1/auth/verify-otp" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$TEST_EMAIL\",\"code\":\"$OTP_CODE\"}" 2>/dev/null)
VERIFY_OK=$(echo "$VERIFY_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('verified') or d.get('accessToken') else 'no')" 2>/dev/null || echo "no")
if [ "$VERIFY_OK" = "yes" ]; then
  pass
else
  fail "OTP verification failed: $VERIFY_RESP"
fi

log_test "Login and get access token"
LOGIN_RESP=$(curl -s -X POST "$ENVOY_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}" 2>/dev/null)
ACCESS_TOKEN=$(echo "$LOGIN_RESP" | json_field "accessToken")
if [ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "" ]; then
  pass
  echo "         Token: ${ACCESS_TOKEN:0:20}..."
else
  fail "No access token in response: $LOGIN_RESP"
  ACCESS_TOKEN="invalid"
fi

# ─────────────────────────────────────────────────────────────
# 2. User Profile
# ─────────────────────────────────────────────────────────────
separator "USER PROFILE"

log_test "Update own profile (display name + bio)"
UPDATE_RESP=$(curl -s -X PATCH "$ENVOY_URL/api/v1/users/me" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d '{"displayName":"E2E Test User","bio":"Automated test bio"}' 2>/dev/null)
UPDATE_OK=$(echo "$UPDATE_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('user',{}).get('displayName') else 'no')" 2>/dev/null || echo "no")
if [ "$UPDATE_OK" = "yes" ]; then
  pass
else
  fail "Profile update failed: $UPDATE_RESP"
fi

log_test "Get public profile for $TEST_USERNAME"
PROFILE_RESP=$(curl -s "$ENVOY_URL/api/v1/users/$TEST_USERNAME" 2>/dev/null)
PROFILE_OK=$(echo "$PROFILE_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('user',{}).get('username') else 'no')" 2>/dev/null || echo "no")
if [ "$PROFILE_OK" = "yes" ]; then
  pass
else
  fail "Profile not found: $PROFILE_RESP"
fi

# ─────────────────────────────────────────────────────────────
# 3. Community Operations
# ─────────────────────────────────────────────────────────────
separator "COMMUNITY"

# Community names max 21 chars — use short prefix + last 6 digits of epoch
COMMUNITY_NAME="e2ecom$(date +%s | tail -c 7)"

log_test "Create community ($COMMUNITY_NAME)"
CREATE_RESP=$(curl -s -X POST "$ENVOY_URL/api/v1/communities" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d "{\"name\":\"$COMMUNITY_NAME\",\"description\":\"E2E test community\",\"visibility\":1}" 2>/dev/null)
CREATE_OK=$(echo "$CREATE_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('community',{}).get('name') else 'no')" 2>/dev/null || echo "no")
if [ "$CREATE_OK" = "yes" ]; then
  pass
else
  fail "Community creation failed: $CREATE_RESP"
fi

log_test "Get community by name"
GET_RESP=$(curl -s "$ENVOY_URL/api/v1/communities/$COMMUNITY_NAME" 2>/dev/null)
GET_OK=$(echo "$GET_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if d.get('community',{}).get('name') else 'no')" 2>/dev/null || echo "no")
if [ "$GET_OK" = "yes" ]; then
  pass
else
  fail "Community not found: $GET_RESP"
fi

log_test "List communities"
LIST_RESP=$(curl -s "$ENVOY_URL/api/v1/communities" 2>/dev/null)
LIST_OK=$(echo "$LIST_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print('yes' if 'communities' in d else 'no')" 2>/dev/null || echo "no")
if [ "$LIST_OK" = "yes" ]; then
  pass
else
  fail "Community listing failed: $LIST_RESP"
fi

# ─────────────────────────────────────────────────────────────
# 4. Anonymous Access (BEFORE rate limiting to avoid 429s)
# ─────────────────────────────────────────────────────────────
separator "ANONYMOUS ACCESS"

log_test "Anonymous can browse communities (no auth)"
ANON_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$ENVOY_URL/api/v1/communities" 2>/dev/null)
if [ "$ANON_CODE" = "200" ]; then
  pass
else
  fail "Expected 200, got $ANON_CODE"
fi

log_test "Anonymous can view profiles (no auth)"
ANON_PROFILE_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$ENVOY_URL/api/v1/users/$TEST_USERNAME" 2>/dev/null)
if [ "$ANON_PROFILE_CODE" = "200" ]; then
  pass
else
  fail "Expected 200, got $ANON_PROFILE_CODE"
fi

log_test "Anonymous cannot create community (auth required)"
ANON_CREATE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ENVOY_URL/api/v1/communities" \
  -H 'Content-Type: application/json' \
  -d '{"name":"anoncommunity","description":"should fail","visibility":1}' 2>/dev/null)
if [ "$ANON_CREATE_CODE" = "401" ] || [ "$ANON_CREATE_CODE" = "403" ]; then
  pass
else
  fail "Expected 401/403, got $ANON_CREATE_CODE"
fi

# ─────────────────────────────────────────────────────────────
# 5. Rate Limiting (last — it depletes anonymous quota)
# ─────────────────────────────────────────────────────────────
separator "RATE LIMITING"

log_test "Rate limit triggers 429 on rapid anonymous requests"
GOT_429=false
for i in $(seq 1 20); do
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$ENVOY_URL/api/v1/communities" 2>/dev/null)
  if [ "$HTTP_CODE" = "429" ]; then
    GOT_429=true
    break
  fi
done
if [ "$GOT_429" = true ]; then
  pass
  echo "         429 received after $i requests"
else
  fail "No 429 received after 20 rapid requests"
fi

# ─────────────────────────────────────────────────────────────
# Results
# ─────────────────────────────────────────────────────────────
separator "RESULTS"
echo ""
printf "  Total:  %d\n" "$TOTAL"
printf "  ${GREEN}Passed: %d${NC}\n" "$PASSED"
if [ "$FAILED" -gt 0 ]; then
  printf "  ${RED}Failed: %d${NC}\n" "$FAILED"
else
  printf "  Failed: %d\n" "$FAILED"
fi
echo ""

if [ "$FAILED" -eq 0 ]; then
  printf "${GREEN}All Phase 2 E2E tests passed!${NC}\n"
  exit 0
else
  printf "${RED}Some tests failed. Check output above for details.${NC}\n"
  echo ""
  echo "Debugging tips:"
  echo "  docker compose ps                          # check service status"
  echo "  docker compose logs {service} --tail=50    # check logs"
  echo "  curl localhost:9901/clusters               # check Envoy cluster health"
  echo "  docker compose exec postgres psql -U redyx -d auth -c '\\dt'  # check DB tables"
  exit 1
fi
