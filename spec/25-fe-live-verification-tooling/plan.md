# Plan — FE 25: live-verification tooling and workflow

## `tui/internal/devtool/queue.go`

`AddQueue`/`RemoveQueue` both call a shared `execBrokerOp(ctx, cfg,
operation, arg)` that POSTs a Jolokia `exec` request to the Broker MBean
(`org.apache.activemq:type=Broker,brokerName=<name>`), matching the request
shape already used by `jolokia.Client.SendMessage` (`Origin: http://localhost`
header, `Content-Type: application/json`, HTTP Basic auth from
`config.QueueConfig`). Not part of `queue.Backend` — this is broker admin,
not a queue-data operation, and only meaningful for Jolokia.

## `tui/internal/devtool/proxy.go`

`StartProxy(ctx, timeout)`:
1. `ProxyRunning()` check first — refuse to double-start.
2. Run `gradlew bootJar` (`gradlew.bat` on Windows) in `ProxyDir`
   (`../mq-proxy`, relative to `tui/` — every Taskfile entry that invokes
   `go run` from `dir: tui` shares this assumption).
3. Launch `javaBinary() -jar build/libs/mq-proxy.jar` detached
   (`exec.Command(...).Start()`, not `.Run()`), output redirected to
   `ProxyDir/devtool.log`.
4. Write the PID to `ProxyDir/devtool.pid`.
5. `waitHTTP` against `http://localhost:8080/api/queues` up to `timeout`;
   on failure, kill the process and remove the pid file before returning
   the error (don't leave stale state for the next start/stop).

`StopProxy()`: read the pid file, `os.FindProcess` + `.Kill()`, remove both
bookkeeping files.

`javaBinary()`: `$JAVA_HOME/bin/java` (`.exe` on Windows) if `JAVA_HOME` is
set, else `"java"` (PATH lookup via `exec.Command`'s default behavior).

`waitHTTP(ctx, url, timeout)`: polls every 300ms; any HTTP response (even a
4xx/5xx) counts as "up" — this is a liveness check, not an auth check.

## `tui/cmd/devtool/main.go`

Same shape as `cmd/seedqueue`: parse args, load config for
`add-queue`/`remove-queue` (reusing `cfg.ActiveConn()`, erroring if the
active connection isn't `jolokia`), delegate to `internal/devtool`, plain
stdout/stderr reporting.

## Taskfile

```yaml
test:queue:add:
  desc: 'Create a disposable ActiveMQ queue for manual testing. Usage: task test:queue:add -- <name>'
  dir: tui
  cmds: [go run ./cmd/devtool add-queue {{.CLI_ARGS}}]

test:queue:remove:
  desc: 'Remove a disposable ActiveMQ queue created for manual testing. Usage: task test:queue:remove -- <name>'
  dir: tui
  cmds: [go run ./cmd/devtool remove-queue {{.CLI_ARGS}}]

dev:proxy:start:
  desc: Build and start mq-proxy in the background, for live-testing the proxy backend.
  dir: tui
  cmds: [go run ./cmd/devtool start-proxy]

dev:proxy:stop:
  desc: Stop the mq-proxy instance started by dev:proxy:start.
  dir: tui
  cmds: [go run ./cmd/devtool stop-proxy]
```

## Testing

`queue_test.go`: `httptest.Server` asserting the exact mbean/operation/
arguments JSON shape and headers for both `AddQueue` and `RemoveQueue`,
plus the Jolokia-failure-status and transport-failure error paths.

`proxy_test.go`: `waitHTTP` against an already-up `httptest.Server`
(near-instant), against nothing listening (times out), and against an
already-cancelled context. `javaBinary` with `JAVA_HOME` set/unset via
`t.Setenv`.

`StartProxy`/`StopProxy` themselves aren't unit tested — they spawn a real
Gradle build and JVM process, which is slow, environment-dependent (needs
a JDK, a broker), and not hermetic. Verified manually instead (see
spec.md, Definition of done #2), which is also how this surfaced the
Java-version and stale-pid-file issues fixed along the way.

## `.claude/skills/verify-live/SKILL.md`

Not code — a playbook. Content mirrors what was actually done and learned
verifying FE 21–24 this cycle: broker-safety rules, the tmux driving
pattern and key reference, the `tview` gotchas (bracket-tag swallowing,
`trackEnd` scroll latch, `Pages` z-order, `Form` focus-index persistence),
and how to stand up `mq-proxy` for the proxy-backend path.

## No new dependencies
