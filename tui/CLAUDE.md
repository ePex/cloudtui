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

- `cmd/tui/` — entrypoint only (`main.go`); no logic beyond wiring.
- `internal/app/` — the application shell: layout, global hotkeys, view routing.
- `internal/ui/` — the `View` interface shared across resource views.
- `internal/ui/views/` — individual resource view implementations.

## Testing

- Standard library `testing` only — no assertion library. Table-driven
  tests where a function has multiple cases; `t.Helper()` on test
  helpers; `t.TempDir()`/`t.Setenv()` for filesystem/env-dependent tests.
- One `_test.go` file per source file, same package (no separate `_test`
  package), colocated in the same directory.
- If something is genuinely untestable, say so explicitly in the test file
  or the relevant spec's `plan.md`, and verify manually instead.

## Dependencies

- Currently: `tview`/`tcell` (UI).
- Justify any new dependency in the relevant spec's `plan.md` before
  adding it to `go.mod`.
