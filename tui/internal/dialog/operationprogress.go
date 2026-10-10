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
		"...........g.g..",
		"..........gggg..",
		"..########GGggg.",
		".#ppppppppGg#gG.",
		".#prcyprcpGgggG.",
		".#ppppppppGgggG.",
		"..########GGggg.",
		"....#..#....#..#",
	},
	{
		"...........g.g..",
		"..........gggg..",
		"..########GGggg.",
		".#ppppppppGg#gG.",
		".#pcyprycpGgggG.",
		".#ppppppppGgggG.",
		"..########GGggg.",
		"...#..#....#..#.",
	},
}

const (
	nyanTrackWidth     = 34
	nyanCatWidth       = 16
	nyanAnimationSteps = 12
	nyanSpriteHeight   = 8
)

// OperationProgress is a blocking visual for a slow queue operation. Call
// Tick periodically while it is visible to animate the rainbow trail.
type OperationProgress struct {
	host      ui.Host
	root      *tview.Flex
	label     *tview.TextView
	cat       *tview.TextView
	trail     *tview.TextView
	bar       *tview.TextView
	counter   *tview.TextView
	background string
	visible   bool
	completed int
	total     int
	frame     int
}

// NewOperationProgress builds the reusable operation progress overlay.
func NewOperationProgress(host ui.Host) *OperationProgress {
	op := &OperationProgress{host: host, background: "black"}
	op.label = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.cat = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true).SetWrap(false)
	op.trail = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.bar = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.counter = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true)
	op.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(op.label, 1, 0, false).
		AddItem(op.cat, nyanSpriteHeight/2, 0, false).
		AddItem(op.trail, 1, 0, false).
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
	op.background = p.Background
	op.root.SetBackgroundColor(bg).SetBorderColor(tcell.GetColor(p.Border)).SetTitleColor(tcell.GetColor(p.Border))
	for _, text := range []*tview.TextView{op.label, op.cat, op.trail, op.bar, op.counter} {
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
	op.cat.SetText(op.renderNyanSprite())
	op.trail.SetText(op.renderNyanTrack())

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
	position := op.nyanPosition()
	trailWidth := position
	var track strings.Builder
	for i := 0; i < trailWidth; i++ {
		color := nyanColors[(i+op.frame)%len(nyanColors)]
		fmt.Fprintf(&track, "[%s]━[-]", color)
	}
	remaining := nyanTrackWidth - position
	if remaining > 0 {
		track.WriteString("[gray]")
		track.WriteString(strings.Repeat("─", remaining))
		track.WriteString("[-]")
	}
	return track.String()
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

// renderNyanSprite turns a tiny two-frame pixel sprite into half-block
// terminal cells. This gives the modal a GIF-like cat animation without
// requiring a terminal-specific image protocol.
func (op *OperationProgress) renderNyanSprite() string {
	frame := nyanSpriteFrames[op.frame%len(nyanSpriteFrames)]
	position := op.nyanPosition()
	var sprite strings.Builder
	for row := 0; row < nyanSpriteHeight; row += 2 {
		sprite.WriteString(strings.Repeat(" ", position))
		for col := 0; col < nyanCatWidth; col++ {
			top := nyanPixelColors[rune(frame[row][col])]
			bottom := nyanPixelColors[rune(frame[row+1][col])]
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
		sprite.WriteString(strings.Repeat(" ", nyanTrackWidth-position-nyanCatWidth))
		if row+2 < nyanSpriteHeight {
			sprite.WriteByte('\n')
		}
	}
	return sprite.String()
}

var _ ui.Themeable = (*OperationProgress)(nil)
