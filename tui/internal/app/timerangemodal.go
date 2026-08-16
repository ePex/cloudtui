package app

import (
	"fmt"
	"time"

	"github.com/rivo/tview"
)

// showTimeRangeModal opens the shared Relative/Absolute time range overlay,
// prefilled from the calling view's current timeRange (spec/53), and stores
// onApply to be called with the user's selection once they apply (a relative
// preset click, or the Absolute tab's Apply button). Unlike showMessageFilter
// (single-view-scoped), this modal is reused by both logSearchView and
// datadogLogsView, so it has no view-specific state of its own — the caller
// supplies current/onApply each time.
func (a *App) showTimeRangeModal(current timeRange, onApply func(timeRange)) {
	a.timeRangeOnApply = onApply

	// Prefill both tabs unconditionally (mirrors showMessageFilter filling
	// all fields from mv.filter every time): current.presetIdx/from/to are
	// zero-valued for whichever mode isn't active, which harmlessly clears
	// the other tab's stale state from a previous view/session instead of
	// leaving it stuck on whatever was there last.
	a.timeRangeRelativeList.SetCurrentItem(current.presetIdx)
	a.timeRangeAbsoluteForm.GetFormItem(0).(*tview.InputField).SetText(formatTimeRangeDateTime(current.from))
	a.timeRangeAbsoluteForm.GetFormItem(1).(*tview.InputField).SetText(formatTimeRangeDateTime(current.to))

	a.rootPages.ShowPage("time-range")
	a.timeRangeVisible = true
	a.switchTimeRangeTab(current.mode)
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

// closeTimeRangeModal hides the overlay without applying anything. Focus
// goes to a.pages rather than a specific view's table (mirrors
// closeThemePicker, not closeMessageFilter) — this modal is shared across
// views, so it has no single "the table" to return to; a.pages resolves to
// whichever view is currently the front page.
func (a *App) closeTimeRangeModal() {
	a.rootPages.HidePage("time-range")
	a.timeRangeVisible = false
	a.tv.SetFocus(a.pages)
}

// switchTimeRangeTab activates mode's tab: re-renders the tab indicator,
// switches timeRangePages, and focuses the tab's primitive (the relative
// list or the absolute form). Called both on open and by the 'R'/'A'
// shortcuts while the modal is visible.
func (a *App) switchTimeRangeTab(mode timeRangeMode) {
	a.timeRangeActiveTab = mode
	a.renderTimeRangeTabs()
	switch mode {
	case timeRangeRelative:
		a.timeRangePages.SwitchToPage("relative")
		a.tv.SetFocus(a.timeRangeRelativeList)
	case timeRangeAbsolute:
		a.timeRangePages.SwitchToPage("absolute")
		// tview.Form remembers its internal focus index across SetFocus
		// calls (caught live via verify-live, spec/53) — without resetting
		// it to 0 (the From field) here, reopening the modal after a
		// previous Absolute-tab session that ended on the Apply/Cancel
		// button would silently leave keystrokes going nowhere useful.
		a.timeRangeAbsoluteForm.SetFocus(0)
		a.tv.SetFocus(a.timeRangeAbsoluteForm)
	}
}

// renderTimeRangeTabs redraws the tab indicator, bolding the active tab's
// label in the theme's accent color — a real "[color]...[-]" tag, not
// literal brackets (those are swallowed by tview's tag parser, same gotcha
// documented on queues.go's updateTitle).
func (a *App) renderTimeRangeTabs() {
	accent := a.cfg.Colors.Accent
	text := a.cfg.Colors.Text
	relative, absolute := "Relative (R)", "Absolute (A)"
	if a.timeRangeActiveTab == timeRangeRelative {
		relative = fmt.Sprintf("[%s::b]%s[-:-:-]", accent, relative)
		absolute = fmt.Sprintf("[%s]%s[-]", text, absolute)
	} else {
		relative = fmt.Sprintf("[%s]%s[-]", text, relative)
		absolute = fmt.Sprintf("[%s::b]%s[-:-:-]", accent, absolute)
	}
	a.timeRangeTabs.SetText(fmt.Sprintf("  %s    %s  ", relative, absolute))
}

// applyTimeRangeRelative builds a relative timeRange for presetIdx, closes
// the modal, and hands it to timeRangeOnApply — wired as each
// timeRangeRelativeList item's selected-func, so selecting a preset applies
// immediately (no separate Apply step, unlike the Absolute tab).
func (a *App) applyTimeRangeRelative(presetIdx int) {
	tr := timeRange{mode: timeRangeRelative, presetIdx: presetIdx}
	a.closeTimeRangeModal()
	a.timeRangeOnApply(tr)
}

// applyTimeRangeAbsolute parses the Absolute tab's From/Until fields via the
// shared parseFilterDate (RFC3339, local "YYYY-MM-DD HH:MM", or UTC bare
// "YYYY-MM-DD" — see messages.go). On a parse error, the status bar
// reports it and the modal stays open for correction — same pattern as
// applyMessageFilter.
func (a *App) applyTimeRangeAbsolute() {
	from := a.timeRangeAbsoluteForm.GetFormItem(0).(*tview.InputField).GetText()
	until := a.timeRangeAbsoluteForm.GetFormItem(1).(*tview.InputField).GetText()

	fromT, err := parseFilterDate("from", from)
	if err != nil {
		a.statusBar.SetText(fmt.Sprintf("[red]%s[-]", err))
		return
	}
	untilT, err := parseFilterDate("until", until)
	if err != nil {
		a.statusBar.SetText(fmt.Sprintf("[red]%s[-]", err))
		return
	}

	tr := timeRange{mode: timeRangeAbsolute, from: fromT, to: untilT}
	a.closeTimeRangeModal()
	a.timeRangeOnApply(tr)
}
