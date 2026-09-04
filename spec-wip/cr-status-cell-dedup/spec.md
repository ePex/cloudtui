# CR: dedup showError/showStatus/table-clear boilerplate

Date: 2026-09-04

## Purpose

`ssmparams.go`, `secrets.go`, `logs.go`, `codepipelinelist.go`, and
`codepipelinedetail.go` each hand-roll the same two small shapes,
flagged in the 2026-09-04 architectural review (`BACKLOG.md`) as a
smaller-scale duplication of the same kind the load/reauth helper
(`runAWSLoad`, shipped earlier) already fixed:

1. **The table-body clear-loop**, `for table.GetRowCount() > 1 {
   table.RemoveRow(table.GetRowCount() - 1) }`, repeated 3-4 times per
   file — inside `repaint()`, `showError()`, `showStatus()`, and (for
   `codepipelinedetail.go` specifically) `Open()` and `Render()` too.
2. **The single-row error/status cell**, `showError`/`showStatus`:
   null out the view's own cached data, clear the table body, write
   one `tview.TableCell` (red "Error: …" or accent-colored status
   text) into row 1, set the table's title — identical shape in all 5
   views, varying only in: which column the cell goes in (1 for
   SSM/Secrets/CloudWatch Logs, which have a leftmost favorite-star
   column; 0 for the two CodePipeline views, which don't), the title
   text (static per-view text for 4 of them, a per-pipeline dynamic
   title for `CodePipelineDetailView`), and which struct field(s) get
   nulled (`all`+`filtered` for 4 of them, just `stages` for
   `CodePipelineDetailView`).

## Scope

- A new shared file `internal/view/statuscell.go` with two helpers:
  - `clearTableBody(table *tview.Table)` — the 2-line clear-loop,
    used everywhere it currently appears across the 5 files (not just
    inside `showError`/`showStatus` — also `repaint()`, `Open()`, and
    `Render()`, which have the identical loop today).
  - `showStatusCell(table *tview.Table, col int, text string, color
    tcell.Color, title string, clearState func())` — the full
    `showError`/`showStatus` shape, built on `clearTableBody`;
    `clearState` is the one piece that's genuinely per-view (different
    field names/types), passed in as a closure.
- Each of the 5 views' `showError`/`showStatus` rewritten to call
  `showStatusCell`, collapsing from ~17 lines each to ~3.
- Each of the 5 views' other clear-loop call sites (`repaint()`
  everywhere; `Open()`/`Render()` in `codepipelinedetail.go`) rewritten
  to call `clearTableBody(table)` instead of the inline loop.

## Out of scope

- No behavior change — pure refactor, same rendering, same column
  indices, same titles, same nulled fields per view.
- `queues.go` isn't included — it wasn't named in the original review
  finding (which listed only the 5 AWS views), and checking it
  directly shows its `showError`/`showStatus` differs enough that
  forcing it into the same `showStatusCell` shape wouldn't be a clean
  fit: no `SetTitle` call at all (title is left alone on error/status,
  unlike the 5 AWS views), no cached-state nulling, and
  `SetExpansion(5)` instead of `3`. Bending `showStatusCell` to cover
  this too (optional title, optional expansion) would add parameters
  for one caller's variance — left as a possible future follow-up if
  `queues.go` is ever revisited on its own, not bundled here.
- No change to `repaint()`'s own row-population logic, favorite-star
  handling, or title-formatting logic beyond the clear-loop line
  itself.

## Data & config

No new packages. New file `tui/internal/view/statuscell.go` +
`statuscell_test.go`. Touches
`tui/internal/view/{ssmparams,secrets,logs,codepipelinelist,
codepipelinedetail}.go`; existing `_test.go` files for those 5 views
expected to need no changes (same acceptance bar as the load/reauth
dedup CR — behavior-preserving refactor, existing assertions should
keep passing unmodified).
