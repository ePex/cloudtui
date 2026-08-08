# Tasks — FE 26: golden-path smoke-test script

Plan: [plan.md](plan.md)

1. [x] `tui/internal/devtool/config.go` — `AddProxyConnection`.
2. [x] `tui/internal/devtool/config_test.go`.
3. [x] `cmd/devtool`: `add-proxy-conn` subcommand.
4. [x] `tui/scripts/smoke-test.sh` (executable).
5. [x] `Taskfile.yml`: `smoke:test`.
6. [x] `.claude/skills/verify-live/SKILL.md` and `tui/CLAUDE.md`: reference
   the script.
7. [x] `go build ./...`, `go vet ./...`, `go test ./...`.
8. [x] Ran the script live — first attempt revealed a real cleanup-ordering
   bug (queues survived because `config.yaml` hadn't been restored yet
   when `remove-queue` ran, so it was still pointed at the proxy
   connection and devtool's jolokia-only guard silently rejected the
   removal). Fixed the ordering in `cleanup()`.
9. [x] Re-verified after the fix: ran twice back-to-back (idempotency) and
   confirmed via direct broker queries (not just the script's own log
   output) that both queues were actually gone and `config.yaml` was
   byte-identical to the pre-run backup both times.
10. [x] Verified the failure path: unset `JAVA_HOME` to force `start-proxy`
    to fail, confirmed the script reported FAIL with a non-zero exit code
    and still fully cleaned up (queues removed, config restored, no stale
    `devtool.pid`).
