package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

func TestShowTimeRangeModalRelativePrefill(t *testing.T) {
	a := New(config.Default())
	current := ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 3}

	a.timeRangeModal.Show(current, func(ui.TimeRange) {})

	if !a.timeRangeModal.visible {
		t.Fatal("timeRangeModal.Show() did not open the overlay")
	}
	if a.timeRangeModal.activeTab != ui.TimeRangeRelative {
		t.Errorf("timeRangeModal.activeTab = %v, want timeRangeRelative", a.timeRangeModal.activeTab)
	}
	if got := a.timeRangeModal.relativeList.GetCurrentItem(); got != 3 {
		t.Errorf("relative list current item = %d, want 3", got)
	}
}

func TestShowTimeRangeModalAbsolutePrefill(t *testing.T) {
	a := New(config.Default())
	// Non-midnight times so the assertions also prove the time of day
	// survives the round trip, not just the date (spec/53 revision — the
	// Absolute tab must support setting a time, not only a date).
	// Constructed in time.Local (not UTC): formatTimeRangeDateTime
	// renders via .Local(), so this is what makes the expected wall-clock
	// strings below correct regardless of the machine's timezone.
	from := time.Date(2026, 8, 1, 9, 30, 0, 0, time.Local)
	to := time.Date(2026, 8, 2, 17, 45, 0, 0, time.Local)
	current := ui.TimeRange{Mode: ui.TimeRangeAbsolute, From: from, To: to}

	a.timeRangeModal.Show(current, func(ui.TimeRange) {})

	if a.timeRangeModal.activeTab != ui.TimeRangeAbsolute {
		t.Errorf("timeRangeModal.activeTab = %v, want timeRangeAbsolute", a.timeRangeModal.activeTab)
	}
	fromText := a.timeRangeModal.absoluteForm.GetFormItem(0).(*tview.InputField).GetText()
	untilText := a.timeRangeModal.absoluteForm.GetFormItem(1).(*tview.InputField).GetText()
	if fromText != "2026-08-01 09:30" {
		t.Errorf("From field = %q, want %q", fromText, "2026-08-01 09:30")
	}
	if untilText != "2026-08-02 17:45" {
		t.Errorf("Until field = %q, want %q", untilText, "2026-08-02 17:45")
	}
}

func TestCloseTimeRangeModal(t *testing.T) {
	a := New(config.Default())
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(ui.TimeRange) {})

	a.timeRangeModal.close()

	if a.timeRangeModal.visible {
		t.Error("timeRangeModal.close() left visible true")
	}
}

func TestTimeRangeModalEscapeCloses(t *testing.T) {
	a := New(config.Default())
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(ui.TimeRange) {})

	capture := a.timeRangeModal.flex.GetInputCapture()
	if capture == nil {
		t.Fatal("timeRangeModal.flex has no input capture set")
	}
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if a.timeRangeModal.visible {
		t.Error("Esc did not close the time range modal")
	}
}

func TestTimeRangeModalEscapeDoesNotApply(t *testing.T) {
	a := New(config.Default())
	applied := false
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 1}, func(ui.TimeRange) {
		applied = true
	})

	a.timeRangeModal.flex.GetInputCapture()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if applied {
		t.Error("Esc must not call onApply")
	}
}

func TestTimeRangeModalTabSwitching(t *testing.T) {
	a := New(config.Default())
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(ui.TimeRange) {})
	capture := a.timeRangeModal.flex.GetInputCapture()

	if got := capture(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone)); got != nil {
		t.Errorf("'A' capture returned %v, want nil (event consumed)", got)
	}
	if a.timeRangeModal.activeTab != ui.TimeRangeAbsolute {
		t.Errorf("timeRangeModal.activeTab = %v, want timeRangeAbsolute after 'A'", a.timeRangeModal.activeTab)
	}

	if got := capture(tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone)); got != nil {
		t.Errorf("'R' capture returned %v, want nil (event consumed)", got)
	}
	if a.timeRangeModal.activeTab != ui.TimeRangeRelative {
		t.Errorf("timeRangeModal.activeTab = %v, want timeRangeRelative after 'R'", a.timeRangeModal.activeTab)
	}
}

// TestSwitchTimeRangeTabResetsAbsoluteFormFocus guards a bug caught live
// (verify-live, spec/53): tview.Form remembers its last-focused item
// across Application.SetFocus calls, so reopening the modal after a
// previous Absolute-tab session that ended on Apply/Cancel would leave
// keystrokes going to the button instead of the From field, unless
// timeRangeModal.switchTab explicitly resets the form's internal focus to 0.
func TestSwitchTimeRangeTabResetsAbsoluteFormFocus(t *testing.T) {
	a := New(config.Default())
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeAbsolute}, func(ui.TimeRange) {})
	a.timeRangeModal.absoluteForm.SetFocus(2) // simulate a prior session ending on the Apply button

	a.timeRangeModal.switchTab(ui.TimeRangeAbsolute)

	item, button := a.timeRangeModal.absoluteForm.GetFocusedItemIndex()
	if item != 0 || button != -1 {
		t.Errorf("form focus = (item=%d, button=%d), want (item=0, button=-1) — the From field", item, button)
	}
}

