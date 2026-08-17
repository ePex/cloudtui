package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/ui"
)

func newTestTimeRangeModal(t *testing.T) (*TimeRangeModal, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewTimeRangeModal(host), host
}

func TestShowTimeRangeModalRelativePrefill(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	current := ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 3}

	tm.Show(current, func(ui.TimeRange) {})

	if !tm.visible {
		t.Fatal("TimeRangeModal.Show() did not open the overlay")
	}
	if tm.activeTab != ui.TimeRangeRelative {
		t.Errorf("TimeRangeModal.activeTab = %v, want timeRangeRelative", tm.activeTab)
	}
	if got := tm.relativeList.GetCurrentItem(); got != 3 {
		t.Errorf("relative list current item = %d, want 3", got)
	}
}

func TestShowTimeRangeModalAbsolutePrefill(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	// Non-midnight times so the assertions also prove the time of day
	// survives the round trip, not just the date (spec/53 revision — the
	// Absolute tab must support setting a time, not only a date).
	// Constructed in time.Local (not UTC): formatTimeRangeDateTime
	// renders via .Local(), so this is what makes the expected wall-clock
	// strings below correct regardless of the machine's timezone.
	from := time.Date(2026, 8, 1, 9, 30, 0, 0, time.Local)
	to := time.Date(2026, 8, 2, 17, 45, 0, 0, time.Local)
	current := ui.TimeRange{Mode: ui.TimeRangeAbsolute, From: from, To: to}

	tm.Show(current, func(ui.TimeRange) {})

	if tm.activeTab != ui.TimeRangeAbsolute {
		t.Errorf("TimeRangeModal.activeTab = %v, want timeRangeAbsolute", tm.activeTab)
	}
	fromText := tm.absoluteForm.GetFormItem(0).(*tview.InputField).GetText()
	untilText := tm.absoluteForm.GetFormItem(1).(*tview.InputField).GetText()
	if fromText != "2026-08-01 09:30" {
		t.Errorf("From field = %q, want %q", fromText, "2026-08-01 09:30")
	}
	if untilText != "2026-08-02 17:45" {
		t.Errorf("Until field = %q, want %q", untilText, "2026-08-02 17:45")
	}
}

func TestCloseTimeRangeModal(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(ui.TimeRange) {})

	tm.close()

	if tm.visible {
		t.Error("TimeRangeModal.close() left visible true")
	}
}

func TestTimeRangeModalEscapeCloses(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(ui.TimeRange) {})

	capture := tm.flex.GetInputCapture()
	if capture == nil {
		t.Fatal("TimeRangeModal.flex has no input capture set")
	}
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if tm.visible {
		t.Error("Esc did not close the time range modal")
	}
}

func TestTimeRangeModalEscapeDoesNotApply(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	applied := false
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 1}, func(ui.TimeRange) {
		applied = true
	})

	tm.flex.GetInputCapture()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if applied {
		t.Error("Esc must not call onApply")
	}
}

func TestTimeRangeModalTabSwitching(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(ui.TimeRange) {})
	capture := tm.flex.GetInputCapture()

	if got := capture(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone)); got != nil {
		t.Errorf("'A' capture returned %v, want nil (event consumed)", got)
	}
	if tm.activeTab != ui.TimeRangeAbsolute {
		t.Errorf("TimeRangeModal.activeTab = %v, want timeRangeAbsolute after 'A'", tm.activeTab)
	}

	if got := capture(tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone)); got != nil {
		t.Errorf("'R' capture returned %v, want nil (event consumed)", got)
	}
	if tm.activeTab != ui.TimeRangeRelative {
		t.Errorf("TimeRangeModal.activeTab = %v, want timeRangeRelative after 'R'", tm.activeTab)
	}
}

