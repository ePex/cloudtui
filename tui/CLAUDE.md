# CLAUDE.md — tui module

Go-specific conventions for `tui/`. Repo-wide rules (workflow gating,
`spec/` conventions, cross-platform constraints) live in the root
`CLAUDE.md` and apply here too; this file only adds what's specific to
this module.

## Style and formatting

- `gofmt`/`goimports` formatting is mandatory; run before every commit.
- Errors are wrapped with context: `fmt.Errorf("...: %w", err)`, never
  discarded or logged-and-swallowed.
- Idiomatic Go naming (MixedCaps, no underscores); avoid package-name
  stutter.
- No package-level mutable state.

## Package layout

- `cmd/cloudtui/` — entrypoint only (`main.go`); no logic beyond wiring.
- `cmd/seedqueue/` — entrypoint only; dev tool that sends sample JSON
  messages to a queue (`task seed:queue -- <queue> <count>`).
- `cmd/devtool/` — entrypoint only; dev tool for live-testing setup: create/
  remove disposable queues via JMX, start/stop a local mq-proxy instance
  (`task test:queue:add`/`remove`, `task dev:proxy:start`/`stop`).
- `internal/app/` — the application shell: layout, global hotkeys, view routing.
- `internal/dialog/` — modal overlay types (confirm, connection
  manager/editor, message filter, time range, Datadog/theme/AWS
  profile pickers) implementing internal/ui's Host contract.
- `internal/ui/` — the `View`/`Host`/`ViewHost` interfaces shared across
  resource views.
- `internal/ui/views/` — the home dashboard's table rendering (`home.go`);
  not to be confused with `internal/view/`.
- `internal/view/` — the resource views themselves (queues, messages,
  SSM parameters, Secrets Manager, CloudWatch/Datadog logs, CodePipeline,
  Settings, Log, and each one's detail view), each depending on
  `ui.ViewHost` rather than `internal/app`'s concrete `*App`.
- `internal/queue/` — `Backend` interface and `Summary` type for queue data sources.
- `internal/queue/jolokia/` — Jolokia HTTP client implementing `queue.Backend`.
- `internal/queue/secretbackend/` — `queue.Backend` decorator that
  resolves a connection's password from AWS Secrets Manager on first
  use, caching and transparently re-resolving on a stale/rotated
  secret (see `spec/56-fe-amq-connection-aws-secret-password`).
- `internal/seed/` — sample JSON message generation, used by `cmd/seedqueue`.
- `internal/devtool/` — JMX queue admin and mq-proxy process management,
  used by `cmd/devtool`.
- `internal/awsprofile/` — read-only discovery of AWS CLI profiles from
  `~/.aws/config`/`~/.aws/credentials` (via `aws-sdk-go-v2/config`), shown
  in Settings → AWS Profiles. No credential resolution, no AWS API calls.

## Testing

- Standard library `testing` only — no assertion library. Table-driven
  tests where a function has multiple cases; `t.Helper()` on test
  helpers; `t.TempDir()`/`t.Setenv()` for filesystem/env-dependent tests.
- One `_test.go` file per source file, same package (no separate `_test`
  package), colocated in the same directory.
- If something is genuinely untestable, say so explicitly in the test file
  or the relevant spec's `plan.md`, and verify manually instead.
- For changes touching queue/message/connection behavior, "verify manually"
  means driving the real TUI against a real broker, not just reading the
  code — use the `verify-live` skill (`.claude/skills/verify-live/`), which
  also covers `task seed:queue`, `task test:queue:add`/`remove`, and
  `task dev:proxy:start`/`stop`. Record what you checked in the feature's
  `tasks.md`.
- `task smoke:test` (`tui/scripts/smoke-test.sh`) automates the golden path
  (list/browse/mark/delete/move messages, switch to the proxy backend) as a
  quick regression check — run it after changes likely to affect that path,
  but it doesn't replace verifying a *new* feature's specific behavior by
  hand (see the `verify-live` skill for why).

## Dependencies

- Currently: `tview`/`tcell` (UI), `aws-sdk-go-v2/config` (AWS shared
  config/credentials file parsing — justification in
  `spec/28-fe-aws-profile-discovery/plan.md`).
- Justify any new dependency in the relevant spec's `plan.md` before
  adding it to `go.mod`.
