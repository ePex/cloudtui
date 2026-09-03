# Plan

## New helper

`tui/internal/ui/inputfield.go` (new file, alongside `style.go`'s
existing small `tview` widget helpers):

```go
package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SetInputFieldText populates field's text and works around a tview
// v0.42.0 bug: InputField.SetText -> TextArea.Replace calls
// findCursor(false, row), which recomputes the cursor's true position
// but skips scrolling columnOffset to keep it visible. A value longer
// than the field's width then renders with the cursor hidden and the
// field still showing its start, not its end.
//
// Firing a synthetic End keypress through the field's own input
// handler takes the same code path a real keypress would: InputField
// forwards KeyEnd to its TextArea, whose moveCursor() always ends
// with findCursor(true, row) - the clamping variant. Box's
// WrapInputHandler has no focus guard, so this works even when the
// field doesn't currently have focus (the common case right after
// populating a form).
func SetInputFieldText(field *tview.InputField, text string) {
	field.SetText(text)
	field.InputHandler()(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone), func(tview.Primitive) {})
}
```

## Call sites switched from `field.SetText(x)` to `ui.SetInputFieldText(field, x)`

Only sites restoring a previously-stored, potentially-long value.
Sites that clear a field to `""` or set a fixed short label are left
alone (nothing to scroll to).

- `tui/internal/dialog/connections.go`: lines 281 (Name), 311 (Broker
  Name), 314 (URL), 315 (Username), 327 (AWS Profile), 338 (Secret
  Name), 340 (Password) — 7 sites. (Line 96, `cm.hints.SetText(...)`,
  is a `*tview.TextView`, not an `InputField` — not in scope.)
- `tui/internal/dialog/datadogsettings.go`: lines 48 (Site), 49
  (Access Token) — 2 sites.
- `tui/internal/dialog/messagefilter.go`: lines 176 (JMS Type), 184
  (From Date), 185 (To Date), 190 (Max Count) — 4 sites. (Lines 159
  and 260 clear fields to `""` — not in scope.)
- `tui/internal/dialog/timerangemodal.go`: lines 105, 106 (From/To
  datetime) — 2 sites. (Line 204, `tm.tabs.SetText(...)`, is a
  `*tview.TextView` — not in scope.)
- `tui/internal/view/logsearch.go`: lines 141, 231 (restored search
  pattern) — 2 sites.
- `tui/internal/view/datadoglogs.go`: line 200 (restored query) — 1
  site.
- `tui/internal/view/messages.go`: line 192 (restored quick search) —
  1 site. (Line 103 clears to `""` — not in scope.)
- `tui/internal/view/logs.go`: line 111 (restored filter) — 1 site.
- `tui/internal/view/queues.go`: line 131 (restored filter) — 1 site.
- `tui/internal/view/ssmparams.go`: line 109 (restored filter) — 1
  site.
- `tui/internal/view/secrets.go`: line 108 (restored filter) — 1 site.
- `tui/internal/view/codepipelinelist.go`: line 117 (restored filter)
  — 1 site.

24 call sites total across 12 files, plus the new helper.

## Testing

`tui/internal/ui/inputfield_test.go`: build a narrow `*tview.InputField`
(`SetFieldWidth` smaller than a test value's length), call
`SetInputFieldText` with a value long enough to overflow it, `SetRect`
it, `Draw` it onto a `tcell.NewSimulationScreen("")`, then assert via
the screen's `GetCursor() (x, y int, visible bool)` that the cursor is
reported visible — this is a real, black-box test of the actual bug
(no live terminal needed; `SimulationScreen` is the same one `tview`
itself uses in its own tests). A second case with a value shorter than
the field width is included as a control (cursor already visible
either way, confirming the helper doesn't do anything harmful for the
non-overflowing case).

## Task breakdown approach

One task for the new helper + its test, then group the 21 call-site
updates by file into a handful of tasks (dialog package together,
view-package filter inputs together) rather than 21 separate
one-line tasks — see `tasks.md`.