// TestSwitchTimeRangeTabResetsAbsoluteFormFocus guards a bug caught live
// (verify-live, spec/53): tview.Form remembers its last-focused item
// across Application.SetFocus calls, so reopening the modal after a
// previous Absolute-tab session that ended on Apply/Cancel would leave
// keystrokes going to the button instead of the From field, unless
// timeRangeModal.switchTab explicitly resets the form's internal focus to 0.
func TestSwitchTimeRangeTabResetsAbsoluteFormFocus(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeAbsolute}, func(ui.TimeRange) {})
	tm.absoluteForm.SetFocus(2) // simulate a prior session ending on the Apply button

	tm.switchTab(ui.TimeRangeAbsolute)

	item, button := tm.absoluteForm.GetFocusedItemIndex()
	if item != 0 || button != -1 {
		t.Errorf("form focus = (item=%d, button=%d), want (item=0, button=-1) — the From field", item, button)
	}
}

func TestTimeRangeModalOtherKeysPassThrough(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(ui.TimeRange) {})

	capture := tm.flex.GetInputCapture()
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	if got := capture(event); got != event {
		t.Errorf("capture() altered/swallowed a non-Esc/R/A key: got %v, want it passed through unchanged", got)
	}
}

func TestApplyTimeRangeRelative(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	var got ui.TimeRange
	applied := false
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeRelative}, func(tr ui.TimeRange) {
		applied = true
		got = tr
	})

	tm.applyRelative(2)

	if !applied {
		t.Fatal("TimeRangeModal.applyRelative() did not call onApply")
	}
	if got.Mode != ui.TimeRangeRelative || got.PresetIdx != 2 {
		t.Errorf("onApply got %+v, want mode=relative presetIdx=2", got)
	}
	if tm.visible {
		t.Error("TimeRangeModal.applyRelative() did not close the modal")
	}
}

func TestApplyTimeRangeAbsoluteValid(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	var got ui.TimeRange
	applied := false
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeAbsolute}, func(tr ui.TimeRange) {
		applied = true
		got = tr
	})
	tm.absoluteForm.GetFormItem(0).(*tview.InputField).SetText("2026-08-01")
	tm.absoluteForm.GetFormItem(1).(*tview.InputField).SetText("2026-08-02")

	tm.applyAbsolute()

	if !applied {
		t.Fatal("TimeRangeModal.applyAbsolute() did not call onApply")
	}
	wantFrom := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if got.Mode != ui.TimeRangeAbsolute || !got.From.Equal(wantFrom) || !got.To.Equal(wantTo) {
		t.Errorf("onApply got %+v, want mode=absolute from=%v to=%v", got, wantFrom, wantTo)
	}
	if tm.visible {
		t.Error("TimeRangeModal.applyAbsolute() did not close the modal")
	}
}

// TestApplyTimeRangeAbsoluteWithTime covers spec/53's revision: the
// Absolute tab must support setting a time, not only a date.
func TestApplyTimeRangeAbsoluteWithTime(t *testing.T) {
	tm, _ := newTestTimeRangeModal(t)
	var got ui.TimeRange
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeAbsolute}, func(tr ui.TimeRange) {
		got = tr
	})
	tm.absoluteForm.GetFormItem(0).(*tview.InputField).SetText("2026-08-01 09:30")
	tm.absoluteForm.GetFormItem(1).(*tview.InputField).SetText("2026-08-02 17:45")

	tm.applyAbsolute()

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
	tm, host := newTestTimeRangeModal(t)
	applied := false
	tm.Show(ui.TimeRange{Mode: ui.TimeRangeAbsolute}, func(ui.TimeRange) {
		applied = true
	})
	tm.absoluteForm.GetFormItem(0).(*tview.InputField).SetText("not-a-date")
	tm.absoluteForm.GetFormItem(1).(*tview.InputField).SetText("")

	tm.applyAbsolute()

	if applied {
		t.Error("TimeRangeModal.applyAbsolute() called onApply despite an invalid date")
	}
	if !tm.visible {
		t.Error("TimeRangeModal.applyAbsolute() closed the modal despite a parse error")
	}
	if !strings.Contains(host.status, "invalid from") {
		t.Errorf("status = %q, want it to mention the invalid \"from\" field", host.status)
	}
}
