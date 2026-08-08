# Spec — FE 26: golden-path smoke-test script

Date: 2026-08-08

## Background

FE 25 built reusable tooling (`devtool`) and a manual playbook
(`verify-live` skill) for live-testing against a real broker, but explicitly
scoped out automating any of it — "tmux capture-pane timing against a live
broker is too flaky" for CI. This spec adds a middle ground: one script that
automates the *golden path itself* (not per-feature scripts, which was
rejected in the same conversation for the same flakiness/maintenance
reasons multiplied across N files) as a manual, on-demand regression check.

## Problem

No fast way to sanity-check "is the core path still working" after a change,
short of manually replaying the `verify-live` steps by hand every time.

## Solution

`tui/scripts/smoke-test.sh` (wired to `task smoke:test`) drives the real TUI
binary in tmux through: list queues → seed/browse messages → mark → delete
→ mark → move → switch to the mq-proxy backend → confirm it sees the same
broker state. It creates its own disposable, uniquely-named queues (via
`devtool add-queue`, PID-suffixed to avoid collisions across runs), starts
`mq-proxy` itself, registers a temporary proxy connection directly into
`config.yaml` (bypassing the connection-editor form entirely — see design
notes), and restores everything via a `trap cleanup EXIT`, backing up and
restoring `config.yaml` verbatim rather than trying to surgically undo
changes made through the UI.

## Scope

### In scope

- `tui/scripts/smoke-test.sh`.
- `tui/internal/devtool/config.go`'s `AddProxyConnection` (pure function,
  unit tested) + a `devtool add-proxy-conn` CLI subcommand, so the script
  can register a test connection without driving the connection editor's
  form (which has its own known focus-persistence quirk — see the
  `verify-live` skill — that a script has no good way to probe for itself
  the way a human/agent can).
- `task smoke:test` in `Taskfile.yml`.
- `verify-live` skill and `tui/CLAUDE.md` updated to reference it.

### Out of scope

- CI integration — same reasoning as FE 25: this needs a live broker and
  (for the backend-switch phase) a JDK 21+ `JAVA_HOME`, and involves
  `sleep`-paced terminal capture. It's a manual/on-demand tool.
- Per-feature scripts — explicitly rejected in favor of this single
  shared-path script plus Go regression tests per bug (the pattern already
  used for e.g. the queue-scroll-to-top and multi-select fixes).
- Testing the connection *editor* UI itself (creating a connection by
  filling out the form) — that's `tui/internal/app/connections_test.go`'s
  job; this script only needs a connection to *exist*, which
  `add-proxy-conn` gives it directly and more reliably.

## Design notes

- **Bash, not Go**, unlike every other tool added this cycle
  (`seedqueue`, `devtool`). The reasoning that favored Go for those
  (cross-platform via `go run` from any OS) doesn't transfer here: the
  script's entire purpose is orchestrating `tmux`, which has no native
  Windows build regardless of what language drives it, so Go would buy no
  cross-platform benefit while adding ceremony for what's fundamentally a
  sequence of shell commands with `sleep`s. Documented as POSIX/macOS/Linux
  only, matching the `verify-live` skill's own scope.
- **`config.yaml` full backup/restore, not surgical UI-driven cleanup.**
  Adding/removing a connection through the editor form was deliberately
  avoided for the *setup* side (see `add-proxy-conn` above); for teardown,
  restoring the whole file from a pre-run backup is simpler and more
  robust than trying to compute which connection to reactivate or
  reason about list positions after the fact.
- **Queue names are PID-suffixed** (`smoketest-src-$$`), so concurrent or
  interrupted-and-rerun invocations don't collide with leftover state from
  a previous run.
- **Found live, while building this script**: cleanup originally removed
  the disposable queues *before* restoring `config.yaml`, so by that point
  the active connection was still the temporary proxy one — and `devtool
  add-queue`/`remove-queue` refuse to run against a non-jolokia active
  connection. Both `remove-queue` calls silently failed (swallowed by
  `|| true`), leaking both test queues on every run that reached the
  backend-switch phase. Fixed by restoring `config.yaml` first. Caught by
  actually running the script twice and checking the broker afterward, not
  by reading the script.

## Files touched

| File | Change |
|---|---|
| `tui/scripts/smoke-test.sh` | new |
| `tui/internal/devtool/config.go` | new — `AddProxyConnection` |
| `tui/internal/devtool/config_test.go` | new |
| `tui/cmd/devtool/main.go` | add `add-proxy-conn` subcommand |
| `Taskfile.yml` | add `smoke:test` |
| `.claude/skills/verify-live/SKILL.md` | mention the script |
| `tui/CLAUDE.md` | mention the script |

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. `task smoke:test` passes end-to-end against the real broker.
3. Verified idempotency: ran it twice back-to-back successfully.
4. Verified the failure path: forced `start-proxy` to fail (wrong JDK via
   unset `JAVA_HOME`) and confirmed cleanup still fully ran (queues
   removed, `config.yaml` restored, no stale pid file) and the script
   exited non-zero with a clear message.
5. Verified full cleanup after both a passing and a failing run by
   querying the broker directly (not just trusting the script's own
   "cleaning up..." log line) — this is exactly how the stale-active-
   connection bug above was actually caught.
