# Plan — FE 26: golden-path smoke-test script

## `tui/internal/devtool/config.go`

```go
func AddProxyConnection(cfg config.Config, name, alias, url, username, password string) (config.Config, error)
```

Pure — takes and returns a `config.Config`, doesn't touch disk. Errors on a
duplicate name (matching the connection editor's own uniqueness rule).
`cmd/devtool`'s `add-proxy-conn` subcommand is the thin
`LoadDefault` → `AddProxyConnection` → `SaveDefault` wrapper.

## `tui/scripts/smoke-test.sh` structure

1. Preconditions: `command -v tmux`/`go`, fail fast with a clear message if
   missing (before touching the broker).
2. `trap cleanup EXIT` registered immediately, so any failure from this
   point on — including `set -e` aborting on an unexpected command
   failure — still cleans up. Order inside `cleanup`, deliberately:
   kill tmux session → **restore `config.yaml` from backup** → remove
   disposable queues → stop `mq-proxy` → remove the built binary. (Backup
   restore must precede queue removal — see spec.md's "found live" note.)
3. Build the binary to a `mktemp` path.
4. Create two disposable queues (`add-queue`), PID-suffixed.
5. Back up `config.yaml` if present; count existing connections (for
   navigating the connection manager list later — the new entry is always
   appended last, at index `EXISTING_CONN_COUNT`).
6. `start-proxy`, then `add-proxy-conn` to register the test connection.
7. Launch the TUI in tmux; drive it through the golden path using two
   helpers:
   - `wait_for(needle, timeout)` — polls `capture-pane` every 200ms rather
     than a fixed sleep, per the general `run` skill's guidance; returns
     the matching capture or fails loudly with the last screen on timeout.
   - `filter_queues(needle)` — the queues list's `/` restores the
     *previous* filter text (`queues.go`: `filterInput.SetText(qv.filter)`)
     rather than starting blank, so this explicitly backspaces first
     rather than assuming an empty field. (The move-picker's search field
     doesn't have this issue — `showMovePicker` clears it on every open —
     so plain `/` + text is fine there.)
8. Connection-manager navigation: no filter/search exists there at all, so
   getting to the newly-added connection is `Down` × `EXISTING_CONN_COUNT`
   from the top (cursor resets to item 0 each time the manager opens, and
   the new entry is always last since it was appended).
9. Assertions are plain substring checks on captured text (pending counts,
   status-bar messages like `"Moved 2 message(s)"`), not screen-position-
   sensitive — resilient to layout changes, not to wording changes (that's
   an accepted tradeoff for a script whose whole point is human-readable
   failure output).

## Testing

`AddProxyConnection`: unit tested (append behavior, duplicate-name
rejection) — no file I/O in the test, matching the "keep it pure" design.

The script itself has no automated test (it *is* the test) — verified by
actually running it: twice back-to-back for idempotency, and once with
`JAVA_HOME` unset to exercise the failure/cleanup path. See spec.md,
Definition of done.

## No new dependencies

`tmux` was already a prerequisite (via the `verify-live` skill); no new
external tools introduced.
