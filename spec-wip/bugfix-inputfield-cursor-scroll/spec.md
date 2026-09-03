# Bugfix: text-field cursor not visible for long pre-filled values

Date: 2026-09-03

## Problem

Reported in the connection editor: when editing an existing connection
whose Name, URL, or Secret Name is longer than the field's visible
width, the field shows only the *start* of the value and the cursor
isn't visible anywhere — the user can't tell the field is even
focused, let alone where the cursor is.

Root cause (confirmed by reading `tview` v0.42.0's source, vendored at
`github.com/rivo/tview@v0.42.0`): `InputField.SetText()` calls
`TextArea.Replace()`, which recomputes the cursor's true end-of-text
position but calls `t.findCursor(false, row)` — the `clamp=false`
variant skips the branch that scrolls `columnOffset` to keep the
cursor inside the visible window. The field's `Draw()` only shows the
cursor when `column - columnOffset < width`; since `columnOffset`
never moved, that check fails and the cursor is hidden, while the
visible slice still shows the start of the text.

This is specific to **programmatic `SetText()` calls that pre-populate
a field with an already-long value** — normal typing/arrow-key/
backspace handling all call `findCursor(true, row)` (via
`moveCursor`), which scrolls correctly. So this never shows up while
typing a value long enough to overflow, only when an existing long
value is loaded into a field that's about to be edited or is tabbed
into.

## Where it happens

Confirmed in the connection editor (`tui/internal/dialog/connections.go`)
on any pre-filled field wider than its visible width: Name, URL,
Secret Name, AWS Profile, Username, Password.

The same `SetText(existingValue)` pattern — restoring a previously-set
value into a field — also exists in:

- `tui/internal/dialog/datadogsettings.go` (Site, Access Token)
- `tui/internal/dialog/messagefilter.go` (JMS type, dates, max count)
- `tui/internal/dialog/timerangemodal.go`
- `tui/internal/view/logsearch.go` (restored search pattern)
- `tui/internal/view/datadoglogs.go` (restored query)
- `tui/internal/view/messages.go` (restored quick search)
- `tui/internal/view/logs.go`, `queues.go`, `ssmparams.go`,
  `secrets.go` (restored filter text)
- `tui/internal/view/codepipelinelist.go`

Any of these hits the same bug once the restored value is longer than
the field's visible width — matching the user's "maybe others as
well".

## Fix

Work around the upstream bug without patching/vendoring `tview`:
after `SetText()`, synthetically feed a `tcell.KeyEnd` event through
the field's own `InputHandler()`. Confirmed by reading the source that
`InputField`'s key switch falls through to `TextArea.InputHandler()`
for `KeyEnd`, which calls `moveCursor()`, which always ends with
`t.findCursor(true, row)` — the same clamping call a real keypress
would trigger, with no dependency on the field currently having focus
(`Box.WrapInputHandler` has no focus guard).

**Correction, found via live testing after task 1 first shipped**:
the above is necessary but not sufficient. `moveCursor()` is a
complete no-op while `TextArea.lastWidth` is still `0` — which is the
case until the field has been drawn for real at least once. The very
common case of editing *any* connection for the first time in a
session hits exactly this: the synthetic keypress fired immediately
after `SetText()` did nothing (field never drawn yet), and worse, left
the cursor "resolved" in a way that permanently blocked `tview`'s own
(very limited — it only resolves position, never scroll offset) first
draw self-heal. The fix now establishes `lastWidth` itself first, via
a throwaway off-screen `Draw()` using the field's own current rect —
`tview.Box` defaults to a non-zero rect from construction (`NewBox`:
15×10), so this works even before the field has ever been part of a
visible layout. See `tui/internal/ui/inputfield.go`'s doc comment for
the full mechanism.

A small helper, `ui.SetInputFieldText(field *tview.InputField, text
string)` in `tui/internal/ui`, wraps `SetText` + the throwaway draw +
the synthetic `KeyEnd` in one call. Every call site above that
populates a field with a previously-stored value switches from
`field.SetText(x)` to `ui.SetInputFieldText(field, x)`.

## Scope

- New helper in `tui/internal/ui` (exact filename decided in
  `plan.md`).
- Every `SetText(existingValue)` call site listed above switches to
  the new helper — same root cause, same fix, all in this one PR
  rather than fixing only the reported instance and leaving the
  already-identified rest as known bugs.
- Unit test for the helper itself (its cursor-scroll math can be
  tested against `tview.InputField`'s public `GetFieldWidth`/`GetText`
  — no live terminal needed).

## Out of scope

- Not filing/patching upstream `tview` — the synthetic-keypress
  workaround is self-contained and doesn't require forking the
  dependency.
- Not touching call sites that set a field's text to a short/fixed
  value the user just typed themselves (e.g. clearing a field, or
  values that are never going to overflow the field width) — only
  sites that restore a previously-stored, potentially-long value.
- No change to field widths — the fix is scroll behavior, not making
  fields wider.
