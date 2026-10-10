package dialog

import (
	"strings"
	"testing"
)

func TestOperationProgressShowAndHide(t *testing.T) {
	host := newTestHost()
	op := NewOperationProgress(host)
	op.Show("Deleting from orders", 4)

	if !op.Visible() {
		t.Fatal("Show() left the overlay hidden")
	}
	if got := host.shownPages[len(host.shownPages)-1]; got != operationProgressPage {
		t.Errorf("shown page = %q, want %q", got, operationProgressPage)
	}
	if host.focused != op.Primitive() {
		t.Error("Show() did not focus the progress overlay")
	}
	if got := op.label.GetText(true); got != "Deleting from orders" {
		t.Errorf("label = %q", got)
	}
	op.Hide()
	if op.Visible() {
		t.Error("Hide() left the overlay visible")
	}
	if got := host.hiddenPages[len(host.hiddenPages)-1]; got != operationProgressPage {
		t.Errorf("hidden page = %q, want %q", got, operationProgressPage)
	}
	if host.focusMainCalls != 1 {
		t.Errorf("FocusMain() calls = %d, want 1", host.focusMainCalls)
	}
}

func TestOperationProgressDeterminateRendering(t *testing.T) {
	op := NewOperationProgress(newTestHost())
	op.Show("Moving messages", 4)
	catAtStart := op.cat.GetText(true)
	op.SetProgress(2, 4)
	if got := op.cat.GetText(true); got == catAtStart {
		t.Error("cat did not move along the track with determinate progress")
	}

	text := renderedScreenText(t, op.Primitive(), 52, 10)
	if !strings.Contains(text, "Moving messages") {
		t.Errorf("overlay is missing operation label: %q", text)
	}
	if !strings.Contains(text, "50%") || !strings.Contains(text, "Message 2 of 4") {
		t.Errorf("overlay is missing determinate progress: %q", text)
	}
	if !strings.Contains(text, "=^.^=") {
		t.Errorf("overlay is missing Nyan Cat: %q", text)
	}
}

func TestOperationProgressIndeterminateAnimation(t *testing.T) {
	op := NewOperationProgress(newTestHost())
	op.Show("Loading queues", 0)
	before := op.bar.GetText(true)
	catBefore := op.cat.GetText(true)
	op.Tick()
	after := op.bar.GetText(true)
	if before == after {
		t.Fatalf("Tick() did not move the indeterminate indicator: %q", before)
	}
	if strings.Contains(after, "%") {
		t.Errorf("indeterminate indicator unexpectedly shows a percentage: %q", after)
	}
	if got := op.cat.GetText(true); got == catBefore {
		t.Error("cat did not move along the track in indeterminate mode")
	}
	op.Hide()
	before = op.cat.GetText(true)
	op.Tick()
	if got := op.cat.GetText(true); got != before {
		t.Errorf("Tick() changed a hidden overlay: %q -> %q", before, got)
	}
}

func TestOperationProgressClampsCompletedCount(t *testing.T) {
	op := NewOperationProgress(newTestHost())
	op.Show("Deleting", 3)
	op.SetProgress(8, 3)
	if got := op.bar.GetText(true); !strings.Contains(got, "100%") {
		t.Errorf("clamped bar = %q, want 100%%", got)
	}
	if got := op.counter.GetText(true); got != "Message 3 of 3" {
		t.Errorf("clamped counter = %q, want Message 3 of 3", got)
	}
}
