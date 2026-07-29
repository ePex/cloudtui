# Tasks — Debug logging

Plan: [spec.md](spec.md)

1. [x] **Logger init** — in `cmd/tui/main.go`: initialise a `log/slog`
   text-format logger writing to `~/.cloudtui/cloudtui.log` (create dir +
   file, truncate on startup). Write startup log entries before `app.New()`:
   config file path loaded, active theme. Add `cloudtui.log` (or
   `~/.cloudtui/`) to `.gitignore`.

2. [x] **Log view** — `tui/internal/app/log.go`: `logView` implementing
   `ui.View`, `ui.Shortcuttable`, and `activatable`; a scrollable read-only
   `tview.TextView` that reads and displays `~/.cloudtui/cloudtui.log` on
   `Activate()` and on shortcut `r`; shows a graceful message if the file is
   absent. Register as the `"log"` view in `app.go`: home dashboard entry,
   `:log` command (via existing `switchTo` default), `l` global hotkey.
   `log_test.go`: `Name()`, implements `Shortcuttable`, shortcut key `r`
   present, `Activate()` with missing file shows fallback.