func TestTimeRangeModalOtherKeysPassThrough(t *testing.T) {
	a := New(config.Default())
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(ui.TimeRange) {})

	capture := a.timeRangeModal.flex.GetInputCapture()
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	if got := capture(event); got != event {
		t.Errorf("capture() altered/swallowed a non-Esc/R/A key: got %v, want it passed through unchanged", got)
	}
}

func TestApplyTimeRangeRelative(t *testing.T) {
	a := New(config.Default())
	var got ui.TimeRange
	applied := false
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(tr ui.TimeRange) {
		applied = true
		got = tr
	})

	a.timeRangeModal.applyRelative(2)

	if !applied {
		t.Fatal("timeRangeModal.applyRelative() did not call onApply")
	}
	if got.Mode != ui.TimeRangeRelative || got.PresetIdx != 2 {
		t.Errorf("onApply got %+v, want mode=relative presetIdx=2", got)
	}
	if a.timeRangeModal.visible {
		t.Error("timeRangeModal.applyRelative() did not close the modal")
	}
}

func TestApplyTimeRangeAbsoluteValid(t *testing.T) {
	a := New(config.Default())
	var got ui.TimeRange
	applied := false
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeAbsolute}, func(tr ui.TimeRange) {
		applied = true
		got = tr
	})
	a.timeRangeModal.absoluteForm.GetFormItem(0).(*tview.InputField).SetText("2026-08-01")
	a.timeRangeModal.absoluteForm.GetFormItem(1).(*tview.InputField).SetText("2026-08-02")

	a.timeRangeModal.applyAbsolute()

	if !applied {
		t.Fatal("timeRangeModal.applyAbsolute() did not call onApply")
	}
	wantFrom := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if got.Mode != ui.TimeRangeAbsolute || !got.From.Equal(wantFrom) || !got.To.Equal(wantTo) {
		t.Errorf("onApply got %+v, want mode=absolute from=%v to=%v", got, wantFrom, wantTo)
	}
	if a.timeRangeModal.visible {
		t.Error("timeRangeModal.applyAbsolute() did not close the modal")
	}
}

// TestApplyTimeRangeAbsoluteWithTime covers spec/53's revision: the
// Absolute tab must support setting a time, not only a date.
func TestApplyTimeRangeAbsoluteWithTime(t *testing.T) {
	a := New(config.Default())
	var got ui.TimeRange
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeAbsolute}, func(tr ui.TimeRange) {
		got = tr
	})
	a.timeRangeModal.absoluteForm.GetFormItem(0).(*tview.InputField).SetText("2026-08-01 09:30")
	a.timeRangeModal.absoluteForm.GetFormItem(1).(*tview.InputField).SetText("2026-08-02 17:45")

	a.timeRangeModal.applyAbsolute()

	// Typed values are interpreted as local time, not UTC (see
	// parseFilterDate's doc comment) — built via time.Local, not a
	// hardcoded UTC offset, so this passes regardless of the machine's
	// timezone.
	wantFrom := time.Date(2026, 8, 1, 9, 30, 0, 0, time.Local)
	wantTo := time.Date(2026, 8, 2, 17, 45, 0, 0, time.Local)
	if !got.From.Equal(wantFrom) || !got.To.Equal(wantTo) {
		t.Errorf("onApply got from=%v to=%v, want from=%v to=%v", got.From, got.To, wantFrom, wantTo)
	}
}

func TestApplyTimeRangeAbsoluteInvalidDate(t *testing.T) {
	a := New(config.Default())
	applied := false
	a.timeRangeModal.Show(ui.TimeRange{Mode: ui.TimeRangeAbsolute}, func(ui.TimeRange) {
		applied = true
	})
	a.timeRangeModal.absoluteForm.GetFormItem(0).(*tview.InputField).SetText("not-a-date")
	a.timeRangeModal.absoluteForm.GetFormItem(1).(*tview.InputField).SetText("")

	a.timeRangeModal.applyAbsolute()

	if applied {
		t.Error("timeRangeModal.applyAbsolute() called onApply despite an invalid date")
	}
	if !a.timeRangeModal.visible {
		t.Error("timeRangeModal.applyAbsolute() closed the modal despite a parse error")
	}
	if got := a.statusBar.GetText(true); !strings.Contains(got, "invalid from") {
		t.Errorf("statusBar = %q, want it to mention the invalid \"from\" field", got)
	}
}
