# Developer verification tooling

_Condensed from spec/23, spec/25, spec/26 — see those folders for the incremental history._

## Purpose

Reusable, checked-in tooling for developing and manually verifying cloudtui
against a real ActiveMQ broker, instead of hand-rolling throwaway JMX
scripts and tmux driving sequences every session. Deliberately **not**
CI-automated: driving a real TUI via tmux against a live broker is
time-sensitive (`capture-pane` timing, process startup) and considered too
flaky for a CI gate — this is manual/on-demand tooling plus Go regression
tests per bug found, not an automation replacement.

## Behavior / user flow

### `cmd/seedqueue` — bulk-seed sample messages
`task seed:queue -- <queue> <count>` sends `<count>` sample JSON "order
event" messages to an existing queue via the active connection's **Jolokia**
backend only (not proxy). Message shape:
```json
{"id": 1, "event": "order.created", "timestamp": "...", "amount": 12.34, "customer": "acme-corp"}
```
`event`/`customer` are picked at random from small fixed lists; `id` is
sequential starting at 1. Does not create the destination queue (ActiveMQ's
JMX `sendTextMessage` requires it to already exist).

### `cmd/devtool` — disposable test-queue and mq-proxy lifecycle management
- `add-queue <name>` / `remove-queue <name>` (`task test:queue:add`/
  `remove`) — JMX `addQueue`/`removeQueue` on the Broker MBean, Jolokia
  only. Needed because a brand-new queue can't be seeded into existence
  directly (JMX `sendTextMessage` requires the destination to pre-exist).
  Refuses to run against a non-jolokia active connection.
- `start-proxy` / `stop-proxy` (`task dev:proxy:start`/`stop`) — builds (if
  needed) and launches `mq-proxy` as a background process, polling until it
  answers HTTP requests before returning; stops it via a pid file +
  `os.Process.Kill()` (not shell `kill`/`taskkill`, for genuine
  cross-platform behavior). A failed start cleans up its own stale pid
  file. Java binary resolution prefers `$JAVA_HOME/bin/java` over a plain
  `"java"` from `PATH` — a `PATH` java can silently resolve to an older JDK
  than `mq-proxy`'s toolchain requires (Java 21+), producing
  `UnsupportedClassVersionError` otherwise.
- `add-proxy-conn <name> <url> <username> <password>` — writes a proxy
  connection directly into `config.yaml` via `AddProxyConnection` (pure,
  unit-tested function), bypassing the connection-editor form entirely.
  Exists because a script/tool has no reliable way to drive the editor
  form's own focus-persistence quirks the way a human/agent can.

### `tui/scripts/smoke-test.sh` (`task smoke:test`) — golden-path regression check
Drives the real TUI binary in tmux through the core path in one script (not
per-feature scripts — rejected as multiplying the same flakiness/
maintenance cost across N files): list queues → seed/browse messages → mark
→ delete → mark → move → switch to the mq-proxy backend → confirm it sees
the same broker state.
- Creates its own disposable, PID-suffixed queue names (e.g.
  `smoketest-src-$$`) via `devtool add-queue`, so concurrent/rerun
  invocations don't collide with leftover state.
- Starts `mq-proxy` itself and registers a temporary proxy connection via
  `devtool add-proxy-conn`.
- Cleanup is a `trap cleanup EXIT`: **restores `config.yaml` from a full
  backup taken before the run** (not surgical UI-driven undo) **before**
  removing the disposable queues — order matters, because
  `add-queue`/`remove-queue` refuse to run against a non-jolokia active
  connection, and by cleanup time the active connection may still be the
  temporary proxy one.
- Bash, not Go (unlike `seedqueue`/`devtool`) — the script's whole purpose
  is orchestrating `tmux`, which has no native Windows build regardless of
  implementation language, so Go buys no cross-platform benefit here.
  POSIX/macOS/Linux only, matching the `verify-live` skill's scope.
- Does not test the connection *editor* UI itself (that's
  `connections_test.go`'s job) — only needs a connection to exist, which
  `add-proxy-conn` provides directly.

### `.claude/skills/verify-live/SKILL.md`
A project skill capturing the tmux driving pattern (build, launch,
send-keys, capture-pane, key reference), broker-safety rules (don't assume
the dev broker is empty or disposable), and `tview` gotchas found through
live driving. `tui/CLAUDE.md`'s Testing section points to it for any change
touching queue/message/connection behavior — several real bugs (a
`tview.Table` swallowing `"[x]"` as a color tag, a queue list scrolling to
the bottom on first load, an invisible confirm dialog, a connection editor
with no Esc-to-cancel) were only ever caught this way, not by unit tests.

## Data & config

- `tui/internal/seed/` — `Sender` interface (`SendMessage(ctx, queueName,
  body string) error`), `Run(...)` send loop, `sampleMessage(...)`
  generation.
- `tui/internal/devtool/queue.go` — `AddQueue`/`RemoveQueue` via JMX.
- `tui/internal/devtool/proxy.go` — `StartProxy`/`StopProxy`/`waitHTTP`/
  `javaBinary`.
- `tui/internal/devtool/config.go` — `AddProxyConnection`.
- `Taskfile.yml` tasks: `seed:queue`, `test:queue:add`, `test:queue:remove`,
  `dev:proxy:start`, `dev:proxy:stop`, `smoke:test`.

## Implementation notes

- `tui/cmd/seedqueue/main.go`, `tui/cmd/devtool/main.go` — thin CLI
  wrappers: load config, resolve the active connection, construct a
  `jolokia.Client`, delegate to the internal package.
- `mq-proxy/.gitignore` ignores `devtool.pid`/`devtool.log`.

## Notable gotchas worth preserving

- Java version mismatch: a `PATH`-resolved `java` can silently be an older
  JDK than `mq-proxy` needs even when `$JAVA_HOME` points at the right one
  (e.g. via a version-manager "current" symlink) — always prefer
  `$JAVA_HOME/bin/java` when set.
- Cleanup ordering bug (found by actually running smoke-test.sh twice and
  checking the broker, not by reading the script): removing disposable
  queues *before* restoring `config.yaml` leaves the active connection
  pointed at the temporary proxy backend, so `devtool remove-queue` (Jolokia
  -only) silently no-ops against it. Always restore `config.yaml` first,
  then remove queues.
- Full-file backup/restore beats surgical UI-driven cleanup for
  `config.yaml` — simpler and more robust than reasoning about which
  connection to reactivate or list positions after the fact.
