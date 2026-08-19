# Plan: list navigation wrap-around toggle

## Approach

Add one small shared type, `ui.TableWrap`, that owns the wrap-enabled
bool and the edge-wrapping logic for a single `tview.Table`. Each of the
9 list views holds one `ui.TableWrap` value and wires it into its
existing `table.SetInputCapture` closure via a short guard clause
inserted at the top, before that view's existing switch statement.

### `ui.TableWrap` (new — `tui/internal/ui/tablewrap.go`)

```go
type TableWrap struct {
    enabled bool
}

func (w *TableWrap) Enabled() bool { return w.enabled }
func (w *TableWrap) Toggle()       { w.enabled = !w.enabled }

// HandleNav intercepts an up/down navigation key (KeyUp/KeyDown, or the
// 'j'/'k' vim aliases every list view already supports) for table.
// headerRows is the number of non-selectable fixed header rows at the
// top (today always 1 — row 0 — across every list view). It returns the
// event to forward to the table's own InputHandler: nil if it fully
// handled the navigation itself (because it wrapped), otherwise a
// normalized KeyUp/KeyDown event so 'j'/'k' keep working as arrow-key
// aliases exactly as they do today. Callers should only invoke this for
// events that are already known to be nav keys (see integration pattern
// below); it does not itself filter out non-nav events.
func (w *TableWrap) HandleNav(table *tview.Table, headerRows int, event *tcell.EventKey) *tcell.EventKey
```

Implementation: determine `down`/`up` from the event, build the
normalized `KeyDown`/`KeyUp` event tview's default handler expects. If
wrap is disabled, or the table has no selectable data rows
(`table.GetRowCount()-1 < headerRows`), return the normalized event
unchanged (today's clamped behavior). Otherwise read
`row, col := table.GetSelection()`; if moving down from the last row,
`table.Select(headerRows, col)` and return `nil`; if moving up from the
first data row, `table.Select(table.GetRowCount()-1, col)` and return
`nil`. Any other position: return the normalized event so the table's
own handler moves the selection by one row as usual.

This is the only new production code — no changes to `tview`/`tcell`
usage patterns elsewhere, no new dependency.

### Per-view integration pattern

Every view's `table.SetInputCapture` closure gets two additions, inserted
before its existing switch (which may be the boolean-expression style
`switch { case event.Rune() == 'x': }` used in `messages.go`, or the
rune-tag style `switch event.Rune() { case 'x': }` used in the other 8
files — the guard works identically either way since it's a plain `if`,
not part of either switch):

```go
if event.Rune() == 'j' || event.Rune() == 'k' ||
    event.Key() == tcell.KeyDown || event.Key() == tcell.KeyUp {
    return <view>.wrapNav.HandleNav(<view>.table, 1, event)
}
if event.Rune() == 'W' {
    <view>.wrapNav.Toggle()
    return nil
}
```

The existing `case 'j': return KeyDown` / `case 'k': return KeyUp` cases
already inside each view's switch become dead (the guard above returns
first) and are deleted as part of the same edit.

Each view struct gains one field: `wrapNav ui.TableWrap` (zero value is
"disabled", matching the spec's default-off requirement — no
constructor wiring needed).

Each view's `Shortcuts()` gains one entry reflecting live state, e.g.:

```go
wrap := "off"
if qv.wrapNav.Enabled() {
    wrap = "on"
}
...
{Key: "W", Description: "wrap: " + wrap},
```

### Header-row count

All 9 views currently call `table.SetFixed(1, 0)` and mark row 0's cells
`SetSelectable(false)` in their `setHeader`/equivalent — a single header
row. `headerRows` is passed as a literal `1` at each call site rather
than hardcoded inside `TableWrap`, so a future view with a different
header shape isn't blocked by the helper. This will be double-checked
file-by-file during implementation (tasks.md breaks out one task per
view specifically so each file's actual structure is confirmed, not
assumed from the pattern seen in `messages.go`/`queues.go`).

