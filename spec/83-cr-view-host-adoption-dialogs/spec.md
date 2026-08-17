# Spec — CR 83: adopt `ui.ViewHost` in the 5 dialog-coupled view types

Date: 2026-08-17

## Background

CR 82 adopted `ui.ViewHost` in the 9 dialog-free view types. This CR
covers the remaining 5, flagged back in CR 82's own audit as needing
more than a mechanical rename: `queues.go`, `messages.go`,
`message_detail.go`, `logsearch.go`, `datadoglogs.go` each call a
dialog's `Show(...)` directly (`.confirm`, `.movePicker`,
`.sendMessage`, `.messageFilter`, `.timeRangeModal`). Per CR 80's
design decision, `ui.ViewHost` deliberately doesn't expose dialog
access — once views move to `internal/view` they'll import
`internal/dialog` directly and take the specific dialog instances they
need as constructor parameters, the same pattern `ConnEditor` already
uses for its `ConnManager` sibling reference
(`NewConnEditor(host ui.Host, manager *ConnManager)`).

Read all 5 files in full (not just grepped `.app.`) to ground the
scope precisely:

**1. Raw field/method access identical in shape to CR 82's finding** —
these files reach `.tv`, `.cfg`, `.contextPanel`, `.statusBar`,
`.backend`, and (new to this batch) `.pages` and 3 lowercase func
fields, instead of the exported `ui.ViewHost`/`ui.Host` equivalents
CR 80 already added:

| Raw access today | `ui.ViewHost`/`ui.Host` equivalent |
|---|---|
| `.app.tv.SetFocus(p)` | `.host.SetFocus(p)` |
| `.app.tv.QueueUpdateDraw(f)` | `.host.QueueUpdateDraw(f)` |
| `.app.cfg` (`.Colors`, `.ActiveAWSProfile`, `.Datadog`, ...) | `.host.Config()` |
| `.app.contextPanel.SetText(text)` | `.host.SetContextHint(text)` |
| `.app.statusBar.SetText(text)` | `.host.SetStatus(text)` |
| `.app.pages.SwitchToPage(name)` | `.host.SwitchToPage(name)` |
| `.app.backend` (bare field) | `.host.Backend()` |
| `.app.filterLogEvents(...)` | `.host.FilterLogEvents(...)` |
| `.app.searchDatadogLogs(...)` | `.host.SearchDatadogLogs(...)` |
| `.app.listDatadogFacetValues(...)` | `.host.ListDatadogFacetValues(...)` |

`messages.go`'s Esc handler already calls the exported `a.SwitchTo("queues")`
— no change needed there, same as CR 82's 6 already-exported symbols.

**2. Dialog access — the reason this is its own CR.** Five call
sites reach a dialog through `*App` directly:

| File | Dialogs called |
|---|---|
| `queues.go` | `.confirm`, `.movePicker`, `.sendMessage` |
| `messages.go` | `.messageFilter`, `.sendMessage`, `.confirm`, `.movePicker` |
| `message_detail.go` | `.movePicker`, `.confirm` |
| `logsearch.go` | `.timeRangeModal` |
| `datadoglogs.go` | `.timeRangeModal` |

