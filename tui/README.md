# tui

Go terminal UI (tview/tcell), k9s-style: a command prompt (`:secrets`,
`:params`, `:queues`, `:q`) switches between resource views. See root
`CLAUDE.md` for the agreed architecture.

## Layout

| Path                    | Description                                      |
|-------------------------|---------------------------------------------------|
| `cmd/tui/`              | Entrypoint (`main.go`)                            |
| `internal/app/`         | App shell: header, command prompt, page routing   |
| `internal/ui/`          | `View` interface shared by resource views          |
| `internal/ui/views/`    | Resource views (secrets, params, queues)          |

## Run

From the repo root: `task run:tui`, `task build:tui`, or `task test:tui`.