## Files touched

- `tui/internal/ui/tablewrap.go` (new) + `tablewrap_test.go` (new) — the
  shared helper and its unit tests (wrap on/off at both edges, single
  data row, empty table, toggle).
- `tui/internal/view/queues.go`
- `tui/internal/view/messages.go`
- `tui/internal/view/logs.go`
- `tui/internal/view/logsearch.go`
- `tui/internal/view/ssmparams.go`
- `tui/internal/view/secrets.go`
- `tui/internal/view/codepipelinelist.go`
- `tui/internal/view/codepipelinedetail.go`
- `tui/internal/view/datadoglogs.go`

Each of the 9 view files: add the `wrapNav` field, the guard clause, the
`W` case, the `Shortcuts()` entry, remove the now-dead `j`/`k` cases from
the existing switch. `datadoglogs.go` additionally has a service/env
filter `DropDown` — that widget is untouched (spec explicitly scopes
wrap to the table-based views' own list, not dropdowns).

- `spec/` — no area doc exists yet for "list navigation" as a
  cross-cutting concern; merge-back will fold a short note into each
  touched view's existing spec area (e.g. `spec/08-message-browser-and-
  detail`, `spec/07-activemq-queue-list`, `spec/17-aws-cloudwatch-logs`,
  etc.) rather than creating a new `spec/` folder, since this is a small
  behavioral addition to existing views, not a new capability area.

## Key decisions & trade-offs

- **Shared helper vs. per-view duplication**: a shared `ui.TableWrap`
  keeps the edge-case logic (empty table, single row, header offset) in
  one tested place instead of copy-pasted 9 times with 9 chances to get
  the boundary condition wrong. Trade-off: every view takes a small
  dependency on a new `internal/ui` type, but that's consistent with the
  existing `ui.StyleList`/`ui.Shortcuttable` pattern other cross-cutting
  UI concerns already use in this package.
- **`if` guard before the switch, not a new switch case**: since 8 of 9
  files use a rune-tag switch (`switch event.Rune() { ... }`), a case
  inside that switch can't also match `event.Key() == tcell.KeyUp` (a
  different comparison target). A plain `if` above the switch handles
  both key forms uniformly without restructuring each file's existing
  switch style.
- **Field, not constructor param**: `wrapNav ui.TableWrap` as a
  zero-value struct field (not passed into `NewXView(...)`) keeps every
  constructor signature unchanged — this is a pure behavior addition,
  not a new dependency the caller needs to supply.
- **No cross-view state**: each view's `TableWrap` is independent by
  construction (it's a field on that view's struct) — matches the
  spec's "toggling wrap on the message browser doesn't affect the
  queues list" requirement with no extra code.

## Testing

- Unit tests for `ui.TableWrap` covering: wrap disabled (unchanged
  clamped behavior), wrap enabled at bottom edge, wrap enabled at top
  edge, wrap enabled mid-list (normal single-step move), empty table
  (header row only), single-data-row table (up and down both stay put,
  no crash from selecting the same row as "wrap").
- No new unit tests needed per-view beyond what already exists — the
  per-view change is thin wiring around the shared, tested helper.
  `Shortcuts()` already isn't unit-tested elsewhere in this codebase
  (spot-checked: no existing `_test.go` asserts on `Shortcuts()`
  contents), so no new convention introduced there.
- Manual verification: per `tui/CLAUDE.md`, changes touching queue/
  message behavior need `verify-live`. This CR touches navigation across
  all list views including the message browser, so tasks.md will include
  a `verify-live` pass (toggle `W`, confirm wrap at both edges, confirm
  default-off, confirm independence between two different views) as
  well as manual checks for the AWS/Datadog-backed views that
  `verify-live` doesn't cover (those are checked by hand against
  whatever the developer has configured, same as other CRs touching
  those views).
