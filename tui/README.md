# tui

Go terminal UI using tview/tcell. See root `CLAUDE.md` for repository conventions.

## Layout

| Path             | Description                        |
|------------------|------------------------------------|
| `cmd/cloudtui/`  | Entrypoint (`main.go`)             |
| `internal/app/`  | Application shell                  |
| `internal/ui/`   | View interface and implementations |

## Run

From the repo root: `task run:tui`, `task build:tui`, or `task test:tui`.
