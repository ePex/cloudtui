package dialog

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// TimeRangeModal is the shared Relative/Absolute time range overlay
// (spec/53), used by both logSearchView and datadogLogsView. Unlike most
// other overlays it has no view-specific state of its own — the caller
// supplies current/onApply each time Show is called. Named TimeRangeModal,
// not TimeRange, to avoid shadowing ui.TimeRange, the value type it edits.
type TimeRangeModal struct {
	host         ui.Host
	flex         *tview.Flex
	tabs         *tview.TextView
	pages        *tview.Pages
	relativeList *tview.List
	absoluteForm *tview.Form
	visible      bool
	activeTab    ui.TimeRangeMode
	onApply      func(ui.TimeRange)
}

// NewTimeRangeModal builds the time range overlay's widgets.
func NewTimeRangeModal(host ui.Host) *TimeRangeModal {
	tm := &TimeRangeModal{host: host}
	tm.tabs = tview.NewTextView().SetDynamicColors(true)
	tm.relativeList = tview.NewList().ShowSecondaryText(false)
	for i, p := range ui.TimeRangePresets {
		idx := i
		tm.relativeList.AddItem(p.Label, "", 0, func() {
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
			tm.switchTab(ui.TimeRangeRelative)
			return nil
		case event.Rune() == 'A':
			tm.switchTab(ui.TimeRangeAbsolute)
			return nil
		}
		return event
	})
	return tm
}

// Show opens the shared Relative/Absolute time range overlay, prefilled
// from the calling view's current ui.TimeRange (spec/53), and stores
// onApply to be called with the user's selection once they apply (a
// relative preset click, or the Absolute tab's Apply button).
func (tm *TimeRangeModal) Show(current ui.TimeRange, onApply func(ui.TimeRange)) {
	tm.onApply = onApply

	// Prefill both tabs unconditionally (mirrors MessageFilter.Show
	// filling all fields from mv.filter every time): current.PresetIdx/
	// From/To are zero-valued for whichever mode isn't active, which
	// harmlessly clears the other tab's stale state from a previous
	// view/session instead of leaving it stuck on whatever was there last.
	tm.relativeList.SetCurrentItem(current.PresetIdx)
	tm.absoluteForm.GetFormItem(0).(*tview.InputField).SetText(formatTimeRangeDateTime(current.From))
	tm.absoluteForm.GetFormItem(1).(*tview.InputField).SetText(formatTimeRangeDateTime(current.To))

	tm.host.ShowPage("time-range")
	tm.visible = true
	tm.switchTab(current.Mode)
}

// formatTimeRangeDateTime renders t for the Absolute tab's From/Until
// fields, or "" for a zero value (unset). Unlike formatFilterDate (used by
// the message filter, date-only), this keeps minute precision — the same
// "2006-01-02 15:04" layout ui.TimeRange.Label() already renders an absolute
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
func (tm *TimeRangeModal) close() {
	tm.host.HidePage("time-range")
	tm.visible = false
	tm.host.FocusMain()
}

// ApplyPalette recolors the time range overlay for a live theme switch.
func (tm *TimeRangeModal) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	tm.flex.SetBackgroundColor(bg)
	tm.flex.SetBorderColor(tcell.GetColor(p.Border))
	tm.flex.SetTitleColor(tcell.GetColor(p.Border))
	tm.tabs.SetBackgroundColor(bg)
	tm.renderTabs()
	tm.pages.SetBackgroundColor(bg)
	ui.StyleList(tm.relativeList, p)
	tm.relativeList.SetBackgroundColor(bg)
	tm.absoluteForm.SetBackgroundColor(bg)
	tm.absoluteForm.SetBorderColor(tcell.GetColor(p.Border))
	tm.absoluteForm.SetTitleColor(tcell.GetColor(p.Border))
}

var _ ui.Themeable = (*TimeRangeModal)(nil)

// Primitive returns TimeRangeModal's root widget, for sizing/embedding.
func (tm *TimeRangeModal) Primitive() tview.Primitive { return tm.flex }

// Visible reports whether TimeRangeModal is currently shown.
func (tm *TimeRangeModal) Visible() bool { return tm.visible }

// switchTab activates mode's tab: re-renders the tab indicator, switches
// pages, and focuses the tab's primitive (the relative list or the
// absolute form). Called both on open and by the 'R'/'A' shortcuts while
// the modal is visible.
func (tm *TimeRangeModal) switchTab(mode ui.TimeRangeMode) {
	tm.activeTab = mode
	tm.renderTabs()
	switch mode {
	case ui.TimeRangeRelative:
		tm.pages.SwitchToPage("relative")
		tm.host.SetFocus(tm.relativeList)
	case ui.TimeRangeAbsolute:
		tm.pages.SwitchToPage("absolute")
		// tview.Form remembers its internal focus index across SetFocus
		// calls (caught live via verify-live, spec/53) — without resetting
		// it to 0 (the From field) here, reopening the modal after a
		// previous Absolute-tab session that ended on the Apply/Cancel
		// button would silently leave keystrokes going nowhere useful.
		tm.absoluteForm.SetFocus(0)
		tm.host.SetFocus(tm.absoluteForm)
	}
}

// renderTabs redraws the tab indicator, bolding the active tab's label in
// the theme's accent color — a real "[color]...[-]" tag, not literal
// brackets (those are swallowed by tview's tag parser, same gotcha
// documented on queues.go's updateTitle).
func (tm *TimeRangeModal) renderTabs() {
	colors := tm.host.Config().Colors
	accent := colors.Accent
	text := colors.Text
	relative, absolute := "Relative (R)", "Absolute (A)"
	if tm.activeTab == ui.TimeRangeRelative {
		relative = fmt.Sprintf("[%s::b]%s[-:-:-]", accent, relative)
		absolute = fmt.Sprintf("[%s]%s[-]", text, absolute)
	} else {
		relative = fmt.Sprintf("[%s]%s[-]", text, relative)
		absolute = fmt.Sprintf("[%s::b]%s[-:-:-]", accent, absolute)
	}
	tm.tabs.SetText(fmt.Sprintf("  %s    %s  ", relative, absolute))
}

// applyRelative builds a relative ui.TimeRange for presetIdx, closes the
// modal, and hands it to onApply — wired as each relativeList item's
// selected-func, so selecting a preset applies immediately (no separate
// Apply step, unlike the Absolute tab).
func (tm *TimeRangeModal) applyRelative(presetIdx int) {
	tr := ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: presetIdx}
	tm.close()
	tm.onApply(tr)
}

// applyAbsolute parses the Absolute tab's From/Until fields via the
// shared parseFilterDate (RFC3339, local "YYYY-MM-DD HH:MM", or UTC bare
// "YYYY-MM-DD" — see messages.go). On a parse error, the status bar
// reports it and the modal stays open for correction — same pattern as
// messageFilter.apply.
func (tm *TimeRangeModal) applyAbsolute() {
	from := tm.absoluteForm.GetFormItem(0).(*tview.InputField).GetText()
	until := tm.absoluteForm.GetFormItem(1).(*tview.InputField).GetText()

	fromT, err := ui.ParseFilterDate("from", from)
	if err != nil {
		tm.host.SetStatus(fmt.Sprintf("[red]%s[-]", err))
		return
	}
	untilT, err := ui.ParseFilterDate("until", until)
	if err != nil {
		tm.host.SetStatus(fmt.Sprintf("[red]%s[-]", err))
		return
	}

	tr := ui.TimeRange{Mode: ui.TimeRangeAbsolute, From: fromT, To: untilT}
	tm.close()
	tm.onApply(tr)
}
