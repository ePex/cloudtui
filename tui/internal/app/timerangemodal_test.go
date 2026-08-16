package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestShowTimeRangeModalRelativePrefill(t *testing.T) {
	a := New(config.Default())
	current := timeRange{mode: timeRangeRelative, presetIdx: 3}

	a.showTimeRangeModal(current, func(timeRange) {})

	if !a.timeRangeVisible {
		t.Fatal("showTimeRangeModal() did not open the overlay")
	}
	if a.timeRangeActiveTab != timeRangeRelative {
		t.Errorf("timeRangeActiveTab = %v, want timeRangeRelative", a.timeRangeActiveTab)
	}
	if got := a.timeRangeRelativeList.GetCurrentItem(); got != 3 {
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
	current := timeRange{mode: timeRangeAbsolute, from: from, to: to}

	a.showTimeRangeModal(current, func(timeRange) {})

	if a.timeRangeActiveTab != timeRangeAbsolute {
		t.Errorf("timeRangeActiveTab = %v, want timeRangeAbsolute", a.timeRangeActiveTab)
	}
	fromText := a.timeRangeAbsoluteForm.GetFormItem(0).(*tview.InputField).GetText()
	untilText := a.timeRangeAbsoluteForm.GetFormItem(1).(*tview.InputField).GetText()
	if fromText != "2026-08-01 09:30" {
		t.Errorf("From field = %q, want %q", fromText, "2026-08-01 09:30")
	}
	if untilText != "2026-08-02 17:45" {
		t.Errorf("Until field = %q, want %q", untilText, "2026-08-02 17:45")
	}
}

func TestCloseTimeRangeModal(t *testing.T) {
	a := New(config.Default())
	a.showTimeRangeModal(timeRange{mode: timeRangeRelative}, func(timeRange) {})

	a.closeTimeRangeModal()

	if a.timeRangeVisible {
		t.Error("closeTimeRangeModal() left timeRangeVisible true")
	}
}

func TestTimeRangeModalEscapeCloses(t *testing.T) {
	a := New(config.Default())
	a.showTimeRangeModal(timeRange{mode: timeRangeRelative}, func(timeRange) {})

	capture := a.timeRangeFlex.GetInputCapture()
	if capture == nil {
		t.Fatal("timeRangeFlex has no input capture set")
	}
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if a.timeRangeVisible {
		t.Error("Esc did not close the time range modal")
	}
}

func TestTimeRangeModalEscapeDoesNotApply(t *testing.T) {
	a := New(config.Default())
	applied := false
	a.showTimeRangeModal(timeRange{mode: timeRangeRelative, presetIdx: 1}, func(timeRange) {
		applied = true
	})

	a.timeRangeFlex.GetInputCapture()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if applied {
		t.Error("Esc must not call onApply")
	}
}

func TestTimeRangeModalTabSwitching(t *testing.T) {
	a := New(config.Default())
	a.showTimeRangeModal(timeRange{mode: timeRangeRelative}, func(timeRange) {})
	capture := a.timeRangeFlex.GetInputCapture()

	if got := capture(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone)); got != nil {
		t.Errorf("'A' capture returned %v, want nil (event consumed)", got)
	}
	if a.timeRangeActiveTab != timeRangeAbsolute {
		t.Errorf("timeRangeActiveTab = %v, want timeRangeAbsolute after 'A'", a.timeRangeActiveTab)
	}

	if got := capture(tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone)); got != nil {
		t.Errorf("'R' capture returned %v, want nil (event consumed)", got)
	}
	if a.timeRangeActiveTab != timeRangeRelative {
		t.Errorf("timeRangeActiveTab = %v, want timeRangeRelative after 'R'", a.timeRangeActiveTab)
	}
}

// TestSwitchTimeRangeTabResetsAbsoluteFormFocus guards a bug caught live
// (verify-live, spec/53): tview.Form remembers its last-focused item
// across Application.SetFocus calls, so reopening the modal after a
// previous Absolute-tab session that ended on Apply/Cancel would leave
// keystrokes going to the button instead of the From field, unless
// switchTimeRangeTab explicitly resets the form's internal focus to 0.
func TestSwitchTimeRangeTabResetsAbsoluteFormFocus(t *testing.T) {
	a := New(config.Default())
	a.showTimeRangeModal(timeRange{mode: timeRangeAbsolute}, func(timeRange) {})
	a.timeRangeAbsoluteForm.SetFocus(2) // simulate a prior session ending on the Apply button

	a.switchTimeRangeTab(timeRangeAbsolute)

	item, button := a.timeRangeAbsoluteForm.GetFocusedItemIndex()
	if item != 0 || button != -1 {
		t.Errorf("form focus = (item=%d, button=%d), want (item=0, button=-1) — the From field", item, button)
	}
}

