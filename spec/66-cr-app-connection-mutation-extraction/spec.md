# Spec — CR 66: extract config-mutating logic out of overlay `save`/`delete`/`activate` methods

Date: 2026-08-16

## Background

This is the first slice of phase 3 of the `internal/app` package split
(`spec/64-cr-app-package-split/spec.md`) — moving the ~10 modal overlays
into a new `internal/dialog` package. CR 64's original phase 3 line
undersold the work: an audit done while starting this CR found that
**every one of the 10 overlay types** holds an `app *App` field and
calls `a.rootPages`/`a.tv` directly (not just the 3 originally
flagged), and half of them (`connManager`, `connEditor`, `datadogEditor`,
`awsProfilesPicker`) mutate `a.cfg` fields and call `config.SaveDefault`
directly inline in their `save`/`delete`/`activate` methods. Moving any
overlay to a new package means `internal/dialog` can no longer hold a
concrete `*App` field — `internal/app` needs to import `internal/dialog`
(for the field types on `App`), so `internal/dialog` importing
`internal/app` back for `*App` would be a cycle. The fix is an interface
(overlays depend on a `Host`-shaped contract, not the concrete `*App`) —
`spec/64`'s table has been corrected to reflect this.

Before designing that interface, four overlay methods were read in
full (`connEditor.save`, `connManager.delete`, `datadogEditor.save`,
`awsProfilesPicker.activate`) to see their actual shape rather than
guess it. All four do the same kind of thing: validate/build a value
from form input (UI concern, stays put), then mutate `a.cfg`, possibly
rebuild the active backend, refresh dependent UI, and persist (`App`
concerns, entangled with UI in one function body today).

`connEditor.save`'s active-connection case is the most involved: if the
connection being edited is the currently active one, it rebuilds the
backend in place (`newBackendForConn(a, conn)`, passing `*App` itself —
the hardest case in the whole audit) and updates `queuesV.backend` +
`infoPanel`. This is a different operation from `switchConnection`
(which activates an already-existing, unedited connection by name) —
but `connManager.delete`'s active-connection case *is* exactly
`switchConnection` (remove the entry, pick a new active name, call the
existing method) — already proving the "wrap a task-shaped App method"
approach works, since that's precisely what `delete` already does today
for its active-connection branch.

## Problem

An interface can express "call this method" cleanly but not "write this
field directly" — and 5 of the 10 overlay types do the latter today
(`a.cfg.Connections =`, `.ActiveConnection =`, `.ActiveAWSProfile =`,
`.Datadog =`, `a.backend =`, `a.queuesV.backend =`). Designing a `Host`
interface against today's shape would mean either exposing raw field
setters (leaks `App`'s internal structure into the interface, one
setter per field forever) or designing the interface in the abstract
before writing any code against it (this repeatedly went wrong in
`spec/64`'s original phase 3 line — better to extract first, formalize
the interface from what the extraction actually produces).

## Solution

Extract the config-mutation + backend-rebuild + persist + refresh logic
out of the four methods into four new, task-shaped `App` methods —
pure code motion, no interface yet, no file moves yet, no behavior
change. Each new method lives in the same file as the overlay it
serves (matching the CR 59–62 precedent of colocating an `App` method
with the feature it belongs to):

| New method | File | Replaces the inline logic in | Reuses |
|---|---|---|---|
| `(a *App) SaveConnection(conn config.Connection, origName string, isNew bool)` | connections.go | `connEditor.save`'s append-or-replace + conditional active-backend rebuild + persist + `refreshSettingsList` | — |
| `(a *App) DeleteConnection(name string) error` | connections.go | `connManager.delete`'s confirmed-callback body (remove from list; if it was active, pick the new active name and call the existing `switchConnection`; else persist) | `switchConnection` |
| `(a *App) SaveDatadogConfig(cfg config.DatadogConfig) error` | datadogsettings.go | `datadogEditor.save`'s set + persist + `refreshSettingsList` | — |
| `(a *App) SetActiveAWSProfile(name string) error` | awsprofiles.go | `awsProfilesPicker.activate`'s set + `infoPanel` update + `refreshSettingsList` + persist | — |

Each overlay method keeps its own part (reading form fields, validating
input, deciding the user-facing status message, closing itself) and
calls the new `App` method for the mutation instead of inlining it.
`SaveConnection`/`DeleteConnection` return an error only where the
original code already had a failure path worth surfacing (`config.SaveDefault`
failing — currently just logged via `slog.Error`, preserved as-is); the
"cannot delete the only connection" / duplicate-name guards stay in the
overlay, since they're about whether to even attempt the operation, not
the operation itself.

## Scope

### In scope

- Four new `App` methods per the table above.
- `connEditor.save`, `connManager.delete`, `datadogEditor.save`,
  `awsProfilesPicker.activate` updated to call them instead of inlining
  the logic — same behavior, same call sites elsewhere untouched.
- Exact method signatures (especially error handling — return `error`
  vs. keep the existing log-and-swallow) finalized in plan.md by
  matching each method's actual current failure handling, not decided
  here.

### Out of scope

- Declaring the actual `Host`/`ui.Host` interface — that's the next CR
  in this sub-sequence, once these four methods (plus the ones already
  implicit today — `switchTheme`, `switchConnection`) give a concrete,
  stable set of signatures to formalize an interface from.
- Changing any overlay's `app *App` field to an interface type, or
  moving any file to a new package — later CRs in this sub-sequence.
- The 3 hardest remaining cases from the original audit
  (`sendMessageOverlay`/`messageFilter` reaching into `messagesV`/
  `queuesV` for `.load()`/`.filter`/`.table`) — different shape of
  problem (reaching into a *view*, not mutating config), handled in a
  separate future CR once phase 4 (views) is also underway, since the
  right fix likely depends on how views end up exposed.
- Any behavior change. Pure code motion.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. The four `App` methods exist, and the four overlay methods call them
   instead of inlining the logic.
3. No behavior change — verified live (`verify-live` skill): edit the
   active connection's settings and confirm the backend/queues view
   updates; delete the active connection and confirm it switches to
   another; delete a non-active connection; save Datadog settings;
   activate a different AWS profile. All five already have real broker/
   AWS-file interaction, so this needs live verification, not just
   `go test`.
