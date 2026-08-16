# Plan — Bugfix 57: hide Broker Name when Backend is proxy

## Approach

Wire the existing Backend dropdown's `SetSelectedFunc` (currently `nil`) to
a new `rebuildConnEditorTail(backend string)` that rebuilds every form item
after Backend — Broker Name (jolokia only), URL, Username, Password Source,
and Password/Password Secret — reusing the `RemoveFormItem`/`Add*` trick FE
56 introduced for the Password Source swap. Unlike that swap, Broker Name
sits in the *middle* of the item list, so a single-item replace isn't
enough: everything after it needs to shift. Rebuilding the whole tail is
simpler and more robust than manually computing shifted indices.

Because items after Backend can now appear at different positions
depending on whether Broker Name is present, `showConnEditor` and
`saveConnEditor` switch from `GetFormItem(<fixed index>)` to
`GetFormItemByLabel(<label>)` for everything from Broker Name onward. Name
(0) and Backend (1) stay fixed-index — they're never removed or reordered.

### Preserving typed values across a rebuild

Toggling Backend mid-edit already didn't reset any other field before this
change (its callback was `nil`). `rebuildConnEditorTail` must keep that:
before removing anything, it reads the current text/selection out of every
field it's about to remove (via `GetFormItemByLabel`, `nil`-safe), then
passes those values back in as the new fields' initial `value` argument
instead of leaving them empty. This includes Broker Name itself — hiding it
for proxy doesn't discard what was typed; toggling back to jolokia restores
it.

**Update, found during live verification:** the capture-before-remove trick
above doesn't work for Broker Name specifically, because the value has
nowhere to live *while* it's hidden — the field it would be read from
doesn't exist during that gap. A jolokia → proxy → jolokia round trip came
back with Broker Name silently reset to `""`. Fixed with a new `App` field,
`connEditorBrokerName string`, that shadows the value across that gap:
`rebuildConnEditorTail` updates it whenever the field currently exists and
otherwise leaves it alone (so it still holds the last real value when the
field reappears). `showConnEditor` sets it explicitly from
`conn.Queue.BrokerName` *after* the Backend dropdown's `SetCurrentOption`
call, not before — that call always fires `rebuildConnEditorTail`
synchronously (even when the index isn't actually changing), and that
function's own capture step would otherwise immediately overwrite whatever
`showConnEditor` had just set with the *previous* (stale/empty) field's
text.

### A latent bug this touches

`setConnEditorPasswordField` currently does `RemoveFormItem(6)` — a
hardcoded index that assumed Broker Name (and therefore item 6 being the
last item) is always present. Once Broker Name can be absent, the last
item is index 5, not 6, and `RemoveFormItem(6)` would panic (slice
out-of-range) the moment a user picks "AWS Secret" while Backend is set to
proxy. Fixing this — `RemoveFormItem(f.GetFormItemCount() - 1)` instead of
a literal `6` — is in scope here since it's a direct consequence of making
Broker Name conditional, not a separate bug.

### Why not resize the overlay

Item count varies (7 for jolokia, 6 for proxy), but the overlay's height
(`centered(a.connEditorForm, 64, 20)`) stays a fixed literal sized for the
larger (jolokia) case. Proxy just gets one extra blank row at the bottom
instead of the current single spare row — not clipped, not worth the
complexity of resizing the overlay live when Backend changes.

## Files touched

### `tui/internal/app/app.go`

- The existing `if dd, ok := a.connEditorForm.GetFormItem(1).(*tview.DropDown); ok { styleDropDown(dd, cfg.Colors) }`
  block (Backend dropdown) gains a `dd.SetSelectedFunc(...)` call, wired
  after the full initial chain is built — same reasoning as the Password
  Source dropdown: wiring it inside `AddDropDown(...)` itself would fire
  during construction, before the rest of the chain exists.
  ```go
  dd.SetSelectedFunc(func(_ string, idx int) {
      backends := []string{"jolokia", "proxy"}
      a.rebuildConnEditorTail(backends[idx])
  })
  ```
- No change to the static initial chain itself (`AddInputField`/
  `AddDropDown` calls) — it already builds the full jolokia layout
  (Backend defaults to option 0 = jolokia), which is the correct starting
  state.
- New `App` field `connEditorBrokerName string` — see "Preserving typed
  values across a rebuild" above.
- No height/comment change (see "Why not resize the overlay").

### `tui/internal/app/connections.go`

- New `rebuildConnEditorTail(backend string)`: captures current Broker
  Name/URL/Username/Password-Source-selection/Password-or-Secret-text via
  `GetFormItemByLabel` (each `nil`-safe — absent fields just contribute
  `""`/option 0), removes every item from index 2 onward
  (`for f.GetFormItemCount() > 2 { f.RemoveFormItem(2) }`), then re-adds:
  Broker Name (only if `backend != "proxy"`, prefilled from the captured
  value), URL, Username, Password Source (prefilled to the captured
  selection, `nil` callback — wiring happens after, same reasoning as
  above), and finally Password *or* Password Secret (AWS) matching the
  captured selection, prefilled from the captured text. Re-fetches the
  freshly-created Password Source dropdown by label to `styleDropDown` it
  (a new `tview.DropDown` instance needs this every time — see the
  dropdown-styling gotcha from FE 22) and wire its `SetSelectedFunc` to
  `setConnEditorPasswordField`, exactly as the static constructor did in
  `app.go` before this change.
- `setConnEditorPasswordField`: `RemoveFormItem(6)` → `RemoveFormItem(f.GetFormItemCount() - 1)`
  (see "A latent bug this touches").
- `showConnEditor`: `GetFormItem(2..6)` calls become `GetFormItemByLabel(...)`
  for Broker Name (only when `conn.Backend != "proxy"`), URL, Username,
  Password Source, and Password/Password Secret. `SetCurrentOption` on the
  Backend dropdown (item 1, still fixed-index) now fires
  `rebuildConnEditorTail`, which must run *before* the label-based prefill
  calls below it (already true — it's synchronous, same call stack).
- `saveConnEditor`: same `GetFormItem(2..6)` → `GetFormItemByLabel(...)`
  swap; Broker Name read is `nil`-safe (empty string when the field isn't
  present, i.e. backend is proxy) instead of relying on the
  `backend == "proxy"` branch to discard it.

## Testing

- No new automated test file — this is UI-structure logic
  (`tview.Form` item wiring) with no non-trivial branching outside of
  what's already exercised by existing connection-editor behavior; per
  `tui/CLAUDE.md`, "if something is genuinely untestable, say so
  explicitly... and verify manually instead." The one piece of real logic
  (`setConnEditorPasswordField`'s index fix) is covered by manual
  verification of the exact scenario that used to panic: proxy backend +
  switching Password Source to "AWS Secret".
- Manual (`verify-live` skill, per `tui/CLAUDE.md` — connection-editor
  behavior): open editor on a jolokia connection (Broker Name shown,
  prefilled); toggle to proxy (Broker Name disappears, URL/Username/
  Password Source/Password untouched); toggle back to jolokia (Broker Name
  reappears with its original value restored, not blank); with Backend set
  to proxy, switch Password Source to "AWS Secret" (must not panic — this
  is the latent-bug scenario); save a proxy connection and confirm no
  Broker Name ends up in `config.yaml`.
