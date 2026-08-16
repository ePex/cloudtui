#!/usr/bin/env bash
#
# Exercises the REST contract shared by mq-proxy and the reference
# queue-management API directly (no TUI, no tmux — see smoke-test.sh for
# the TUI-driving equivalent): send 10 messages, check the fields the TUI's
# detail view needs, check filtering, delete one message, bulk-delete
# several, move one message, bulk-move several, and purge what's left.
# Prints PASS/FAIL per step and a final summary.
#
# Requires: curl, jq.
#
# Usage:
#   ./scripts/verify-proxy-api.sh <base-url> <username> <password> <queue> [target-queue]
#
# Examples:
#   ./scripts/verify-proxy-api.sh http://localhost:8080 cloudtui changeme my-disposable-queue
#   ./scripts/verify-proxy-api.sh http://localhost:8110 <user> <pass> <an-authorized-queue-name>
#
# queue must already exist / be usable by the given credentials — mq-proxy
# auto-creates a queue on first send, but the reference API's credentials
# are typically restricted to specific pre-existing queues, so pick one
# you're authorized for there.
#
# DESTRUCTIVE: this script ends by purging BOTH queue and target-queue.
# Only point it at disposable/test queues you own — never at a queue with
# real messages you care about. No confirmation prompt, same convention as
# this repo's other disposable-queue tooling (task test:queue:add/seed:queue).

set -uo pipefail
cd "$(dirname "$0")/.."

for bin in curl jq; do
  command -v "$bin" >/dev/null 2>&1 || { echo "verify-proxy-api: '$bin' is required but not found" >&2; exit 1; }
done

