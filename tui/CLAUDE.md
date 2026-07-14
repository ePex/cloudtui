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
  stutter (`queue.Backend`, not `queue.QueueBackend`).
- No package-level mutable state, with one deliberate exception:
  `internal/app/theme.go`'s `applyTheme` mutates the `tview.Styles`
  package var once, at startup, before any primitive is constructed —
  this is tview's own intended theming extension point, not a pattern
  to reuse elsewhere.

## Package layout

- `cmd/tui/` — entrypoint only (`main.go`); no logic beyond wiring.
- `internal/app/` — the shell: layout, global hotkeys, theming, and any
  view that needs live config/backends rather than being a stateless
  placeholder (e.g. `settings`, `queues`).
- `internal/ui/` — the `View`/`Filterable` interfaces shared across
  resource views; deliberately has no dependency on `internal/config`
  or AWS so it stays swappable/testable in isolation.
- `internal/ui/views/` — stateless resource views (placeholders and
  anything that doesn't need live config/backend access).
- `internal/config/` — `Config`/`Palette`/`AWSConfig` schema, load/save.
- `internal/awsprofile/` — local AWS profile discovery, stdlib only.
- `internal/queue/` — the `QueueBackend` interface; `internal/queue/proxy`
  is the `mq-proxy`-backed implementation used both locally and in AWS
  (see root `CLAUDE.md`'s architecture section for why).
- `internal/mqproxyclient/generated/` — generated from `api/openapi.yaml`
  at build time; never hand-edit, never diff-review like normal source.

## AWS access

- All AWS SDK calls live in `internal` service wrappers, never directly
  in `internal/app`/`internal/ui` code, so the UI stays non-blocking and
  testable without real AWS credentials.
- Secret values are masked by default wherever the UI renders them, and
  are never written to logs.

## Testing

- Standard library `testing` only — no assertion library. Table-driven
  tests where a function has multiple cases; `t.Helper()` on test
  helpers; `t.TempDir()`/`t.Setenv()` for filesystem/env-dependent tests
  (see `internal/awsprofile/awsprofile_test.go` for the pattern).
- One `_test.go` file per source file, same package (no separate `_test`
  package), colocated in the same directory.
- If something's genuinely untestable through the normal API (e.g.
  `styleList`'s effect on `tview.List`, which exposes setters but no
  getters for the resulting style), say so explicitly in the test file
  or the relevant spec's `plan.md`, and verify manually instead —
  per root `CLAUDE.md`'s testing rule, this has to be stated, not
  silently skipped.

## Dependencies

- Currently: `tview`/`tcell` (UI), `gopkg.in/yaml.v3` (config, since the
  standard library has no YAML support), generated client code from
  `api/openapi.yaml`. No `aws-sdk-go-v2` yet — it arrives with the first
  feature that makes a real AWS call.
- Justify any new dependency in the relevant spec's `plan.md` before
  adding it to `go.mod`.
