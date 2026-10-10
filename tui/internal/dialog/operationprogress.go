package dialog

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

const operationProgressPage = "operation-progress"

var nyanColors = [...]string{"red", "yellow", "green", "cyan", "blue", "magenta"}

const (
	nyanTrackWidth     = 34
	nyanCatWidth       = 5
	nyanAnimationSteps = 12
)

// OperationProgress is a blocking visual for a slow queue operation. Call
// Tick periodically while it is visible to animate the rainbow trail.
type OperationProgress struct {
	host      ui.Host
	root      *tview.Flex
	label     *tview.TextView
	cat       *tview.TextView
	bar       *tview.TextView
	counter   *tview.TextView
	visible   bool
	completed int
	total     int
	frame     int
}

// NewOperationProgress builds the reusable operation progress overlay.
func NewOperationProgress(host ui.Host) *OperationProgress {
	op := &OperationProgress{host: host}
	op.label = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.cat = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.bar = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.counter = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(op.label, 1, 0, false).
		AddItem(op.cat, 1, 0, false).
		AddItem(op.bar, 1, 0, false).
		AddItem(op.counter, 1, 0, false).
		AddItem(nil, 1, 0, false)
	op.root.SetBorder(true).SetTitle(" Working ")
	op.render()
	return op
}

// Show opens the overlay. A total less than or equal to zero selects
// indeterminate mode for operations whose backend exposes no substeps.
func (op *OperationProgress) Show(label string, total int) {
	op.label.SetText(tview.Escape(label))
	op.completed = 0
	op.total = total
	op.frame = 0
	op.visible = true
	op.render()
	op.host.ShowPage(operationProgressPage)
	op.host.SetFocus(op.root)
}

// SetProgress updates determinate progress. Failed items count as completed
// attempts so the bar reflects work performed, not only successful results.
func (op *OperationProgress) SetProgress(completed, total int) {
	if completed < 0 {
		completed = 0
	}
	if total < 0 {
		total = 0
	}
	op.completed, op.total = completed, total
	op.render()
}

// Tick advances the animation. The app calls this on its UI event loop while
// the overlay is visible.
func (op *OperationProgress) Tick() {
	if !op.visible {
		return
	}
	op.frame = (op.frame + 1) % nyanAnimationSteps
	op.render()
}

// Hide closes the overlay and returns focus to the main application.
func (op *OperationProgress) Hide() {
	if !op.visible {
		return
	}
	op.visible = false
	op.host.HidePage(operationProgressPage)
	op.host.FocusMain()
}

// ApplyPalette recolors the overlay when the user switches themes.
func (op *OperationProgress) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	op.root.SetBackgroundColor(bg).SetBorderColor(tcell.GetColor(p.Border)).SetTitleColor(tcell.GetColor(p.Border))
	for _, text := range []*tview.TextView{op.label, op.cat, op.bar, op.counter} {
		text.SetBackgroundColor(bg)
		text.SetTextColor(tcell.GetColor(p.Text))
	}
	op.render()
}

// Primitive returns the widget to embed in the app's pages.
func (op *OperationProgress) Primitive() tview.Primitive { return op.root }

// Visible reports whether the operation overlay is open.
func (op *OperationProgress) Visible() bool { return op.visible }

func (op *OperationProgress) render() {
	op.cat.SetText(op.renderNyanTrack())

	if op.total <= 0 {
		op.counter.SetText("")
		width := 19
		position := op.frame * (width - 5) / (len(nyanColors) - 1)
		line := strings.Repeat("─", position) + "=^.^=" + strings.Repeat("─", width-position-5)
		op.bar.SetText("[gray]" + line + "[-]")
		return
	}
	if op.completed > op.total {
		op.completed = op.total
	}
	const width = 24
	filled := op.completed * width / op.total
	percent := op.completed * 100 / op.total
	bar := "[green]" + strings.Repeat("█", filled) + "[-]" + "[gray]" + strings.Repeat("░", width-filled) + "[-]"
	op.bar.SetText(fmt.Sprintf("%s  %d%%", bar, percent))
	op.counter.SetText(fmt.Sprintf("Message %d of %d", op.completed, op.total))
}

// renderNyanTrack places the cat at the current determinate progress point,
// or bounces it along the rainbow track for opaque operations.
func (op *OperationProgress) renderNyanTrack() string {
	maxPosition := nyanTrackWidth - nyanCatWidth
	position := 0
	if op.total > 0 {
		completed := op.completed
		if completed > op.total {
			completed = op.total
		}
		position = maxPosition * completed / op.total
	} else {
		step := op.frame
		if step >= nyanAnimationSteps/2 {
			step = nyanAnimationSteps - 1 - step
		}
		position = maxPosition * step / (nyanAnimationSteps/2 - 1)
	}
	trailWidth := position
	var track strings.Builder
	for i := 0; i < trailWidth; i++ {
		color := nyanColors[(i+op.frame)%len(nyanColors)]
		fmt.Fprintf(&track, "[%s]━[-]", color)
	}
	track.WriteString("[white]=^.^=[-]")
	remaining := nyanTrackWidth - position - nyanCatWidth
	if remaining > 0 {
		track.WriteString("[gray]")
		track.WriteString(strings.Repeat("─", remaining))
		track.WriteString("[-]")
	}
	return track.String()
}

var _ ui.Themeable = (*OperationProgress)(nil)
