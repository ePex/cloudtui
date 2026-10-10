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

var nyanColors = [...]string{"#FF5B5B", "#FFB52E", "#75D34A", "#40CFD0", "#5488F0", "#BE62DE"}

var nyanPixelColors = map[rune]string{
	'#': "#493B4A", // pastry outline and paws
	'g': "#888888", // shaded fur
	'G': "#D6D6D6", // light fur
	'p': "#F39CCB", // pastry frosting
	'r': "#F65D91", // sprinkles
	'c': "#56D8DE",
	'y': "#FFE36E",
}

var nyanSpriteFrames = [][]string{
	{
		"........g.g.",
		".......gggg.",
		".######GGggg",
		"#ppppppGg#gG",
		"#prcypcGgggG",
		".######GGggg",
	},
	{
		"........g.g.",
		".......gggg.",
		".######GGggg",
		"#ppppppGg#gG",
		"#pcyprcGgggG",
		".######GGggg",
	},
}

const (
	nyanTrackWidth     = 34
	nyanCatWidth       = 12
	nyanAnimationSteps = 12
	nyanSpriteHeight   = 6
	nyanProgressHeight = 8
)

// OperationProgress is a blocking visual for a slow queue operation. Call
// Tick periodically while it is visible to animate the rainbow trail.
type OperationProgress struct {
	host       ui.Host
	root       *tview.Flex
	label      *tview.TextView
	cat        *tview.TextView
	progress   *tview.TextView
	background string
	visible    bool
	completed  int
	total      int
	frame      int
}

// NewOperationProgress builds the reusable operation progress overlay.
func NewOperationProgress(host ui.Host) *OperationProgress {
	op := &OperationProgress{host: host, background: "black"}
	op.label = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.cat = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true).SetWrap(false)
	op.progress = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(op.label, 1, 0, false).
		AddItem(op.cat, nyanProgressHeight/2, 0, false).
		AddItem(op.progress, 1, 0, false).
		AddItem(nil, 0, 1, false)
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
	op.background = p.Background
	op.root.SetBackgroundColor(bg).SetBorderColor(tcell.GetColor(p.Border)).SetTitleColor(tcell.GetColor(p.Border))
	for _, text := range []*tview.TextView{op.label, op.cat, op.progress} {
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
	if op.total > 0 && op.completed > op.total {
		op.completed = op.total
	}
	op.cat.SetText(op.renderNyanProgress())

	if op.total <= 0 {
		op.progress.SetText("Working…")
		return
	}
	percent := op.completed * 100 / op.total
	op.progress.SetText(fmt.Sprintf("%d%%  •  Message %d of %d", percent, op.completed, op.total))
}

func (op *OperationProgress) nyanPosition() int {
	maxPosition := nyanTrackWidth - nyanCatWidth
	if op.total > 0 {
		completed := op.completed
		if completed > op.total {
			completed = op.total
		}
		return maxPosition * completed / op.total
	}
	step := op.frame
	if step >= nyanAnimationSteps/2 {
		step = nyanAnimationSteps - 1 - step
	}
	return maxPosition * step / (nyanAnimationSteps/2 - 1)
}

// renderNyanProgress composites a two-frame pixel sprite on its rainbow
// track and draws it in terminal half-block cells.
func (op *OperationProgress) renderNyanProgress() string {
	frame := nyanSpriteFrames[op.frame%len(nyanSpriteFrames)]
	position := op.nyanPosition()
	canvas := make([][]string, nyanProgressHeight)
	for row := range canvas {
		canvas[row] = make([]string, nyanTrackWidth)
	}
	for row := 0; row < nyanSpriteHeight; row++ {
		for col := 0; col < nyanCatWidth; col++ {
			pixel := nyanPixelColors[rune(frame[row][col])]
			if pixel != "" {
				canvas[row][position+col] = pixel
			}
		}
	}
	trailWidth := nyanTrackWidth
	if op.total > 0 {
		trailWidth = nyanTrackWidth * op.completed / op.total
	}
	for col := 0; col < trailWidth; col++ {
		color := nyanColors[(col+op.frame)%len(nyanColors)]
		canvas[nyanProgressHeight-2][col] = color
		canvas[nyanProgressHeight-1][col] = color
	}

	var sprite strings.Builder
	for row := 0; row < nyanProgressHeight; row += 2 {
		for col := 0; col < nyanTrackWidth; col++ {
			top := canvas[row][col]
			bottom := canvas[row+1][col]
			switch {
			case top == "" && bottom == "":
				sprite.WriteByte(' ')
			case top == "":
				fmt.Fprintf(&sprite, "[%s:%s]▄[-]", bottom, op.background)
			case bottom == "":
				fmt.Fprintf(&sprite, "[%s:%s]▀[-]", top, op.background)
			case top == bottom:
				fmt.Fprintf(&sprite, "[%s]█[-]", top)
			default:
				fmt.Fprintf(&sprite, "[%s:%s]▀[-]", top, bottom)
			}
		}
		if row+2 < nyanProgressHeight {
			sprite.WriteByte('\n')
		}
	}
	return sprite.String()
}

var _ ui.Themeable = (*OperationProgress)(nil)
