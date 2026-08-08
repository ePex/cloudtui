# Tasks — FE 25: live-verification tooling and workflow

Plan: [plan.md](plan.md)

1. [x] `tui/internal/devtool/queue.go` — `AddQueue`/`RemoveQueue` via JMX.
2. [x] `tui/internal/devtool/queue_test.go`.
3. [x] `tui/internal/devtool/proxy.go` — `StartProxy`/`StopProxy`/
   `waitHTTP`/`javaBinary`.
4. [x] `tui/internal/devtool/proxy_test.go`.
5. [x] `tui/cmd/devtool/main.go`.
6. [x] Taskfile: `test:queue:add`/`remove`, `dev:proxy:start`/`stop`.
7. [x] `mq-proxy/.gitignore`: ignore `devtool.pid`/`devtool.log`.
8. [x] `tui/CLAUDE.md`: package layout + Testing section pointing at the
   skill.
9. [x] `.claude/skills/verify-live/SKILL.md`.
10. [x] `go build ./...`, `go vet ./...`, `go test ./...`.
11. [x] Manual verification: `test:queue:add`/`remove` against the real
    broker; `dev:proxy:start`/`stop` end-to-end. Along the way, found and
    fixed two real bugs this exercise was meant to catch:
    - `exec.Command("java", ...)` ignored `$JAVA_HOME`, so `start-proxy`
      failed with `UnsupportedClassVersionError` under sdkman's default
      Java 17 (mq-proxy now needs 21+) → added `javaBinary()`.
    - A failed `start-proxy` left a stale `devtool.pid` behind → added
      cleanup on the `waitHTTP` failure path.
