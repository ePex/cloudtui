# Bugfix: global hotkeys hijack keystrokes typed into an open embedded dropdown

Date: 2026-08-26

## Problem

In the Datadog Logs view, opening either filter dropdown (`serviceFilterDD`/
`envFilterDD`) and typing a letter that happens to be a global hotkey does
**not** type into the dropdown's own prefix-jump search — it fires the
global hotkey instead. Confirmed live: opening the Service dropdown and
pressing `q` quits the entire app while the dropdown is open.

This is a different instance of the "global-hotkey leak" class of bug
already documented in spec/18 ("any new full-page overlay/editor must be
added to the app's overlay-visibility exemption set... this bit the
Datadog settings editor") — but that fix targets full-page overlays
tracked by `overlayVisible`. This bug is in the *other*, older exemption
mechanism: `focusExemptInputs`, the per-view list of input widgets whose
focus should suppress global hotkeys (`internal/app/app.go`'s
`onGlobalKey`).

## Root cause

`onGlobalKey` exempts global hotkeys only when the currently focused
primitive is *identical* (`==`) to one of the widgets registered in
`a.focusExemptInputs`:

```go
focus := a.tv.GetFocus()
for _, in := range a.focusExemptInputs {
    if focus == in {
        return event
    }
}
```

`DatadogLogsView.FilterInputs()` registers the two `*tview.DropDown`
values themselves. This works while a dropdown's popup is **closed** —
`tview.Application.GetFocus()` then genuinely returns the `*DropDown`
pointer. But `tview.DropDown.Focus(delegate)` is implemented as:

```go
func (d *DropDown) Focus(delegate func(p Primitive)) {
    if d.open {
        delegate(d.list)   // an unexported, internal *tview.List
    } else {
        d.Box.Focus(delegate)
    }
}
```

Opening the popup calls `setFocus(d.list)` internally (`openList`,
`dropdown.go`), which sets `Application.focus` to that private `*List`,
not the `DropDown` itself. From that point on, `a.tv.GetFocus() == in`
is comparing the internal list against the outer dropdown pointer —
never equal — so the exemption silently stops matching for every
keystroke while the popup stays open.

Traced and reproduced directly (see below) rather than only inferred:
`d.HasFocus()`, unlike the identity check, **does** correctly delegate:

```go
func (d *DropDown) HasFocus() bool {
    if d.open {
        return d.list.HasFocus()
    }
    return d.Box.HasFocus()
}
```

A throwaway test confirmed the exact divergence: after opening the
dropdown's popup, `a.tv.GetFocus() == dd` becomes `false`, while
`dd.HasFocus()` correctly stays `true`.

## Why the existing regression test didn't catch this

`TestOnGlobalKeyPassesThroughWhenDatadogServiceFilterFocused` (and its
`envFilterDD` counterpart) call `a.tv.SetFocus(a.datadogLogsV.FilterInputs()[1])`
directly — this sets focus on the **closed** dropdown, which is exactly
the one case where the identity check still works. Neither test ever
opens the popup, so neither exercises the actual bug.

## Fix

Replace the identity check with a `HasFocus()` check:

```go
for _, in := range a.focusExemptInputs {
    if in.HasFocus() {
        return event
    }
}
```

`tview.Primitive` already requires `HasFocus() bool`, and every existing
`focusExemptInputs` entry — plain `*tview.InputField`s from every other
view's `FilterInputs()`, plus `DatadogLogsView`'s two `*tview.DropDown`s —
implements it correctly for this purpose: `InputField.HasFocus()` is a
plain identity-equivalent check (no change in behavior for the
non-dropdown cases), while `DropDown.HasFocus()` is the one that
actually delegates through to the internal list when open, closing the
gap. `a.tv.GetFocus()` becomes unused in this function once the loop no
longer needs it.

## Scope

- In scope: `onGlobalKey`'s `focusExemptInputs` check
  (`internal/app/app.go`); the two existing Datadog dropdown regression
  tests, corrected to actually open the popup before asserting (so they
  exercise the real bug instead of only the already-working closed-state
  case).
- Out of scope: the separate `overlayVisible`/full-page-overlay
  exemption mechanism (already correct, not implicated here); any other
  `tview.DropDown` usage in the app — `StyleDropDown`'s other callers
  (theme picker, connection editor, AWS profile picker, Settings) are
  all inside modal overlays already covered by `anyOverlayVisible()`,
  not by `focusExemptInputs`, so they were never exposed to this
  specific bug. `DatadogLogsView` is the only view whose `FilterInputs()`
  returns a `*tview.DropDown` today.

## Manual verification

Unit-testable in full: the corrected regression tests open the dropdown
popup (driving its `InputHandler` the way a real Enter/click would, or
calling the same internal open path a test can reach) before asserting
`onGlobalKey` passes the event through. Given this is a keyboard-focus
interaction bug, also verify manually (build + tmux): open Datadog
Logs' Service filter, open the dropdown, type a global-hotkey letter
(e.g. `q`, `h`, `s`), confirm it searches/jumps within the dropdown
rather than quitting/navigating away; repeat for the Env filter.