None of these become `ui.ViewHost` methods (CR 80's decision). Instead
each constructor gains one field per dialog it actually uses, typed
exactly like `ConnEditor`'s `manager *ConnManager` — e.g.
`newQueuesView(host ui.ViewHost, b queue.Backend, confirm *dialog.ConfirmDialog, movePicker *dialog.MovePicker, sendMessage *dialog.SendMessageOverlay, onSelect func(string))`.
`app.go` already holds constructed `*dialog.ConfirmDialog` etc. on
`a.confirm`/`a.movePicker`/... — those become the arguments passed in.

**Consequence: construction order in `app.go`'s `New()` has to
flip.** Today the 5 dialogs (`a.confirm`, `a.movePicker`,
`a.sendMessage`, `a.messageFilter`, `a.timeRangeModal`) are
constructed *after* all 14 views, since nothing used to depend on them
at construction time. Once these 5 views take dialog pointers as
constructor arguments, the dialogs must exist first. None of the 5
dialogs themselves depend on any view (each only takes `a` as its
`ui.Host`, or another already-built dialog — same as
`ConnManager`/`ConnEditor` today), so moving just their 5 `a.X =
dialog.NewX(...)` construction lines earlier (before `a.queuesV =
...`) is safe; the `ui.Centered(...)`/`a.rootPages.AddPage(...)`
overlay-registration lines that currently follow each one can stay
exactly where they are.

**3. Sibling-view reaches (the `onBack` pattern from CR 82), pointed
at 2 of these 5 files:**

- `message_detail.go` has the *same* three-line "return to messages"
  body repeated at 3 call sites — the `'m'` (move) success handler,
  the `'d'` (delete) success handler, and the Esc handler:
  ```go
  a.pages.SwitchToPage("messages")
  a.tv.SetFocus(a.messagesV.table)
  lines := make([]string, 0, len(a.messagesV.Shortcuts()))
  for _, sc := range a.messagesV.Shortcuts() {
      lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
  }
  a.contextPanel.SetText(strings.Join(lines, "\n"))
  ```
  (manual `Shortcuts()` rebuild, not `UpdateContextPanel`, because
  `messagesView` isn't a registered `ui.View` — same reason
  CR 82's `logdetail.go` needed the manual form.) The `'m'`/`'d'`
  handlers additionally call `a.messagesV.load()` right after this
  block; the Esc handler doesn't. Two callbacks cover this exactly,
  with no invented behavior: `onBack func()` (the shared 3-line body,
  called from all 3 sites) and `onReload func()` (just
  `a.messagesV.load()`, called from the `'m'`/`'d'` sites only, after
  `onBack()`).
- `logsearch.go`'s Esc handler has the single-occurrence version —
  `a.pages.SwitchToPage("cloudwatch-logs"); a.tv.SetFocus(a.logsV.table);
  a.UpdateContextPanel(a.logsV)`. `logsV` *is* a registered `ui.View`
  (unlike `logSearchView` itself), so this one can use
  `UpdateContextPanel` directly — no manual rebuild needed. One
  `onBack func()` parameter, same as CR 82's simple cases.

`queues.go`, `datadoglogs.go`, and `messages.go` need no `onBack`:
`queues.go`/`datadoglogs.go` are top-level registered views with no
Esc-back at all; `messages.go`'s Esc already goes through the
already-`ViewHost`-safe `a.SwitchTo("queues")`.

## Problem

Once these 5 files depend on `ui.ViewHost` instead of `*App`, none of
`.tv`/`.cfg`/`.contextPanel`/`.statusBar`/`.pages`/`.backend`, the 3
lowercase func fields, a dialog field, or a sibling view's fields are
reachable — they're unexported members of a concrete type (or a
different concrete type) the interface deliberately doesn't expose.

## Solution

For each of the 5 files:

1. Struct field `app *App` → `host ui.ViewHost` (matches CR 82's
   convention).
2. Constructor parameter `a *App` → `a ui.ViewHost`, plus one new
   parameter per dialog the file calls (see table above), typed as
   the concrete `*dialog.X`.
3. Every raw call site updated per the rename table above.
4. Every dialog call site (`a.confirm.Show(...)` etc.) updated to use
   the new constructor-injected field instead of reaching through
   `host`/`app`.
5. `message_detail.go`'s constructor gains `onBack func()` and
   `onReload func()`; its 3 sibling-reaching blocks replaced with
   `onBack()` (all 3) followed by `onReload()` (the 2 success-handler
   sites only). `logsearch.go`'s constructor gains `onBack func()`;
   its single Esc-handler block replaced with `onBack()`.
6. `app.go`: the 5 dialogs' construction lines (`a.confirm =
   dialog.NewConfirmDialog(a)` etc.) move earlier, before the 5
   views' construction calls; each of the 5 views' construction
   calls passes the now-already-built dialog pointers plus (for the
   2 detail/search cases) the matching `onBack`/`onReload` closures.

## Scope

### In scope

- `queues.go`, `messages.go`, `message_detail.go`, `logsearch.go`,
  `datadoglogs.go`: field/parameter type swap, dialog fields added,
  all call sites updated per the tables above.
- `message_detail.go`'s `onBack`/`onReload` addition and
  `logsearch.go`'s `onBack` addition, plus `app.go`'s matching
  construction-call updates.
- `app.go`: reordering the 5 dialogs' construction ahead of these 5
  views' construction.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.

### Out of scope

- The physical move of any view or dialog file into `internal/view` —
  later, once all 14 view files depend on `ui.ViewHost`.
- Any behavior change — every substitution above is a same-behavior
  swap; the `onBack`/`onReload` split preserves the exact existing
  call pattern (no site gains or loses a reload it didn't already
  have).

### Live verification

Touches real dialog flows (purge, move, send-message, message filter,
time range) and Esc-back navigation from `message_detail.go` and
`logsearch.go` — worth a `verify-live` pass covering: purge a queue,
move a queue's messages, send a test message, filter messages, move/
delete a single message from its detail view (confirming Esc *and*
successful move/delete both land back on the messages list with a
reload), CloudWatch log search's time-range picker, and Esc from log
search back to the CloudWatch Logs list.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All 5 files hold `host ui.ViewHost` plus the specific `*dialog.X`
   fields they use, instead of `app *App`; zero remaining raw-field,
   unexported-func-field, or through-`app`-dialog access.
3. `message_detail.go` and `logsearch.go`'s Esc handlers call
   `onBack()` (plus `onReload()` where applicable) instead of reaching
   into a sibling view.
4. `app.go`'s dialog construction happens before the 5 views that now
   depend on it; all 5 construction call sites pass the right
   dialog/callback arguments.
5. `gofmt -l` reports nothing; `go vet ./...` clean.
6. Dialog flows and Esc-back live-verified per the Live verification
   section above.
7. No behavior change.
