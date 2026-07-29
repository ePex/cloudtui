# Spec 03 — Debug logging

Date: 2026-07-30

## What

Add a lightweight, persistent log file and an in-app log viewer so that
startup problems are diagnosable and the app's runtime behaviour is
observable without attaching a debugger.

### Log file

On startup `cmd/tui/main.go` initialises a `log/slog` text-format logger
writing to `~/.cloudtui/cloudtui.log`. The directory and file are created if
absent; the file is truncated on each startup so it never grows unboundedly.
`log/slog` is part of the Go standard library since Go 1.21 — no new
dependency.

**What is logged (startup only):**
- Config file path that was loaded
- Active theme

### `:log` view

`internal/app/log.go` implements `logView`: a read-only scrollable
`tview.TextView` that reads and displays `~/.cloudtui/cloudtui.log`. It is a
standard registered view: home dashboard entry, reachable via `:log`, and
`Activate()` reloads the file each time the view becomes active. Shortcut `r`
manually refreshes. If the log file is absent a short message is shown instead
of an error.

### `l` global hotkey

Pressing `l` from anywhere (when the prompt/filter is not focused) switches to
the log view, consistent with `h` → home and `s` → settings.

## Why

Currently there is no persistent record of what the app does on startup. If
something goes wrong — config not found, wrong file loaded, unexpected theme —
the only feedback is whatever appears briefly in the status bar. A log file
gives a durable, inspectable record of startup state, and the `:log` view
surfaces it without leaving the TUI.

## Scope

- `cmd/tui/main.go` — logger init: `log/slog` text handler, output to
  `~/.cloudtui/cloudtui.log`; create dir+file, truncate on startup; write
  startup entries (config file path, active theme) before `app.New()`.
- `tui/internal/app/log.go` — `logView` implementing `ui.View`,
  `ui.Shortcuttable`, and `activatable`; scrollable read-only `tview.TextView`;
  `Activate()` and shortcut `r` reload the file; graceful fallback when file
  is absent.
- `internal/app/app.go` — register `logView` as the `"log"` view; add `"log"`
  to the home dashboard; add `l` global hotkey.
- `internal/app/statusbar.go` — add `l: Log` to the idle hotkey legend.
- `internal/app/help.go` — add `l → log` to the help modal.
- `tui/internal/app/log_test.go` — tests: `Name()`, implements
  `Shortcuttable`, shortcut key `r` present, `Activate()` with missing file
  shows fallback message.
- `.gitignore` — add `cloudtui.log` (or `~/.cloudtui/`) so the log file is
  never accidentally committed.

## Out of scope

- Logging of any runtime operations beyond startup — future additions per
  feature need.
- Log level filtering or structured JSON output.
- Log rotation beyond truncation on startup.
- In-app log filtering or search.
