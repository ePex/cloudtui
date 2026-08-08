# Spec — FE 25: live-verification tooling and workflow

Date: 2026-08-08

## Background

Verifying FE 21–24 this cycle (proxy backend, named connections, messages
multi-select, several `tview` display bugs) repeatedly relied on the same
ad-hoc setup: building the TUI binary, driving it via `tmux send-keys`/
`capture-pane`, and hand-rolling raw JMX calls (via throwaway Go files,
rewritten from scratch more than once) to create/remove disposable test
queues or spin up `mq-proxy` for testing the proxy backend. Several real
bugs (a `tview.Table` swallowing `"[x]"` as a color tag, a queue list
scrolling to the bottom on first load, an invisible confirm dialog, a
connection editor with no Esc-to-cancel) were only caught by this kind of
live driving — unit tests alone did not catch any of them.

## Problem

No repeatable, checked-in way to do this. Every verification session
reinvented the JMX admin script and the tmux driving sequence from scratch,
which is wasted effort and a source of mistakes (e.g. guessing which form
field currently has focus, or picking a filter test string that
accidentally matches everything).

## Solution

1. `tui/cmd/devtool` — a small CLI (mirroring `cmd/seedqueue`'s shape) for:
   - `add-queue <name>` / `remove-queue <name>` — JMX `addQueue`/
     `removeQueue` on the Broker MBean, for disposable test queues
     (ActiveMQ's `sendTextMessage` requires the destination to already
     exist, so a brand-new queue can't be seeded into existence directly).
   - `start-proxy` / `stop-proxy` — build (if needed) and launch `mq-proxy`
     as a background process, waiting until it answers HTTP requests;
     stop it via a pid file.
   Wired into `Taskfile.yml` as `test:queue:add`/`remove` and
   `dev:proxy:start`/`stop`.
2. `.claude/skills/verify-live/SKILL.md` — a project skill capturing the
   tmux driving pattern (build, launch, send-keys, capture-pane, key
   reference), the broker-safety rules learned this cycle (don't assume
   the dev broker is empty or disposable), and the `tview` gotchas found
   along the way, so they don't get rediscovered.
3. `tui/CLAUDE.md`'s Testing section now points to the skill for changes
   touching queue/message/connection behavior.

## Scope

### In scope

- `tui/internal/devtool/` (`queue.go`, `proxy.go`) and `tui/cmd/devtool/`.
- Unit tests for the testable pieces: the JMX request shape (`AddQueue`/
  `RemoveQueue`, via `httptest`), the HTTP-readiness poll (`waitHTTP`), and
  Java-binary resolution (`javaBinary`, `$JAVA_HOME` vs. `PATH`).
- Taskfile wiring.
- The `verify-live` skill.
- `tui/CLAUDE.md` Testing section update.

### Out of scope

- Automating this into a CI-gated regression suite. Driving a real TUI via
  tmux against a real broker is inherently time-sensitive
  (`capture-pane` timing, process startup) — good for a documented manual
  step, not a reliable CI gate. See discussion in the originating
  conversation: Tier 2 (full automation) was explicitly deferred in favor
  of this lighter Tier 1 (reusable tooling + documented workflow).
- Process supervision beyond a bare pid file (no restart-on-crash, no
  log rotation) — this is dev-only tooling, not a production concern.
- `add-queue`/`remove-queue` for the proxy backend — JMX is Jolokia-specific
  by nature; mq-proxy already has its own send/purge/delete endpoints for
  managing state through that backend directly.

## Design notes

- **`javaBinary()` resolves `$JAVA_HOME/bin/java` when set**, not just
  `"java"` from `PATH`. Found live: this machine's `PATH` java resolves to
  Java 17 via sdkman's "current" symlink, but mq-proxy's toolchain was
  bumped to Java 21 (spec `chore(mq-proxy)` commit), so a plain
  `exec.Command("java", ...)` failed with `UnsupportedClassVersionError`
  until this fix.
- **A failed `start-proxy` cleans up its own pid file** (found live: the
  first `UnsupportedClassVersionError` run left a stale `devtool.pid`
  pointing at an already-dead process).
- **`start-proxy`/`stop-proxy` use a pid file + `os.Process.Kill()`**
  rather than shell `kill`/`taskkill`, for genuine cross-platform behavior
  through the Go standard library rather than shelling out to an
  OS-specific command.

## Files touched

| File | Change |
|---|---|
| `tui/internal/devtool/queue.go` | new — `AddQueue`/`RemoveQueue` via JMX |
| `tui/internal/devtool/queue_test.go` | new |
| `tui/internal/devtool/proxy.go` | new — `StartProxy`/`StopProxy`/`waitHTTP`/`javaBinary` |
| `tui/internal/devtool/proxy_test.go` | new |
| `tui/cmd/devtool/main.go` | new — CLI wiring |
| `Taskfile.yml` | add `test:queue:add`/`remove`, `dev:proxy:start`/`stop` |
| `mq-proxy/.gitignore` | ignore `devtool.pid`/`devtool.log` |
| `tui/CLAUDE.md` | package layout + Testing section |
| `.claude/skills/verify-live/SKILL.md` | new |

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Verified live: `test:queue:add`/`remove` confirmed against the real
   broker (queue created, `QueueSize` readable, then confirmed gone after
   removal). `dev:proxy:start`/`stop` confirmed end-to-end, including
   reproducing and fixing the Java-version mismatch and the stale-pid-file
   case on a failed start.
