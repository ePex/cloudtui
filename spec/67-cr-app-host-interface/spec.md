# Spec — CR 67: declare the `Host` interface

Date: 2026-08-16

## Background

Second slice of phase 3 of the `internal/app` package split
(`spec/64-cr-app-package-split/spec.md`). CR 66 extracted four
config-mutating overlay methods into named `App` methods
(`SaveConnection`, `DeleteConnection`, `SaveDatadogConfig`,
`SetActiveAWSProfile`), on the reasoning that a stable, task-shaped
method beats designing an interface in the abstract before writing any
code against it. Combined with two full file re-reads done for this CR
(`sendmessage.go`, `messagefilter.go`, plus a re-check of
`movepicker.go`), there's now a complete, grounded picture of
everything the 10 overlays touch on `*App`.

## Problem

Moving any overlay into a new `internal/dialog` package needs `*App`
satisfied through an interface (an import cycle otherwise — see CR 64/
66's background for why). That interface hasn't been declared yet.

## Solution

Declare `ui.Host` in `internal/ui` (joining `View`/`Shortcuttable`/
`Themeable` as the shell↔view/overlay contracts already living there).
Add `var _ ui.Host = (*App)(nil)` to `app.go` to prove `*App` satisfies
it. **No overlay changes in this CR** — they keep using `app *App`
directly; only `App` gains the (mostly new, some already-existing)
exported methods the interface requires. The actual field-type swap
(`app *App` → `host ui.Host`) is CR 68.

### Method inventory, grouped by why each overlay needs it

| Method | Already exists on `App`? | Needed by |
|---|---|---|
| `ShowPage(name string)` | No — wraps `a.rootPages.ShowPage` | All 10 |
| `HidePage(name string)` | No — wraps `a.rootPages.HidePage` | All 10 |
| `SetFocus(p tview.Primitive)` | No — wraps `a.tv.SetFocus` | All 10 |
| `FocusMain()` | No — wraps the common `a.tv.SetFocus(a.pages)` fallback | confirm, movePicker, connEditor, messageFilter, timeRangeModal, datadogEditor, themePicker, awsProfilesPicker |
| `QueueUpdateDraw(f func())` | No — wraps `a.tv.QueueUpdateDraw` | movePicker, sendMessageOverlay |
| `SetStatus(text string)` | No — wraps `a.statusBar.SetText` | connManager, connEditor, sendMessageOverlay, messageFilter, timeRangeModal, awsProfilesPicker |
| `SetContextHint(text string)` | No — wraps `a.contextPanel.SetText` | movePicker, sendMessageOverlay |
| `Config() config.Config` | No — read-only snapshot (covers `Colors`, `Theme`, `Connections` for display) | movePicker, sendMessageOverlay, timeRangeModal, themePicker, connEditor (dropdown styling) |
| `SwitchTheme(name string)` | Yes (`switchTheme`, unexported) | themePicker |
| `SwitchConnection(name string)` | Yes (`switchConnection`, unexported) | connManager (direct-activate path, separate from `DeleteConnection`'s internal reuse) |
| `SaveConnection(...)` | Yes (CR 66) | connEditor |
| `DeleteConnection(...)` | Yes (CR 66) | connManager |
| `SaveDatadogConfig(...)` | Yes (CR 66) | datadogEditor |
| `SetActiveAWSProfile(...)` | Yes (CR 66) | awsProfilesPicker |
| `ListAWSProfiles(ctx) ([]awsprofile.Profile, error)` | No — wraps the `a.listAWSProfiles` func field | awsProfilesPicker |
| `Backend() queue.Backend` | No — wraps `a.backend` | movePicker, sendMessageOverlay |
| `ReloadAfterSend(queueName string)` | No — new; extracts `sendMessageOverlay.doSend`'s post-send reload (`queuesV.load()`, and `messagesV.load()` iff `messagesV.queueName == queueName`) | sendMessageOverlay |
| `MessagesFilter() queue.MessageFilter` | No — wraps `a.messagesV.filter` (read) | messageFilter (prefilling the form on `show`) |
| `ApplyMessagesFilter(f queue.MessageFilter)` | No — new; extracts the identical 3-line sequence (`messagesV.filter = f; .updateTitle(); .load()`) duplicated today in both `messageFilter.apply` and `messageFilter.clear` | messageFilter |
| `FocusMessages()` | No — wraps `a.tv.SetFocus(a.messagesV.table)` | messageFilter (`close`) |

20 methods. Larger than the ~15-16 the original coupling audit
estimated (that estimate predated CR 66's extractions and this CR's two
full re-reads) — still a reasonable size for an internal shell↔dialog
contract with one real implementation, not a sign the approach needs
rethinking.

### Considered and rejected: splitting into multiple smaller interfaces

Go interface segregation (many small interfaces an overlay composes)
is idiomatic when different implementations exist or tests need to
mock a subset. Neither applies here — `*App` is the only implementation
today and for the foreseeable future, and there's no test double
requirement driving a split. One `Host` interface is simpler to hold
(overlays get a single `host` field, not several) for no loss of
clarity, since the method groupings above are documentation, not a
type-level distinction.

## Scope

### In scope

- `internal/ui/host.go` (new file): the `Host` interface, all 20 methods.
- `app.go`: `var _ ui.Host = (*App)(nil)`.
- New thin wrapper methods on `App` for every "No" row in the table
  above (12 new methods) — each just delegates to the existing field/
  unexported method; `ReloadAfterSend` and `ApplyMessagesFilter` are the
  only two with more than one line of body (both already fully
  specified above, extracted from existing overlay code).
- `sendMessageOverlay.doSend`'s post-send block and `messageFilter`'s
  duplicated 3-line filter-apply sequence are simplified to call the
  new `App` methods, **as a side effect of adding them** (same
  reasoning as CR 66: a new method with no caller is dead code) — but
  this is the only overlay-code change in this CR. Overlays still hold
  `app *App`, not `host ui.Host`; they just call two more `a.` methods
  than before, exactly like CR 66 did for the four extracted methods.

### Out of scope

- Changing any overlay's `app *App` field to `host ui.Host`, or any
  other overlay call site beyond the two named above — CR 68.
- Moving any file to `internal/dialog` — CR 69.
- Any behavior change beyond the two dedups named above (which are
  pure code motion, like CR 66's extractions).

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `internal/ui/host.go` exists with all 20 methods; `var _ ui.Host =
   (*App)(nil)` compiles.
3. `sendMessageOverlay.doSend` and `messageFilter.apply`/`clear` call
   the new `App` methods instead of inlining the logic — verified live
   (`verify-live` skill: send a message and confirm the queues/messages
   views refresh; apply and clear a message filter and confirm the
   table updates both times) since both paths touch real broker state.
4. No behavior change anywhere else — every other new method is an
   unused, uncalled wrapper at the end of this CR (proven safe by
   compiling; nothing calls it yet, so nothing can regress).
