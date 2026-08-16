# Spec — CR 59: extract App overlay state into dedicated structs

Date: 2026-08-16

## Background

`internal/app/app.go` is 1390 lines, and `App` (`app.go:34`) has over 100
fields. Some of that is legitimately shared shell state (the tview
application/pages, the active backend, cross-cutting async-call hooks like
`revealSecret`/`listPipelines` used by several views for testability). But
a lot of it is state that belongs to one specific overlay or feature —
confirm dialogs, the move-queue picker, the send-message overlay, the
connection manager/editor, the message filter form, the time-range modal,
the AWS profiles picker — all living as flat fields directly on `App`,
with their logic as `(a *App)` methods in `app.go` or a same-package file.

This isn't the only pattern in the codebase, though — 15 views already
don't do this: `queuesView`, `messagesView`, `messageDetailView`,
`settingsView`, `ssmParamsView`, `secretsView`, `paramDetailView`,
`secretDetailView`, `logsView`, `logSearchView`, `logDetailView`,
`datadogLogsView`, `datadogLogDetailView`, `codePipelineListView`, and
`codePipelineDetailView` each have their own struct type (holding an
`app *App` back-reference for reaching shared shell state) with methods
hung off *that* type, in their own file. `reapplyTheme` (`theme.go`)
already reaches into these via e.g. `a.queuesV.table`, not flat fields —
proof the pattern works end to end, including through the one place
(live theme switching) that has to touch nearly everything.

## Problem

The overlays that *don't* follow the established pattern are exactly what
makes `app.go` and the `App` struct feel "packed": every one of their
fields, and (for confirm/move-picker/send-message specifically) their
entire show/close logic, sits in `app.go` mixed in with core shell wiring,
instead of living next to the 15 features that already got their own
file. It's not that Go code is inherently dense — it's that this codebase
already has a working convention that roughly a third of its overlay
state doesn't follow yet.

## Solution

Apply the existing per-feature-struct pattern to the overlays that don't
have it yet. Full inventory, split by how far each already is from the
target shape:

| Group | Fields on `App` today | Has its own file already? | Target |
|---|---|---|---|
| Confirm dialog | `confirmFlex`, `confirmText`, `confirmList`, `confirmVisible` (4) | No — lives in `app.go` | **This CR** |
| Move picker | `movePickerFlex`, `movePickerList`, `movePickerSearch`, `movePickerQueues`, `movePickerPreferred`, `movePickerOnSelect`, `movePickerOnClose`, `movePickerVisible` (8) | No — lives in `app.go` | **This CR** |
| Send message | `sendMessageFlex`, `sendMessageArea`, `sendMessageList`, `sendMessageOnClose`, `sendMessageVisible` (5) | No — lives in `app.go` | **This CR** |
| Connection manager + editor | `connManager*` (4), `connEditor*` (5) | Yes — `connections.go` | Backlog |
| Message filter | `messageFilterForm`, `messageFilterVisible` (2) | Yes — `messagefilter.go` | Backlog |
| Time range modal | `timeRange*` (8) | Yes — `timerangemodal.go` | Backlog |
| Datadog settings editor | `datadogEditorForm`, `datadogEditorVisible` (2) | Yes — `datadogsettings.go` | Backlog |
| Theme picker | `themePickerFlex`, `themePickerList`, `themePickerVisible` (3) | Logic in `settings.go`, construction in `app.go` | Backlog |
| AWS profiles picker | `awsProfiles*` (8) | Yes — `awsprofiles.go` | Backlog |

That's the strategy: **this CR is the first, smallest slice** — the three
overlays that don't even have a file yet, so extracting them gives the
clearest before/after and the least risk of colliding with anything
mid-refactor. The six "Backlog" rows are real, already-scoped follow-up
work — each is its own future CR once this one's pattern has proven out
live, not a promise made and forgotten. They're listed here so the next
CR doesn't have to re-derive this inventory.

### Target shape (per overlay)

Following `queuesView`'s existing shape exactly:

```go
type confirmDialog struct {
	app  *App
	flex *tview.Flex
	text *tview.TextView
	list *tview.List
}

func newConfirmDialog(a *App) *confirmDialog { ... }        // construction, was inline in New()
func (c *confirmDialog) show(question string, onConfirm func()) { ... }  // was a.showConfirm
func (c *confirmDialog) close() { ... }                       // was a.closeConfirm
```

`App` then holds one field: `confirm *confirmDialog` (visibility becomes
implicit — no separate `confirmVisible` bool needed once nothing outside
the struct needs to ask; check current external callers before dropping
it, see plan.md). External call sites (`a.showConfirm(...)`) become
`a.confirm.show(...)` — a mechanical rename, not a behavior change.

### Cross-cutting: `reapplyTheme`

`theme.go`'s `reapplyTheme(a *App, p config.Palette)` is the one place
that already reaches into every existing struct-based view (e.g.
`a.queuesV.table.SetBackgroundColor(...)`), so extending it for the three
newly-extracted overlays (currently `a.confirmFlex`, `a.movePickerList`,
`a.sendMessageArea`, etc.) is exactly the same kind of edit already made
15 times over — updated to `a.confirm.flex`, `a.movePicker.list`,
`a.sendMessage.area`, etc. This must be updated in the same change or
live theme switching breaks for these three overlays.

## Scope

### In scope

- New files `confirm.go`, `movepicker.go`, `sendmessage.go`, each with a
  dedicated struct (`confirmDialog`, `movePicker`, `sendMessageOverlay`)
  replacing the corresponding flat `App` fields and `(a *App)` methods
  currently in `app.go`.
- `app.go`: remove the extracted fields/methods; construction in `New()`
  calls the new files' constructors instead of inlining widget setup.
- `theme.go`: `reapplyTheme`'s sections for these three overlays updated
  to the new field paths.
- `messages_test.go`: the 6 references to the moved fields
  (`a.confirmVisible`, `a.confirmText`, `a.movePickerVisible`) updated to
  the new paths. No behavior change expected — these are structural
  access-path updates only.
- Public entry points (`a.showConfirm(...)`, `a.showMovePicker(...)`,
  `a.showSendMessage(...)`, and their `close*`/helper counterparts) keep
  their existing signatures and call sites everywhere else in the
  codebase (messages.go, message_detail.go, queues.go, etc.) — this is an
  internal-structure change, not an API change, so nothing outside
  `app.go`/`theme.go`/`messages_test.go` should need to change at all.

### Out of scope (this CR)

- The six backlog groups in the table above — each becomes its own future
  CR, in the order listed (smallest/cleanest first).
- Removing the now-possibly-redundant `*Visible` bools if nothing external
  reads them anymore — plan.md decides this per-overlay based on what's
  actually still reading them.
- Any behavior change to any overlay. This is a pure structural refactor;
  if it changes what the user sees or how anything responds, that's a bug
  in the refactor, not an intended side effect.
- Splitting `App`'s remaining ~85 fields (core shell state, the 15
  already-extracted views, and the dependency-injection func fields used
  by several views) — those either already follow the pattern or
  legitimately belong on the shared shell.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `app.go` is meaningfully shorter (the ~260 lines of confirm/move-picker/
   send-message logic plus their `App` struct fields move out).
3. No behavior change: confirm dialogs, the move picker, and send-message
   still work identically — verified live (`verify-live` skill), since
   these are exercised by delete/move/send flows across queue and message
   views.
4. `reapplyTheme` still correctly re-colors all three overlays after a
   live theme switch — verified live, since this is exactly the kind of
   thing unit tests won't catch (per `tui/CLAUDE.md`'s existing tview
   gotchas).
