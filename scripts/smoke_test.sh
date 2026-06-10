#!/usr/bin/env bash
# Smoke test: starts the service via docker compose, exercises every endpoint,
# then reports pass/fail for each assertion.
#
# Usage:
#   bash scripts/smoke_test.sh              # start services, test, leave running
#   bash scripts/smoke_test.sh --cleanup    # start, test, then docker compose down
#   bash scripts/smoke_test.sh --no-start   # assume service already up, just test
#
# Requirements: docker, curl, python3

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
USER_ID="713be58e-0d79-4df2-a85c-9f44ca513a7d"   # Alice Smith
EQUIP_ID="2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49"  # AirCat Drill 4337 (2.1 m/s²)
DURATION=60

# AirCat Drill at 60 min: a8 = 2.1 * sqrt(1/8) ≈ 0.7425, points = round(8.82) = 9
EXPECTED_A8_EXPR="2.1 * (0.125 ** 0.5)"
EXPECTED_POINTS=9

# Colours (suppressed when not connected to a terminal)
if [ -t 1 ]; then
    GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
else
    GREEN=''; RED=''; YELLOW=''; BLUE=''; NC=''
fi

PASS=0
FAIL=0
EXPOSURE_ID=""

# Temp file for sharing the HTTP status code out of the curl subshell.
_STATUS_FILE=$(mktemp)
trap 'rm -f "$_STATUS_FILE"' EXIT

# ─── output helpers ──────────────────────────────────────────────────────────

section() { printf "\n${BLUE}▶ %s${NC}\n" "$1"; }
ok()      { printf "  ${GREEN}✓${NC}  %s\n" "$1"; PASS=$((PASS+1)); }
fail()    { printf "  ${RED}✗${NC}  %s\n" "$1"; FAIL=$((FAIL+1)); }

# ─── http helper ─────────────────────────────────────────────────────────────

# call <METHOD> <URL> [body]
# Prints response body to stdout; HTTP status code readable via $(get_status).
call() {
    local method="$1" url="$2" data="${3:-}"
    local tmpbody status
    tmpbody=$(mktemp)
    if [[ -n "$data" ]]; then
        status=$(curl -s -o "$tmpbody" -w "%{http_code}" \
            -X "$method" -H "Content-Type: application/json" -d "$data" \
            "$url" 2>/dev/null) || status=000
    else
        status=$(curl -s -o "$tmpbody" -w "%{http_code}" -X "$method" \
            "$url" 2>/dev/null) || status=000
    fi
    printf '%s' "$status" > "$_STATUS_FILE"
    cat "$tmpbody"
    rm -f "$tmpbody"
}

get_status() { cat "$_STATUS_FILE"; }

# ─── assertion helpers ───────────────────────────────────────────────────────

# assert_status <label> <expected_code>
# Reads the last HTTP status set by call().
assert_status() {
    local label="$1" expected="$2"
    local actual; actual=$(get_status)
    if [[ "$actual" == "$expected" ]]; then
        ok "$label (HTTP $actual)"
        return 0
    else
        fail "$label — expected HTTP $expected, got HTTP $actual"
        return 1
    fi
}

# assert_json <label> <python_bool_expr> <json_body>
# The expression receives the parsed document as `d`.
assert_json() {
    local label="$1" expr="$2" body="$3"
    if python3 -c "
import sys, json
try:
    d = json.loads(sys.argv[1])
    ok = bool($expr)
except Exception as e:
    print('  parse error:', e, file=sys.stderr)
    ok = False
sys.exit(0 if ok else 1)
" "$body" 2>/dev/null; then
        ok "$label"
    else
        fail "$label"
    fi
}

# ─── lifecycle ───────────────────────────────────────────────────────────────

start_services() {
    section "Starting services"
    docker compose up -d --build
}

wait_ready() {
    section "Waiting for API at $BASE_URL"
    local attempts=0
    until curl -sf "$BASE_URL/exposure" >/dev/null 2>&1; do
        ((attempts++))
        if (( attempts >= 30 )); then
            fail "API did not become ready after 60s"
            exit 1
        fi
        printf "  polling... (%ds)\n" "$((attempts * 2))"
        sleep 2
    done
    ok "API is ready"
}

stop_services() {
    section "Tearing down"
    docker compose down
}

# ─── tests ───────────────────────────────────────────────────────────────────

test_list_exposures() {
    section "GET /exposure — list all"
    local body
    body=$(call GET "$BASE_URL/exposure")
    assert_status "status" 200 || return
    assert_json   "response is a JSON array" "isinstance(d, list)" "$body"
}

test_record_exposure() {
    section "POST /exposure — AirCat Drill (2.1 m/s²) for 60 min"
    local payload body
    payload=$(printf '{"equipment_id":"%s","user_id":"%s","duration":%d}' \
        "$EQUIP_ID" "$USER_ID" "$DURATION")
    body=$(call POST "$BASE_URL/exposure" "$payload")
    assert_status "status" 201 || return

    assert_json "id present"              "isinstance(d.get('id'), str) and len(d['id']) > 0"    "$body"
    assert_json "duration = $DURATION"    "d.get('duration') == $DURATION"                        "$body"
    assert_json "a8 ≈ 0.7425 m/s²"       "abs(d.get('a8', 0) - ($EXPECTED_A8_EXPR)) < 0.001"    "$body"
    assert_json "points = $EXPECTED_POINTS (rounded)" \
                                          "d.get('points') == $EXPECTED_POINTS"                   "$body"
    assert_json "user.id echoed back"     "d.get('user', {}).get('id') == '$USER_ID'"             "$body"
    assert_json "equipment.id echoed back" "d.get('equipment', {}).get('id') == '$EQUIP_ID'"      "$body"
    assert_json "created_at present"      "bool(d.get('created_at'))"                             "$body"

    EXPOSURE_ID=$(python3 -c "import sys,json; print(json.loads(sys.argv[1]).get('id',''))" \
        "$body" 2>/dev/null || echo "")
}

