package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
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

func TestOperationProgressPixelSpriteUsesMultipleColors(t *testing.T) {
	op := NewOperationProgress(newTestHost())
	op.ApplyPalette(config.Default().Colors)
	op.Show("Moving messages", 10)
	op.SetProgress(5, 10)

	const width, height = 54, 14
	op.Primitive().SetRect(0, 0, width, height)
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)
	op.Primitive().Draw(screen)
	screen.Show()

	colors := map[tcell.Color]bool{}
	cells, w, _ := screen.GetContents()
	for _, cell := range cells {
		if len(cell.Runes) == 0 {
			continue
		}
		switch cell.Runes[0] {
		case '▀', '▄', '█':
			fg, _, _ := cell.Style.Decompose()
			colors[fg] = true
		}
	}
	if len(colors) < 4 {
		t.Errorf("pixel cat rendered with %d foreground colors, want multiple sprite colors (screen width %d)", len(colors), w)
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

	text := renderedScreenText(t, op.Primitive(), 52, 14)
	if !strings.Contains(text, "Moving messages") {
		t.Errorf("overlay is missing operation label: %q", text)
	}
	if !strings.Contains(text, "50%") || !strings.Contains(text, "Message 2 of 4") {
		t.Errorf("overlay is missing determinate progress: %q", text)
	}
	if !strings.Contains(text, "▀") && !strings.Contains(text, "▄") {
		t.Errorf("overlay is missing the pixel-art Nyan Cat: %q", text)
	}
}

func TestOperationProgressIndeterminateAnimation(t *testing.T) {
	op := NewOperationProgress(newTestHost())
	op.Show("Loading queues", 0)
	before := op.cat.GetText(true)
	op.Tick()
	after := op.cat.GetText(true)
	if before == after {
		t.Fatalf("Tick() did not animate the indeterminate progress: %q", before)
	}
	if got := op.progress.GetText(true); got != "Working…" {
		t.Errorf("indeterminate status = %q, want Working…", got)
	}
	op.Hide()
	before = op.cat.GetText(true)
	op.Tick()
	if got := op.cat.GetText(true); got != before {
		t.Errorf("Tick() changed a hidden overlay: %q -> %q", before, got)
	}
}

func TestNyanSpriteFramesHaveExpectedDimensions(t *testing.T) {
	for frameIndex, frame := range nyanSpriteFrames {
		if len(frame) != nyanSpriteHeight {
			t.Errorf("frame %d height = %d, want %d", frameIndex, len(frame), nyanSpriteHeight)
		}
		for rowIndex, row := range frame {
			if len(row) != nyanCatWidth {
				t.Errorf("frame %d row %d width = %d, want %d", frameIndex, rowIndex, len(row), nyanCatWidth)
			}
		}
	}
}

func TestOperationProgressClampsCompletedCount(t *testing.T) {
	op := NewOperationProgress(newTestHost())
	op.Show("Deleting", 3)
	op.SetProgress(8, 3)
	if got := op.progress.GetText(true); !strings.Contains(got, "100%") || !strings.Contains(got, "Message 3 of 3") {
		t.Errorf("clamped progress = %q, want 100%% and Message 3 of 3", got)
	}
}
