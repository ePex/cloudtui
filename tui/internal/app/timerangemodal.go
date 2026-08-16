package app

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// timeRangeModal is the shared Relative/Absolute time range overlay
// (spec/53), used by both logSearchView and datadogLogsView. Unlike most
// other overlays it has no view-specific state of its own — the caller
// supplies current/onApply each time show is called. Named timeRangeModal,
// not timeRange, to avoid shadowing the timeRange value type it edits.
type timeRangeModal struct {
	app          *App
	flex         *tview.Flex
	tabs         *tview.TextView
	pages        *tview.Pages
	relativeList *tview.List
	absoluteForm *tview.Form
	visible      bool
	activeTab    timeRangeMode
	onApply      func(timeRange)
}

// newTimeRangeModal builds the time range overlay's widgets.
func newTimeRangeModal(a *App) *timeRangeModal {
	tm := &timeRangeModal{app: a}
	tm.tabs = tview.NewTextView().SetDynamicColors(true)
	tm.relativeList = tview.NewList().ShowSecondaryText(false)
	for i, p := range timeRangePresets {
		idx := i
		tm.relativeList.AddItem(p.label, "", 0, func() {
			tm.applyRelative(idx)
		})
	}
	tm.relativeList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	tm.absoluteForm = tview.NewForm()
	tm.absoluteForm.
		AddInputField("From (YYYY-MM-DD HH:MM or RFC3339)", "", 30, nil, nil).
		AddInputField("Until (YYYY-MM-DD HH:MM or RFC3339)", "", 30, nil, nil).
		AddButton("Apply", func() { tm.applyAbsolute() }).
		AddButton("Cancel", func() { tm.close() })

	tm.pages = tview.NewPages().
		AddPage("relative", tm.relativeList, true, true).
		AddPage("absolute", tm.absoluteForm, true, false)

	tm.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tm.tabs, 1, 0, false).
		AddItem(tm.pages, 0, 1, true)
	tm.flex.SetBorder(true).SetTitle(" Time Range ")
	// Set on the Flex itself (fires before descending to whichever child —
	// the relative list or the absolute form/its fields — currently has
	// focus), not on the individual tabs: 'R'/'A' must switch tabs
	// regardless of which one is focused. Uppercase specifically to avoid
	// colliding with typed text in the Absolute tab's date fields (same
	// reasoning as 'S'/'E' in datadoglogs.go); Left/Right were ruled out
	// because tview.InputField already consumes those for in-field cursor
	// movement while a date field has focus.
	tm.flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape:
			tm.close()
			return nil
		case event.Rune() == 'R':
			tm.switchTab(timeRangeRelative)
			return nil
		case event.Rune() == 'A':
			tm.switchTab(timeRangeAbsolute)
			return nil
		}
		return event
	})
	return tm
}

// show opens the shared Relative/Absolute time range overlay, prefilled
// from the calling view's current timeRange (spec/53), and stores onApply
// to be called with the user's selection once they apply (a relative
// preset click, or the Absolute tab's Apply button).
func (tm *timeRangeModal) show(current timeRange, onApply func(timeRange)) {
	tm.onApply = onApply

	// Prefill both tabs unconditionally (mirrors messageFilter.show
	// filling all fields from mv.filter every time): current.presetIdx/
	// from/to are zero-valued for whichever mode isn't active, which
	// harmlessly clears the other tab's stale state from a previous
	// view/session instead of leaving it stuck on whatever was there last.
	tm.relativeList.SetCurrentItem(current.presetIdx)
	tm.absoluteForm.GetFormItem(0).(*tview.InputField).SetText(formatTimeRangeDateTime(current.from))
	tm.absoluteForm.GetFormItem(1).(*tview.InputField).SetText(formatTimeRangeDateTime(current.to))

	tm.app.rootPages.ShowPage("time-range")
	tm.visible = true
	tm.switchTab(current.mode)
}