test_get_exposure() {
    if [[ -z "$EXPOSURE_ID" ]]; then
        section "GET /exposure/{id} — SKIPPED (no ID from previous test)"
        return
    fi
    section "GET /exposure/$EXPOSURE_ID"
    local body
    body=$(call GET "$BASE_URL/exposure/$EXPOSURE_ID")
    assert_status "status" 200 || return
    assert_json "id matches"        "d.get('id') == '$EXPOSURE_ID'"                     "$body"
    assert_json "duration correct"  "d.get('duration') == $DURATION"                    "$body"
    assert_json "a8 ≈ 0.7425 m/s²" "abs(d.get('a8', 0) - ($EXPECTED_A8_EXPR)) < 0.001" "$body"
    assert_json "points = $EXPECTED_POINTS" "d.get('points') == $EXPECTED_POINTS"       "$body"
}

test_get_exposure_not_found() {
    section "GET /exposure/{id} — not found"
    call GET "$BASE_URL/exposure/00000000-0000-0000-0000-000000000000" >/dev/null
    assert_status "status" 404
}

test_get_exposure_summary() {
    section "GET /users/$USER_ID/exposure-summary — no time filter"
    local body
    body=$(call GET "$BASE_URL/users/$USER_ID/exposure-summary")
    assert_status "status" 200 || return
    assert_json "user.id matches"  "d.get('user', {}).get('id') == '$USER_ID'"                  "$body"
    assert_json "a8 > 0"           "isinstance(d.get('a8'), (int, float)) and d['a8'] > 0"      "$body"
    assert_json "points > 0"       "isinstance(d.get('points'), (int, float)) and d['points'] > 0" "$body"
}

test_get_exposure_summary_with_filter() {
    section "GET /users/{userId}/exposure-summary — with today's time window"
    local today tomorrow body
    today=$(python3    -c "from datetime import datetime, timezone; \
        print(datetime.now(timezone.utc).strftime('%Y-%m-%dT00:00:00Z'))")
    tomorrow=$(python3 -c "from datetime import datetime, timezone, timedelta; \
        print((datetime.now(timezone.utc)+timedelta(days=1)).strftime('%Y-%m-%dT00:00:00Z'))")

    body=$(call GET "$BASE_URL/users/$USER_ID/exposure-summary?starting_at=${today}&ending_at=${tomorrow}")
    assert_status "status" 200 || return
    assert_json "user.id present" "d.get('user', {}).get('id') == '$USER_ID'" "$body"
    assert_json "a8 > 0 within today's window" \
        "isinstance(d.get('a8'), (int, float)) and d['a8'] > 0" "$body"
}

test_validation_errors() {
    section "Validation — 400 cases"
    local body

    body=$(call POST "$BASE_URL/exposure" \
        '{"equipment_id":"not-a-uuid","user_id":"not-a-uuid","duration":60}')
    assert_status "invalid UUIDs → 400" 400

    body=$(call POST "$BASE_URL/exposure" \
        "$(printf '{"equipment_id":"%s","user_id":"%s","duration":0}' "$EQUIP_ID" "$USER_ID")")
    assert_status "zero duration → 400" 400

    body=$(call POST "$BASE_URL/exposure" \
        "$(printf '{"equipment_id":"%s","user_id":"%s","duration":-1}' "$EQUIP_ID" "$USER_ID")")
    assert_status "negative duration → 400" 400

    body=$(call GET "$BASE_URL/exposure/not-a-uuid")
    assert_status "invalid exposureId → 400" 400

    body=$(call GET "$BASE_URL/users/not-a-uuid/exposure-summary")
    assert_status "invalid userId → 400" 400

    body=$(call GET "$BASE_URL/users/$USER_ID/exposure-summary?starting_at=not-a-date")
    assert_status "invalid starting_at → 400" 400

    body=$(call GET "$BASE_URL/users/$USER_ID/exposure-summary?ending_at=not-a-date")
    assert_status "invalid ending_at → 400" 400
}

test_not_found_entity() {
    section "POST /exposure — unknown equipment/user → 404"
    local body

    body=$(call POST "$BASE_URL/exposure" \
        "$(printf '{"equipment_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","user_id":"%s","duration":30}' "$USER_ID")")
    assert_status "unknown equipment → 404" 404

    body=$(call POST "$BASE_URL/exposure" \
        "$(printf '{"equipment_id":"%s","user_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","duration":30}' "$EQUIP_ID")")
    assert_status "unknown user → 404" 404
}

# ─── main ────────────────────────────────────────────────────────────────────

START=true
CLEANUP=false
for arg in "$@"; do
    case "$arg" in
        --no-start) START=false ;;
        --cleanup)  CLEANUP=true ;;
    esac
done

if [[ "$START" == true ]]; then
    start_services
fi

wait_ready

test_list_exposures
test_record_exposure
test_get_exposure
test_get_exposure_not_found
test_get_exposure_summary
test_get_exposure_summary_with_filter
test_validation_errors
test_not_found_entity

# ─── summary ─────────────────────────────────────────────────────────────────

printf "\n${YELLOW}══════════════════════════${NC}\n"
printf "  ${GREEN}Passed: %d${NC}\n" "$PASS"
if (( FAIL > 0 )); then
    printf "  ${RED}Failed: %d${NC}\n" "$FAIL"
fi
printf "${YELLOW}══════════════════════════${NC}\n\n"

if [[ "$CLEANUP" == true ]]; then
    stop_services
fi

exit $(( FAIL > 0 ? 1 : 0 ))