if [[ $# -lt 4 ]]; then
  sed -n '2,29p' "$0" | sed 's/^# \?//'
  exit 1
fi

BASE_URL="$1"
USERNAME="$2"
PASSWORD="$3"
QUEUE="$4"
TARGET_QUEUE="${5:-${QUEUE}-verify-target}"

PASS_COUNT=0
FAIL_COUNT=0

log() { echo "[verify-proxy-api] $*"; }

pass() {
  log "PASS: $1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
  log "FAIL: $1"
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

assert_eq() {
  local desc="$1" want="$2" got="$3"
  if [[ "$want" == "$got" ]]; then
    pass "$desc (= $got)"
  else
    fail "$desc (want $want, got $got)"
  fi
}

api_get() { curl -s -u "$USERNAME:$PASSWORD" "$BASE_URL$1"; }

api_post() {
  curl -s -u "$USERNAME:$PASSWORD" -X POST "$BASE_URL$1" \
    -H "Content-Type: application/json" -d "$2"
}

# count QUEUE [extra-query-params]
count() {
  local q="$1" extra="${2:-}"
  api_get "/api/management/command/list-messages?sourceQueue=$q${extra:+&$extra}" | jq '.data | length'
}

log "target: $BASE_URL, queue=$QUEUE, target-queue=$TARGET_QUEUE"

# Baseline counts: every assertion below is relative to whatever was
# already on each queue before this run, not an absolute count — so a
# non-empty (but authorized) queue doesn't produce spurious failures.
# Still warn, since queue/target-queue are documented as disposable.
SRC_BASE=$(count "$QUEUE")
TGT_BASE=$(count "$TARGET_QUEUE")
[[ "$SRC_BASE" != "0" ]] && log "WARNING: $QUEUE already has $SRC_BASE message(s) — counts below are relative to that"
[[ "$TGT_BASE" != "0" ]] && log "WARNING: $TARGET_QUEUE already has $TGT_BASE message(s) — counts below are relative to that"

# ---------------------------------------------------------------------------
# 1. Send 10 messages: 7 order-created, 3 invoice-created. Message #1 also
#    carries a correlation ID and a custom header, for the detail-view check.
# ---------------------------------------------------------------------------
log "=== send 10 messages ==="
declare -a IDS=()
for i in $(seq 1 10); do
  jms_type="order-created"
  [[ $i -gt 7 ]] && jms_type="invoice-created"
  body="{\"id\":$i,\"event\":\"$jms_type\"}"
  if [[ $i -eq 1 ]]; then
    payload=$(jq -n --arg q "$QUEUE" --arg t "$jms_type" --arg b "$body" \
      '{targetQueue:$q,jmsType:$t,body:$b,correlationId:"verify-corr-1",headers:{testRun:"verify-proxy-api"}}')
  else
    payload=$(jq -n --arg q "$QUEUE" --arg t "$jms_type" --arg b "$body" \
      '{targetQueue:$q,jmsType:$t,body:$b}')
  fi
  resp=$(api_post "/api/management/command/send-message" "$payload")
  msgid=$(echo "$resp" | jq -r '.data.messageId // empty')
  if [[ -z "$msgid" ]]; then
    fail "send message $i (response: $resp)"
  else
    IDS+=("$msgid")
  fi
done
assert_eq "sent 10 messages" "10" "${#IDS[@]}"
assert_eq "queue has 10 more messages after send" "$((SRC_BASE + 10))" "$(count "$QUEUE")"

# ---------------------------------------------------------------------------
# 2. Detail-view field check. returnBody must be requested explicitly —
#    the reference API defaults it to false (mq-proxy defaults true; tui
#    itself always asks explicitly, see FE 46), so without it this check
#    would spuriously fail against the reference API.
# ---------------------------------------------------------------------------
log "=== detail-view fields ==="
detail=$(api_get "/api/management/command/list-messages?sourceQueue=$QUEUE&filter.maxCount=1&returnBody=true")
if echo "$detail" | jq -e '.data[0] | (.messageId != null and .jmsType != null and .timestamp != null and .body != null and .headers != null)' >/dev/null 2>&1; then
  pass "message carries messageId/jmsType/timestamp/body/headers"
else
  fail "message missing expected detail fields (got: $(echo "$detail" | jq -c '.data[0]'))"
fi

# ---------------------------------------------------------------------------
# 3. Filtering
# ---------------------------------------------------------------------------
log "=== filtering ==="
assert_eq "filter by jmsType=order-created" "7" "$(count "$QUEUE" "filter.jmsType=order-created")"
assert_eq "filter by jmsType=invoice-created" "3" "$(count "$QUEUE" "filter.jmsType=invoice-created")"
assert_eq "filter by maxCount=2" "2" "$(count "$QUEUE" "filter.maxCount=2")"

# ---------------------------------------------------------------------------
# 4. Delete one message (by exact ID)
# ---------------------------------------------------------------------------
log "=== delete one message ==="
resp=$(api_post "/api/management/command/delete-messages" \
  "[{\"sourceQueue\":\"$QUEUE\",\"filter\":{\"messageId\":\"${IDS[0]}\",\"maxCount\":1}}]")
assert_eq "delete-messages(id) returned 1 deleted" "1" "$(echo "$resp" | jq '.data | length')"
assert_eq "queue has 9 more messages after single delete" "$((SRC_BASE + 9))" "$(count "$QUEUE")"

# ---------------------------------------------------------------------------
# 5. Delete several messages (bulk, by JMS type — matches the 3 invoice-created)
# ---------------------------------------------------------------------------
log "=== delete several messages (bulk) ==="
resp=$(api_post "/api/management/command/delete-messages" \
  "[{\"sourceQueue\":\"$QUEUE\",\"filter\":{\"jmsType\":\"invoice-created\",\"maxCount\":3}}]")
assert_eq "bulk delete returned 3 deleted" "3" "$(echo "$resp" | jq '.data | length')"
assert_eq "queue has 6 more messages after bulk delete" "$((SRC_BASE + 6))" "$(count "$QUEUE")"

# ---------------------------------------------------------------------------
# 6. Move one message (by exact ID)
# ---------------------------------------------------------------------------
log "=== move one message ==="
resp=$(api_post "/api/management/command/move-messages" \
  "[{\"sourceQueue\":\"$QUEUE\",\"targetQueue\":\"$TARGET_QUEUE\",\"filter\":{\"messageId\":\"${IDS[1]}\",\"maxCount\":1}}]")
assert_eq "move-messages(id) returned 1 moved" "1" "$(echo "$resp" | jq '.data | length')"
assert_eq "source has 5 more messages after single move" "$((SRC_BASE + 5))" "$(count "$QUEUE")"
assert_eq "target has 1 more message after single move" "$((TGT_BASE + 1))" "$(count "$TARGET_QUEUE")"

# ---------------------------------------------------------------------------
# 7. Move several messages (bulk, maxCount-capped)
# ---------------------------------------------------------------------------
log "=== move several messages (bulk) ==="
resp=$(api_post "/api/management/command/move-messages" \
  "[{\"sourceQueue\":\"$QUEUE\",\"targetQueue\":\"$TARGET_QUEUE\",\"filter\":{\"maxCount\":2}}]")
assert_eq "bulk move returned 2 moved" "2" "$(echo "$resp" | jq '.data | length')"
assert_eq "source has 3 more messages after bulk move" "$((SRC_BASE + 3))" "$(count "$QUEUE")"
assert_eq "target has 3 more messages after bulk move" "$((TGT_BASE + 3))" "$(count "$TARGET_QUEUE")"

# ---------------------------------------------------------------------------
# 8. Purge both queues
# ---------------------------------------------------------------------------
log "=== purge ==="
api_post "/api/management/command/delete-messages" "[{\"sourceQueue\":\"$QUEUE\",\"filter\":{}}]" >/dev/null
assert_eq "source empty after purge" "0" "$(count "$QUEUE")"
api_post "/api/management/command/delete-messages" "[{\"sourceQueue\":\"$TARGET_QUEUE\",\"filter\":{}}]" >/dev/null
assert_eq "target empty after purge" "0" "$(count "$TARGET_QUEUE")"

# ---------------------------------------------------------------------------
log "=== summary ==="
log "$PASS_COUNT passed, $FAIL_COUNT failed"
[[ $FAIL_COUNT -eq 0 ]]
