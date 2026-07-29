# Plan — Debug logging

Spec: [spec.md](spec.md)

## Approach

Two self-contained pieces, implemented in order:

1. **Logger init** — thin wiring in `cmd/tui/main.go`. Open (or create)
   `~/.cloudtui/cloudtui.log` with `O_TRUNC` so previous runs are replaced,
   then pass the file as the `io.Writer` for a `log/slog` text handler set as
   the global default. If the file cannot be created (missing home dir,
   permissions) the handler falls back to `io.Discard` so the TUI is never
   affected by log setup failures. Startup entries (config path, theme) are
   written immediately after `config.LoadDefault()` returns, before
   `app.New()`.

2. **Log view** — `internal/app/log.go`, same package as the other app-level
   views (`settings`, `queues`). Implements `ui.View`, `ui.Shortcuttable`, and
   `activatable`. The underlying primitive is a `tview.TextView` (scrollable,
   no word-wrap, no dynamic colours so log text is not misinterpreted as colour
   tags). `load()` does a plain `os.ReadFile` on the log path and sets the
   result as the text view content; missing file → fallback message, other
   errors → error message. The path is injected at construction time via
   `newLogViewWithPath` so tests can point at a temp file without touching
   `~/.cloudtui/`.

## Files touched

| File | Change |
|------|--------|
| `cmd/tui/main.go` | `openLogFile()` helper; `slog.SetDefault`; startup log entries |
| `internal/app/log.go` | new — `logView`, `newLogView`, `newLogViewWithPath`, `Activate`, `load` |
| `internal/app/log_test.go` | new — 4 unit tests |
| `internal/app/app.go` | `logV` field; home dashboard entry; registration; `l` global hotkey |
| `internal/app/theme.go` | log view background/border/title on theme switch |
| `.gitignore` | `cloudtui.log` entry |

## Key decisions

- **`io.Discard` fallback** — log setup errors are silent; the TUI must always
  start cleanly regardless of filesystem state.
- **Truncate on startup** — keeps the file small without requiring a rotation
  library. One run = one log file.
- **`log/slog` stdlib** — no new dependency; available since Go 1.21, which is
  already the project's minimum.
- **`newLogViewWithPath`** — lets tests inject a temp-file path without any
  env-var or global state, consistent with the pattern used in
  `awsprofile_test.go`.
- **No test for `main.go` file-creation logic** — it is pure OS wiring with no
  branching logic worth unit-testing; verified manually by running the app.
- **No palette entry for `"log"`** — the log view border falls back to the
  palette's `Border` colour, which is appropriate for an informational/utility
  view. A colour override can be added in a future change-request if desired.
