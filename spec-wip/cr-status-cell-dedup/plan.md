# Plan

## Approach

### 1. New `internal/view/statuscell.go`

```go
package view

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// clearTableBody removes every row except the header (row 0) — the
// shared clear-loop behind repaint()/showError()/showStatus() (and,
// in CodePipelineDetailView, Open()/Render() too) across every
// table-backed AWS view in this package.
func clearTableBody(table *tview.Table) {
	for table.GetRowCount() > 1 {
		table.RemoveRow(table.GetRowCount() - 1)
	}
}

// showStatusCell renders a single-row, single-column status/error
// message into table — the shared shape behind SSMParamsView,
// SecretsView, LogsView, CodePipelineListView, and
// CodePipelineDetailView's showError/showStatus: null the view's own
// cached data (clearState), clear the table body, write one cell at
// (1, col) in color with text, then set the table's title.
func showStatusCell(table *tview.Table, col int, text string, color tcell.Color, title string, clearState func()) {
	clearState()
	clearTableBody(table)
	table.SetCell(1, col,
		tview.NewTableCell(text).
			SetTextColor(color).
			SetExpansion(3),
	)
	table.SetTitle(title)
}
```

### 2. Per-view rewrite (mechanical, same shape 5×)

`ssmparams.go` (representative — `secrets.go`/`logs.go` identical
except the view name/title text; `codepipelinelist.go` uses column 0
and a static title; `codepipelinedetail.go` uses column 0, a
per-pipeline dynamic title, and nulls one field instead of two):

```go
func (pv *SSMParamsView) showError(err error) {
	showStatusCell(pv.table, 1, fmt.Sprintf("Error: %v", err), tcell.ColorRed, " SSM Parameters ", func() {
		pv.all = nil
		pv.filtered = nil
	})
}

// showStatus displays an in-progress, non-error message (e.g. while an
// SSO re-auth is running) — same shape as showError but accent-colored
// so it doesn't read as a failure.
func (pv *SSMParamsView) showStatus(msg string) {
	showStatusCell(pv.table, 1, msg, tcell.GetColor(pv.host.Config().Colors.Accent), " SSM Parameters ", func() {
		pv.all = nil
		pv.filtered = nil
	})
}
```

Every other clear-loop call site (`repaint()` in all 5 views; `Open()`
and `Render()` in `codepipelinedetail.go`) has its inline `for
table.GetRowCount() > 1 { ... }` replaced with `clearTableBody(table)`
— a one-line swap, nothing else in those methods changes.

`codepipelinedetail.go`'s pair, for the shape that differs most:

```go
func (dv *CodePipelineDetailView) showError(err error) {
	showStatusCell(dv.table, 0, fmt.Sprintf("Error: %v", err), tcell.ColorRed, fmt.Sprintf(" %s ", dv.pipelineName), func() {
		dv.stages = nil
	})
}

func (dv *CodePipelineDetailView) showStatus(msg string) {
	showStatusCell(dv.table, 0, msg, tcell.GetColor(dv.host.Config().Colors.Accent), fmt.Sprintf(" %s ", dv.pipelineName), func() {
		dv.stages = nil
	})
}
```

## Files touched

- `tui/internal/view/statuscell.go` (new) + `statuscell_test.go` (new).
- `tui/internal/view/{ssmparams,secrets,logs,codepipelinelist,
  codepipelinedetail}.go` — `showError`/`showStatus` rewritten;
  `repaint()` (all 5) and `Open()`/`Render()`
  (`codepipelinedetail.go` only) get their inline clear-loop replaced
  with `clearTableBody(table)`.
- No `_test.go` changes expected for the 5 views — same acceptance
  bar as the load/reauth dedup CR: existing assertions on rendered
  cell text/color/title should keep passing unmodified, since the
  rendered output is identical, only how it's produced changes.

## Testing

- New `statuscell_test.go`: `clearTableBody` (removes all rows down
  to just the header, no-ops on an already-header-only table) and
  `showStatusCell` (writes the right text/color/title at the right
  column, calls `clearState` before touching the table — order
  matters if `clearState` itself reads table state, though none of
  the 5 current callers' closures do).
- Full existing suite for the 5 views must keep passing unchanged —
  the acceptance bar for "this refactor didn't change behavior."
- `go build`/`go vet`/`go test ./...` after each task, `gofmt` before
  each commit.

## Key decisions / trade-offs

- **`clearState func()` stays per-call-site, not folded into a
  generic "reset view state" interface.** Each view nulls different
  fields of different types (`[]awsssm.Parameter`, `[]awscodepipeline.
  StageStatus`, ...) — a closure is the simplest way to let
  `showStatusCell` trigger that without `statuscell.go` knowing
  anything about any specific view's struct shape.
- **`queues.go` excluded** — see `spec.md`'s "Out of scope" for the
  concrete differences (no title update, no state nulling, different
  `SetExpansion` value) that make it not a clean fit for the same
  helper without adding parameters only one caller would use.
- **`clearTableBody` extracted as its own function, not inlined into
  `showStatusCell`**, because it also replaces the clear-loop inside
  `repaint()`/`Open()`/`Render()` — call sites that have nothing to do
  with the status-cell shape at all, just the row-clearing idiom.
- **Task order**: helper first (compiles standalone, nothing depends
  on it yet), then one task per view (each an independent, isolated,
  easily-reviewable diff, matching every prior CR's breakdown this
  session), then merge-back.
