# 2026-07-07 — tui app scaffold

Commit: `feat(tui): scaffold tview app skeleton with command prompt and placeholder views`

## Change

Initialized the Go module `github.com/ePex/cloudtui/tui` (module path taken
from the `github.com/ePex/cloudtui` origin remote) and added:

- `go.mod` / `go.sum` — deps `github.com/rivo/tview` and
  `github.com/gdamore/tcell/v2`.
- `cmd/tui/main.go` — entrypoint; builds an `*app.App` and runs it, printing
  to stderr and exiting 1 on error.
- `internal/app/app.go` — the shell:
  - `tview.Flex` (rows): a one-line header, a one-line command prompt
    (`tview.InputField`), and a `tview.Pages` area holding the resource
    views.
  - Global key capture focuses the prompt on `:` (k9s-style), mirroring the
    prompt's own `SetDoneFunc`, which on Enter reads the typed command,
    switches `Pages` to the matching view name, or stops the app on
    `q`/`quit`; Escape/any non-Enter key just returns focus to `Pages`.
- `internal/ui/view.go` — the `View` interface (`Name`, `Title`,
  `Primitive`) resource views implement so `app.App` can register and
  switch between them without knowing their concrete types.
- `internal/ui/views/placeholder.go` — a private `placeholder` type
  rendering a bordered `TextView` with a title/description/"not yet
  implemented" note; backs all three views below until each gets a real
  AWS-backed table/detail pane.
- `internal/ui/views/{secrets,params,queues}.go` — `NewSecrets`/`NewParams`/
  `NewQueues` constructors, registered under command names `secrets`,
  `params`, `queues`.

## Why

CLAUDE.md's architecture section specifies `tui/` as Go + tview/tcell,
k9s-style (command prompt, resource views, non-blocking UI), with three
resource domains (Secrets Manager, Parameter Store, Amazon MQ queues). This
change lays down that shell and view contract without touching AWS SDK code
yet — no UI code should make AWS calls directly (that lives in a later
service-wrapper layer), so the placeholders intentionally do nothing but
render.

## Verification

```
cd tui
go build ./...   # clean
go vet ./...      # clean
gofmt -l .        # no output — fully formatted
```

`go run ./cmd/tui` was not run inside the sandboxed shell (no real
terminal/tty available there); building the binary and vet/fmt passing was
used as the substitute check.

## Scope note

Placeholder views only — no AWS SDK wiring, no `QueueBackend` interface, no
`mq-proxy` client yet. Those land when each resource view gets its real
implementation.
