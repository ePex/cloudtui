# Debug logging

_Condensed from spec/03-fe-logging — see that folder for the incremental history._

## Purpose

A persistent, inspectable log so startup problems are diagnosable and the app's runtime behavior is observable without attaching a debugger.

## Behavior

### Log file

On startup, the app initializes a `log/slog` text-format logger writing to `~/.cloudtui/cloudtui.log`. The directory and file are created if absent; the file is **truncated on each startup** so it never grows unboundedly (no rotation beyond that). `log/slog` is Go standard library (Go 1.21+) — no external dependency.

What is logged at startup: the config file path that was loaded, and the active theme. (Runtime logging beyond startup is added incrementally per-feature as needed, not blanket-covered by this spec.)

### `:log` view

A read-only, scrollable text view that reads and displays `~/.cloudtui/cloudtui.log`. It is a standard registered view: reachable from Home's "System" section and via `:log`, with `l` as its global hotkey. `Activate()` reloads the file every time the view becomes active; `r` manually refreshes. If the log file is absent, a short fallback message is shown instead of an error.

Lines are colorized by slog level: `level=ERROR` red, `level=WARN` yellow, `level=INFO` aqua/cyan (tcell has no separate "cyan" tag name; aqua is the same `0x00FFFF` value). Unrecognized lines (e.g. `DEBUG`, or a wrapped continuation line) keep the default text color. Each line is passed through `tview.Escape` before rendering, so a literal `[` in logged content (e.g. a Go slice's `%v` formatting) is never misread as a `tview` color tag.

## Data & config

- Log file path: `~/.cloudtui/cloudtui.log` (directory `~/.cloudtui/` created if missing).
- `.gitignore` excludes the log file/directory — it must never be committed.

## Implementation notes

- Logger initialization lives in `cmd/tui/main.go`, before `app.New()` is called (so applying the theme and building the shell are themselves covered by the startup log entries).
- `tui/internal/view/log.go` — `LogView`, implementing `ui.View`, `ui.Shortcuttable`, and an `activatable` interface (`Activate()` reload-on-entry). Originally lived at `internal/app/log.go`; moved into `internal/view` as part of the later package split (see spec/03-architecture-and-package-layout).
- Global hotkey `l` and the `l: Log` status-bar/help-modal entries are wired in the shell composition root (`internal/app/app.go`) and the help modal.

## Notable gotchas worth preserving

- Always run log lines through `tview.Escape` before display — unescaped `[...]`-shaped content in a logged value (a Go slice, a JSON array literal, etc.) will otherwise be interpreted as a tview color/region tag and corrupt the rendered line.