func TestTimeRangeModalOtherKeysPassThrough(t *testing.T) {
	a := New(config.Default())
	a.showTimeRangeModal(timeRange{mode: timeRangeRelative}, func(timeRange) {})

	capture := a.timeRangeFlex.GetInputCapture()
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	if got := capture(event); got != event {
		t.Errorf("capture() altered/swallowed a non-Esc/R/A key: got %v, want it passed through unchanged", got)
	}
}

func TestApplyTimeRangeRelative(t *testing.T) {
	a := New(config.Default())
	var got timeRange
	applied := false
	a.showTimeRangeModal(timeRange{mode: timeRangeRelative}, func(tr timeRange) {
		applied = true
		got = tr
	})

	a.applyTimeRangeRelative(2)

	if !applied {
		t.Fatal("applyTimeRangeRelative() did not call onApply")
	}
	if got.mode != timeRangeRelative || got.presetIdx != 2 {
		t.Errorf("onApply got %+v, want mode=relative presetIdx=2", got)
	}
	if a.timeRangeVisible {
		t.Error("applyTimeRangeRelative() did not close the modal")
	}
}

func TestApplyTimeRangeAbsoluteValid(t *testing.T) {
	a := New(config.Default())
	var got timeRange
	applied := false
	a.showTimeRangeModal(timeRange{mode: timeRangeAbsolute}, func(tr timeRange) {
		applied = true
		got = tr
	})
	a.timeRangeAbsoluteForm.GetFormItem(0).(*tview.InputField).SetText("2026-08-01")
	a.timeRangeAbsoluteForm.GetFormItem(1).(*tview.InputField).SetText("2026-08-02")

	a.applyTimeRangeAbsolute()

	if !applied {
		t.Fatal("applyTimeRangeAbsolute() did not call onApply")
	}
	wantFrom := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if got.mode != timeRangeAbsolute || !got.from.Equal(wantFrom) || !got.to.Equal(wantTo) {
		t.Errorf("onApply got %+v, want mode=absolute from=%v to=%v", got, wantFrom, wantTo)
	}
	if a.timeRangeVisible {
		t.Error("applyTimeRangeAbsolute() did not close the modal")
	}
}

// TestApplyTimeRangeAbsoluteWithTime covers spec/53's revision: the
// Absolute tab must support setting a time, not only a date.
func TestApplyTimeRangeAbsoluteWithTime(t *testing.T) {
	a := New(config.Default())
	var got timeRange
	a.showTimeRangeModal(timeRange{mode: timeRangeAbsolute}, func(tr timeRange) {
		got = tr
	})
	a.timeRangeAbsoluteForm.GetFormItem(0).(*tview.InputField).SetText("2026-08-01 09:30")
	a.timeRangeAbsoluteForm.GetFormItem(1).(*tview.InputField).SetText("2026-08-02 17:45")

	a.applyTimeRangeAbsolute()

	// Typed values are interpreted as local time, not UTC (see
	// parseFilterDate's doc comment) — built via time.Local, not a
	// hardcoded UTC offset, so this passes regardless of the machine's
	// timezone.
	wantFrom := time.Date(2026, 8, 1, 9, 30, 0, 0, time.Local)
	wantTo := time.Date(2026, 8, 2, 17, 45, 0, 0, time.Local)
	if !got.from.Equal(wantFrom) || !got.to.Equal(wantTo) {
		t.Errorf("onApply got from=%v to=%v, want from=%v to=%v", got.from, got.to, wantFrom, wantTo)
	}
}

func TestApplyTimeRangeAbsoluteInvalidDate(t *testing.T) {
	a := New(config.Default())
	applied := false
	a.showTimeRangeModal(timeRange{mode: timeRangeAbsolute}, func(timeRange) {
		applied = true
	})
	a.timeRangeAbsoluteForm.GetFormItem(0).(*tview.InputField).SetText("not-a-date")
	a.timeRangeAbsoluteForm.GetFormItem(1).(*tview.InputField).SetText("")

	a.applyTimeRangeAbsolute()

	if applied {
		t.Error("applyTimeRangeAbsolute() called onApply despite an invalid date")
	}
	if !a.timeRangeVisible {
		t.Error("applyTimeRangeAbsolute() closed the modal despite a parse error")
	}
	if got := a.statusBar.GetText(true); !strings.Contains(got, "invalid from") {
		t.Errorf("statusBar = %q, want it to mention the invalid \"from\" field", got)
	}
}
