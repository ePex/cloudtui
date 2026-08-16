# Plan — CR 59: extract App overlay state into dedicated structs

## Approach

Move each overlay's fields, construction, and show/close/helper logic out
of `app.go` into its own file, following `queuesView`'s existing shape:
a struct holding the widgets plus an `app *App` back-reference, methods
hung off that struct instead of `*App`. `New()` calls a constructor per
overlay instead of inlining widget setup; the `centered(...)`/`AddPage(...)`
wiring that establishes z-order stays in `New()` unchanged (that ordering
is a cross-overlay concern — see the "confirm added last" comment at
`app.go:726` — not something any one overlay's file should own).

Each overlay keeps a `visible bool` field (renamed from e.g.
`confirmVisible`), because two global input-guard checks in `app.go`
(`~793` and `~839`) OR together *every* overlay's visibility flag,
including the six not touched by this CR — so the field survives, just
moves onto the new struct and gets referenced as `a.confirm.visible`
instead of `a.confirmVisible` at those two spots.

## Files touched

### `tui/internal/app/confirm.go` (new)

```go
type confirmDialog struct {
	app  *App
	flex *tview.Flex
	text *tview.TextView
	list *tview.List
}

func newConfirmDialog(a *App) *confirmDialog {
	// body = app.go:362-368's widget construction, unchanged, assigning
	// into the new struct's fields instead of a.confirm*
}

func (c *confirmDialog) show(question string, onConfirm func()) {
	// body = current showConfirm (app.go:899-925), s/a\./c\./ on the
	// moved fields, a.closeConfirm() -> c.close(), still reaches shared
	// state via c.app (c.app.rootPages, c.app.tv)
}

func (c *confirmDialog) close() {
	// body = current closeConfirm (app.go:927-932)
}
```

`visible bool` added as a field (was `confirmVisible` on `App`).

### `tui/internal/app/movepicker.go` (new)

Same shape: `movePicker` struct (`app`, `flex`, `list`, `search`,
`queues []string`, `preferred string`, `onSelect func(string)`,
`onClose func()`, `visible bool`); `newMovePicker(a *App) *movePicker`
(construction, `app.go:370-390`); `(mp *movePicker) show(sourceQueue
string, onSelect func(string), onClose func())` (was `showMovePicker`,
`app.go:1002-1068`); `(mp *movePicker) fillList(filter string)` (was
`fillPickerList`, `app.go:1073-1097`); `(mp *movePicker) close()` (was
`closeMovePicker`, `app.go:1099-1107`).

The free functions only `showMovePicker`/`fillPickerList` use —
`isSystemQueue`, `isDLQQueue`, `isIMQQueue`, `requeueQueueCandidate`,
`sortPickerQueues` (`app.go:934-1000`) — move into this file too, as
unexported package-level functions (not methods; they take no `*App` or
`*movePicker` receiver today and don't need one). `app_test.go` calls
these directly by name today and needs no changes — moving a
package-level function's file within the same package doesn't affect
callers.

### `tui/internal/app/sendmessage.go` (new)

`sendMessageOverlay` struct (`app`, `flex`, `area *tview.TextArea`, `list
*tview.List`, `onClose func()`, `visible bool`); `newSendMessageOverlay(a
*App) *sendMessageOverlay` (construction, `app.go:392-425`); `(sm
*sendMessageOverlay) show(queueName string, onClose func())` (was
`showSendMessage`, `app.go:1109-1126`); `(sm *sendMessageOverlay) doSend(queueName
string)` (was `doSend`, `app.go:1128-1150`); `(sm *sendMessageOverlay)
close()` (was `closeSendMessage`, `app.go:1152-1159`).

### `tui/internal/app/app.go`

- Remove: the 17 struct fields being extracted (4 + 8 + 5), replaced by
  three fields — `confirm *confirmDialog`, `movePicker *movePicker`,
  `sendMessage *sendMessageOverlay`.
- Remove: `showConfirm`/`closeConfirm`, `showMovePicker`/`fillPickerList`/
  `closeMovePicker`, `showSendMessage`/`doSend`/`closeSendMessage`, and
  the five now-relocated free functions — all moved, not duplicated.
- `New()`: the three inline construction blocks (`app.go:362-425`) become
  `a.confirm = newConfirmDialog(a)`, `a.movePicker = newMovePicker(a)`,
  `a.sendMessage = newSendMessageOverlay(a)`; the three local
  `...Overlay := centered(...)` lines right after keep working unchanged,
  just reading e.g. `a.confirm.flex` instead of `a.confirmFlex`. The
  final `AddPage(...)` chain (`app.go:729-741`) is untouched — same local
  var names, same order.
- Every other call site in `app.go` itself that calls `a.showConfirm(...)`,
  `a.showMovePicker(...)`, `a.showSendMessage(...)` becomes
  `a.confirm.show(...)`, `a.movePicker.show(...)`,
  `a.sendMessage.show(...)`.
- The two OR-chains (`~793`, `~839`) updated: `a.confirmVisible` →
  `a.confirm.visible`, `a.movePickerVisible` → `a.movePicker.visible`,
  `a.sendMessageVisible` → `a.sendMessage.visible`. The other flags in
  those chains (`connManagerVisible`, etc.) are untouched — out of scope.

### Other call sites (outside `app.go`)

`a.showConfirm`/`a.showMovePicker`/`a.showSendMessage` are called from
`messages.go`, `message_detail.go`, and `queues.go` (delete/move/purge/send
flows). Every one of these becomes `a.confirm.show(...)` /
`a.movePicker.show(...)` / `a.sendMessage.show(...)` — mechanical
rename, same arguments, same behavior. (Exact call sites enumerated
during implementation via `grep -rn 'a\.show\(Confirm\|MovePicker\|SendMessage\)'`
— not listing them all here since it's a pure rename with no logic
change per site.)

### `tui/internal/app/theme.go`

`reapplyTheme`'s three sections (`~242-286`: move picker, confirm,
send message) updated field-by-field: `a.movePickerFlex` →
`a.movePicker.flex`, `a.confirmText` → `a.confirm.text`,
`a.sendMessageArea` → `a.sendMessage.area`, etc. — same `nil`-guard
pattern already used (`if a.movePicker != nil { if a.movePicker.flex !=
nil { ... } }` — the outer struct itself is always non-nil after `New()`
returns, but keeping the existing inner nil-checks on the widgets is
consistent with how the rest of this function is written and costs
nothing).

### `tui/internal/app/messages_test.go`

The 6 references (`~353`, `~368`, `~384`, `~388`, `~404`, `~434`):
`a.confirmVisible` → `a.confirm.visible`, `a.confirmText.GetText(true)` →
`a.confirm.text.GetText(true)`, `a.movePickerVisible` →
`a.movePicker.visible`. No behavior change — these tests assert overlay
state after triggering delete/move flows; only the access path changes.

## Testing

No new tests — this is a pure structural move with no new branching
(`tui/CLAUDE.md`'s "genuinely untestable... say so explicitly" applies:
there's nothing new to unit-test here that `messages_test.go`'s existing
assertions don't already cover once updated). Existing test suite must
pass unchanged in behavior.

Manual (`verify-live` skill, per `tui/CLAUDE.md` — this touches
message/queue delete-move-send flows and live theme switching, both
called out in the skill's own gotchas list):

- Trigger a confirm dialog (e.g. delete a message), confirm `Esc`/`No`/
  `Yes` all still behave correctly.
- Trigger the move picker from a message, confirm search (`/`), `j`/`k`
  navigation, and selecting a target queue all still work.
- Trigger send-message, confirm `Tab` moves focus between the text area
  and Submit/Cancel, `Esc` cancels from either.
- Switch theme (Settings → Theme) while each of the three overlays is
  open in turn, confirm live recoloring still applies to all of them
  (this is exactly the kind of thing that silently breaks if a
  `reapplyTheme` field path is missed — see the tview gotchas already
  documented in the `verify-live` skill).