// formatTimeRangeDateTime renders t for the Absolute tab's From/Until
// fields, or "" for a zero value (unset). Unlike formatFilterDate (used by
// the message filter, date-only), this keeps minute precision — the same
// "2006-01-02 15:04" layout timeRange.label() already renders an absolute
// range with, and the same layout parseFilterDate accepts back (as local
// time — see its doc comment), so a value round-trips through
// prefill/parse without losing or shifting the time of day. t is stored
// internally as UTC (parseFilterDate's t.UTC()); .Local() here is what
// makes the round trip actually land back on what the user typed, instead
// of the UTC-shifted equivalent.
func formatTimeRangeDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// close hides the overlay without applying anything. Focus goes to
// app.pages rather than a specific view's table (mirrors themePicker's
// close, not messageFilter's) — this modal is shared across views, so it
// has no single "the table" to return to; app.pages resolves to whichever
// view is currently the front page.
func (tm *timeRangeModal) close() {
	tm.app.rootPages.HidePage("time-range")
	tm.visible = false
	tm.app.tv.SetFocus(tm.app.pages)
}

// switchTab activates mode's tab: re-renders the tab indicator, switches
// pages, and focuses the tab's primitive (the relative list or the
// absolute form). Called both on open and by the 'R'/'A' shortcuts while
// the modal is visible.
func (tm *timeRangeModal) switchTab(mode timeRangeMode) {
	tm.activeTab = mode
	tm.renderTabs()
	switch mode {
	case timeRangeRelative:
		tm.pages.SwitchToPage("relative")
		tm.app.tv.SetFocus(tm.relativeList)
	case timeRangeAbsolute:
		tm.pages.SwitchToPage("absolute")
		// tview.Form remembers its internal focus index across SetFocus
		// calls (caught live via verify-live, spec/53) — without resetting
		// it to 0 (the From field) here, reopening the modal after a
		// previous Absolute-tab session that ended on the Apply/Cancel
		// button would silently leave keystrokes going nowhere useful.
		tm.absoluteForm.SetFocus(0)
		tm.app.tv.SetFocus(tm.absoluteForm)
	}
}

// renderTabs redraws the tab indicator, bolding the active tab's label in
// the theme's accent color — a real "[color]...[-]" tag, not literal
// brackets (those are swallowed by tview's tag parser, same gotcha
// documented on queues.go's updateTitle).
func (tm *timeRangeModal) renderTabs() {
	accent := tm.app.cfg.Colors.Accent
	text := tm.app.cfg.Colors.Text
	relative, absolute := "Relative (R)", "Absolute (A)"
	if tm.activeTab == timeRangeRelative {
		relative = fmt.Sprintf("[%s::b]%s[-:-:-]", accent, relative)
		absolute = fmt.Sprintf("[%s]%s[-]", text, absolute)
	} else {
		relative = fmt.Sprintf("[%s]%s[-]", text, relative)
		absolute = fmt.Sprintf("[%s::b]%s[-:-:-]", accent, absolute)
	}
	tm.tabs.SetText(fmt.Sprintf("  %s    %s  ", relative, absolute))
}

// applyRelative builds a relative timeRange for presetIdx, closes the
// modal, and hands it to onApply — wired as each relativeList item's
// selected-func, so selecting a preset applies immediately (no separate
// Apply step, unlike the Absolute tab).
func (tm *timeRangeModal) applyRelative(presetIdx int) {
	tr := timeRange{mode: timeRangeRelative, presetIdx: presetIdx}
	tm.close()
	tm.onApply(tr)
}

// applyAbsolute parses the Absolute tab's From/Until fields via the
// shared parseFilterDate (RFC3339, local "YYYY-MM-DD HH:MM", or UTC bare
// "YYYY-MM-DD" — see messages.go). On a parse error, the status bar
// reports it and the modal stays open for correction — same pattern as
// messageFilter.apply.
func (tm *timeRangeModal) applyAbsolute() {
	from := tm.absoluteForm.GetFormItem(0).(*tview.InputField).GetText()
	until := tm.absoluteForm.GetFormItem(1).(*tview.InputField).GetText()

	fromT, err := parseFilterDate("from", from)
	if err != nil {
		tm.app.statusBar.SetText(fmt.Sprintf("[red]%s[-]", err))
		return
	}
	untilT, err := parseFilterDate("until", until)
	if err != nil {
		tm.app.statusBar.SetText(fmt.Sprintf("[red]%s[-]", err))
		return
	}

	tr := timeRange{mode: timeRangeAbsolute, from: fromT, to: untilT}
	tm.close()
	tm.onApply(tr)
}
