# Plan

## Files touched

- `tui/internal/app/app.go` — `onGlobalKey`'s exemption loop.
- `tui/internal/app/app_test.go` — correct the two existing Datadog
  dropdown regression tests to actually open the popup; no new test
  file needed (the existing tests already target exactly this
  scenario, they just don't trigger the bug).

No new dependencies, no `queue.Backend`/broker interaction — this is a
pure `tview` focus-handling fix, so `verify-live` doesn't apply; the
manual check is a quick tmux drive, not a broker-backed one.

## The fix itself

```go
// Before:
focus := a.tv.GetFocus()
for _, in := range a.focusExemptInputs {
    if focus == in {
        return event
    }
}

// After:
for _, in := range a.focusExemptInputs {
    if in.HasFocus() {
        return event
    }
}
```

One line changed, one line removed (`focus := a.tv.GetFocus()`, now
unused in this function — confirmed no other reference to `focus` in
`onGlobalKey`).

### Key decisions

- **`HasFocus()` over introducing a new interface (e.g. cloudtui-go's
  `DropdownAware`, discovered via the sibling-rebuild comparison this
  session).** Both fix the same bug. `HasFocus()` is strictly simpler
  here: `tview.Primitive` (the type `focusExemptInputs` already holds)
  already requires it, so this is a same-signature swap with no new
  type, no view-side wiring, and no risk of a *future* view forgetting
  to implement the new interface the way `datadogEditorVisible` was
  once forgotten (spec/18's own documented incident). A dedicated
  interface would be worth it if a future case needed the active view
  to report something `HasFocus()` genuinely can't express — not true
  here.
- **No change to `focusExemptInputs`'s contents or `FilterInputs()`
  signatures on any view.** The bug is entirely in how the collected
  primitives are *checked*, not in what's collected.

## Testing

- `TestOnGlobalKeyPassesThroughWhenDatadogServiceFilterFocused`/
  `...EnvFilterFocused`: after `a.tv.SetFocus(dd)`, additionally drive
  `dd.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)`
  (`setFocus` a closure wrapping `a.tv.SetFocus`, matching
  `tview.Application.SetFocus`'s signature — confirmed via a scratch
  test during the spec stage that this genuinely reproduces the bug: an
  identity check against the outer `*DropDown` starts failing at exactly
  this point, while `dd.HasFocus()` stays correct) *before* calling
  `a.onGlobalKey(event)`. Run with the fix reverted first to confirm
  each test actually fails without it (not just passes trivially),
  matching this session's established practice for regression tests
  tied to a live-reproduced bug.
- No new test file — both existing tests already target exactly the
  right scenario (Service/Env filter focused, letter key passed
  through); they just weren't exercising the open-popup state that
  actually breaks.
- Manual verification (build + tmux, no broker needed): open Datadog
  Logs, open the Service dropdown, type a hotkey letter (`q`), confirm
  it doesn't quit and instead behaves as dropdown input (prefix-jump or
  is otherwise consumed by the list); repeat for Env and for at least
  one other hotkey letter (`h` or `s`) to rule out a q-specific fluke.

## Trade-offs / risks accepted

None — this is a minimal, well-understood fix to a mechanism
(`focusExemptInputs`) with a single non-trivial implementer
(`*tview.DropDown`) among its current users, confirmed correct by
direct experimentation before deciding on the approach.
