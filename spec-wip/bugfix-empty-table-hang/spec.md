# Bugfix: arrow keys in an emptied table hang the TUI

Date: 2026-10-10

## Problem

Pressing ↓ (or ↑, `j`/`k`, PgDn) in a table that has no selectable
rows freezes the TUI: the screen stops updating, every
key is ignored, and the process spins at 100 % CPU until it is killed.

Found during the fe-progress-modal regression run, and reproduced on
`main` (`2b1b0cf`):

- **Message list:** open a queue with messages and move the cursor
  down a few rows, go back, open an empty queue, press ↓.
- **Queue list:** move the cursor down a few rows, filter (`/`) to a
  text that matches no queue, press ↓.

A goroutine dump shows the UI goroutine looping inside tview's own
`Table.InputHandler` (`tableDefaultContent.GetCell`).

Root cause (read in `tview` v0.42.0's `table.go`): a table keeps its
selected row when its content is cleared and refilled. Navigation
searches forward/backward for a selectable cell and stops when it
reaches a "final" cell. For ↓ and ↑, that final cell is the **previous
selection**. When the table now has only its header row (nothing
selectable) and the previous selection lies **beyond** the new
content, the search wraps around inside the table forever without
ever reaching it. A table with at least one selectable cell, or with
its selection inside the content, always terminates.

## Where it happens

Every `tview.Table` in the TUI that can be emptied (or shortened)
while its selection stays put:

- views: queues, messages, SSM parameters, Secrets Manager, CloudWatch
  log groups and log search, Datadog logs, CodePipeline list and
  detail;
- `dialog/awsprofiles.go` (AWS profile picker);
- the Home screen table (`ui/views/home.go`) is fixed content, so it's
  in scope only if the shared fix naturally covers it.

Triggers: a reload that returns fewer (or no) rows, a filter or quick
search that matches nothing, switching queues/log groups/pipelines to
one with fewer entries.

## Expected behavior

- Navigation keys in a table with no selectable rows do nothing; the
  app stays responsive.
- When a table's content shrinks below its selection, the selection
  moves to a row that exists (the last remaining data row, or the
  first data row once rows appear again), so navigation always
  starts from a real row.
- Nothing else about navigation changes (wrap-around, header rows,
  marks, the detail views opened with Enter).

## Fix direction (detailed in plan.md)

Keep every table's selection inside its content, in one shared place
rather than in each view, so a table added later can't reintroduce
the hang. No fork or patch of `tview`.

## Out of scope

- Reporting/fixing the bug upstream in `tview` (worth an issue, but
  not part of this change).
- The web console (no `tview`).
- Any other change to table navigation or rendering.
