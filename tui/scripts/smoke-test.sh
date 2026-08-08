#!/usr/bin/env bash
#
# Smoke-tests the cloudtui golden path against a real broker by driving the
# real TUI binary in tmux: list queues, seed/browse/mark/delete/move
# messages, then switch to the mq-proxy backend and confirm it sees the
# same broker state.
#
# Requires: tmux, Go, a reachable ActiveMQ broker matching the active
# connection in tui/config.yaml (Jolokia), and — for the backend-switch
# phase — a JDK capable of running mq-proxy (Java 21+; export JAVA_HOME if
# `java` on PATH resolves to something older, e.g. via sdkman's "current").
#
# POSIX/macOS/Linux only: tmux has no native Windows build. See
# .claude/skills/verify-live/ for the manual version of this same workflow
# and the tview gotchas found while building it.
#
# Usage: ./scripts/smoke-test.sh   (run from tui/, or `task smoke:test`)

set -euo pipefail
cd "$(dirname "$0")/.."

for bin in tmux go; do
  command -v "$bin" >/dev/null 2>&1 || { echo "smoke-test: '$bin' is required but not found" >&2; exit 1; }
done

SESSION="cloudtui-smoke-$$"
BIN="$(mktemp -t cloudtui-smoke.XXXXXX)"
SRC_QUEUE="smoketest-src-$$"
DST_QUEUE="smoketest-dst-$$"
CONN_NAME="smoketest-proxy-$$"
CONN_ALIAS="smk"
CONFIG_BACKUP=""
STARTED_PROXY=0
EXISTING_CONN_COUNT=0

log() { echo "[smoke-test] $*"; }

cleanup() {
  local status=$?
  log "cleaning up..."
  tmux kill-session -t "$SESSION" >/dev/null 2>&1 || true
  # Restore config.yaml (and so the active jolokia connection) BEFORE
  # removing test queues: devtool's add-queue/remove-queue refuse to run
  # against a non-jolokia active connection, and by this point the script
  # may have switched the active connection to the test proxy one.
  if [[ -n "$CONFIG_BACKUP" ]]; then
    mv -f "$CONFIG_BACKUP" config.yaml
  fi
  go run ./cmd/devtool remove-queue "$SRC_QUEUE" >/dev/null 2>&1 || true
  go run ./cmd/devtool remove-queue "$DST_QUEUE" >/dev/null 2>&1 || true
  if [[ "$STARTED_PROXY" == "1" ]]; then
    go run ./cmd/devtool stop-proxy >/dev/null 2>&1 || true
  fi
  rm -f "$BIN"
  if [[ $status -eq 0 ]]; then
    log "PASS"
  else
    log "FAIL (exit $status)"
  fi
  exit $status
}
trap cleanup EXIT

cap() { tmux capture-pane -t "$SESSION" -p; }
send() { tmux send-keys -t "$SESSION" "$@"; }

# wait_for polls the pane (rather than a fixed sleep) until needle appears
# or timeout (seconds) elapses, printing the matching capture on success.
wait_for() {
  local needle="$1" timeout="${2:-5}" tries out
  tries=$((timeout * 5))
  for ((i = 0; i < tries; i++)); do
    out="$(cap)"
    if [[ "$out" == *"$needle"* ]]; then
      printf '%s' "$out"
      return 0
    fi
    sleep 0.2
  done
  log "FAIL: timed out after ${timeout}s waiting for: $needle"
  echo "--- last screen ---" >&2
  cap >&2
  return 1
}

# filter_queues sets the queues-list filter to needle. '/' on that view
# restores whatever filter text was last applied (queues.go:
# filterInput.SetText(qv.filter)) rather than starting empty, so clear it
# first rather than assuming it's blank.
filter_queues() {
  send '/'
  sleep 0.15
  for ((i = 0; i < 40; i++)); do send 'BSpace'; done
  send "$1"
  sleep 0.2
  send 'Enter'
  sleep 0.2
}

