# Plan — CR 70: relocate 4 `App` methods into `host.go`

## Approach

Cut each method verbatim from its current file and paste it into
`host.go`, grouped with the other `Host`-implementing methods (after
`FocusMessages()`, the current last method). No signature, body, or
receiver changes — this is a pure file move.

Methods moving (already fully specified — see `connections.go`/
`datadogsettings.go`/`awsprofiles.go` for their exact current bodies,
unchanged by this CR):

- `SaveConnection(conn config.Connection, origName string, isNew bool)`
  — from `connections.go`
- `DeleteConnection(name string) (wasActive bool)` — from `connections.go`
- `SaveDatadogConfig(cfg config.DatadogConfig)` — from `datadogsettings.go`
- `SetActiveAWSProfile(name string)` — from `awsprofiles.go`

`host.go` already imports `config`; `queue` isn't needed by these four
(no new imports required). Check after the move whether
`connections.go`, `datadogsettings.go`, `awsprofiles.go` still need
every import they currently have — `SaveConnection`/`DeleteConnection`
were the only `slog.Error`/`config.SaveDefault` users in some of these
files' App-method sections, so an unused-import compile error is the
expected signal if a file's import list needs trimming, not something
to pre-guess.

## Files touched

- `internal/app/host.go` (+4 methods)
- `internal/app/connections.go` (−2 methods, possible import trim)
- `internal/app/datadogsettings.go` (−1 method, possible import trim)
- `internal/app/awsprofiles.go` (−1 method, possible import trim)

## Key decisions

- **Grouped at the end of `host.go`, not interleaved by topic** —
  matches how the file already reads top-to-bottom as "everything
  `App` does to satisfy `Host`," not organized by which overlay calls
  what; imposing a different order for just these 4 would be
  inconsistent with the file's existing shape.
- **No new tests** — pure motion of already-tested logic (CR 66
  live-verified all four originally).
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test` pass, all four methods
live in `host.go`, none of the three donor files defines an `(a *App)`
method anymore.