log "building tui..."
go build -o "$BIN" ./cmd/tui

log "creating disposable queues: $SRC_QUEUE, $DST_QUEUE"
go run ./cmd/devtool add-queue "$SRC_QUEUE"
go run ./cmd/devtool add-queue "$DST_QUEUE"

if [[ -f config.yaml ]]; then
  CONFIG_BACKUP="$(mktemp -t cloudtui-config-backup.XXXXXX)"
  cp config.yaml "$CONFIG_BACKUP"
  EXISTING_CONN_COUNT=$(grep -c '^ *- name:' config.yaml || true)
fi

log "starting mq-proxy (needs JAVA_HOME pointed at a 21+ JDK)..."
go run ./cmd/devtool start-proxy
STARTED_PROXY=1

log "registering a test proxy connection ($EXISTING_CONN_COUNT existing)..."
go run ./cmd/devtool add-proxy-conn "$CONN_NAME" "$CONN_ALIAS" \
  http://localhost:8080 cloudtui changeme

log "launching TUI in tmux session $SESSION..."
tmux new-session -d -s "$SESSION" -x 130 -y 40 "$BIN"
wait_for "CLOUDTUI" 10 >/dev/null

log "opening queues list..."
send 'h'
sleep 0.2
send 'Enter'
wait_for "Queues" 5 >/dev/null

log "confirming $SRC_QUEUE is listed and empty..."
filter_queues "$SRC_QUEUE"
out="$(wait_for "$SRC_QUEUE" 5)"

log "seeding 3 messages into $SRC_QUEUE..."
go run ./cmd/seedqueue "$SRC_QUEUE" 3 >/dev/null
send 'r'
sleep 0.5
out="$(wait_for "$SRC_QUEUE" 5)"
[[ "$out" == *"3"* ]] || { log "FAIL: expected a pending count of 3 for $SRC_QUEUE"; echo "$out" >&2; exit 1; }

log "browsing messages..."
send 'Enter'
wait_for "order." 5 >/dev/null # sample seedqueue bodies contain "order.<event>"

log "marking all and deleting..."
send 'a'
wait_for "Marked 3 message(s)" 3 >/dev/null
send 'd'
sleep 0.3
send 'Down'
sleep 0.1
send 'Enter'
wait_for "Deleted 3 message(s)" 5 >/dev/null

log "seeding 2 more messages for the move test..."
go run ./cmd/seedqueue "$SRC_QUEUE" 2 >/dev/null
send 'r'
wait_for "order." 5 >/dev/null

log "marking all and moving to $DST_QUEUE..."
send 'a'
wait_for "Marked 2 message(s)" 3 >/dev/null
send 'm'
sleep 0.3
send '/'
sleep 0.15
send "$DST_QUEUE"
sleep 0.2
send 'Enter'
sleep 0.2
send 'Enter'
wait_for "Moved 2 message(s)" 5 >/dev/null

log "verifying $DST_QUEUE actually received them..."
send 'Escape'
sleep 0.2
filter_queues "$DST_QUEUE"
out="$(wait_for "$DST_QUEUE" 5)"
[[ "$out" == *"2"* ]] || { log "FAIL: expected a pending count of 2 for $DST_QUEUE"; echo "$out" >&2; exit 1; }

log "switching to the proxy backend..."
send 's'
sleep 0.2
send 'Down'
sleep 0.1
send 'Enter'
sleep 0.3
for ((i = 0; i < EXISTING_CONN_COUNT; i++)); do send 'Down'; sleep 0.05; done
send 'Enter'
wait_for "Connection: $CONN_ALIAS" 5 >/dev/null

log "confirming the proxy backend sees the same broker state ($DST_QUEUE)..."
send 'r'
wait_for "$DST_QUEUE" 8 >/dev/null

send 'q'
log "all golden-path checks passed"
