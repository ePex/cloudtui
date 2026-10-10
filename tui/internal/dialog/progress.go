package dialog

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// ProgressModalWidth and ProgressModalHeight are the progress modal's
// fixed outer size, border included.
const (
	ProgressModalWidth  = 60
	ProgressModalHeight = 11
)

// rainbowMinColors is the fewest colors a screen must report for the
// rainbow bar; below that the modal falls back to the spinner.
const rainbowMinColors = 256

// barRows is the bar's height in text rows. Each row holds two pixel
// rows (the upper and lower half of a cell, via '▀'), so the bar is six
// pixels tall: one per rainbow band.
const barRows = 3

// Fixed colors of the rainbow bar. They don't follow the theme: the bar
// is meant to look the same everywhere.
var (
	rainbowBands = [2 * barRows]tcell.Color{
		tcell.NewRGBColor(0xff, 0x00, 0x00), // red
		tcell.NewRGBColor(0xff, 0x99, 0x00), // orange
		tcell.NewRGBColor(0xff, 0xff, 0x00), // yellow
		tcell.NewRGBColor(0x33, 0xff, 0x00), // green
		tcell.NewRGBColor(0x00, 0x99, 0xff), // blue
		tcell.NewRGBColor(0x66, 0x33, 0xff), // purple
	}
	trackColor = tcell.NewRGBColor(0x0b, 0x1f, 0x4b) // night sky behind the trail
	starColor  = tcell.NewRGBColor(0xff, 0xff, 0xff)
	catColors  = map[byte]tcell.Color{
		't': tcell.NewRGBColor(0xf5, 0xc3, 0x8c), // tart crust
		'p': tcell.NewRGBColor(0xff, 0x99, 0xff), // pink frosting
		'm': tcell.NewRGBColor(0xff, 0x33, 0x99), // sprinkles
		'g': tcell.NewRGBColor(0x99, 0x99, 0x99), // fur
		'k': tcell.NewRGBColor(0x00, 0x00, 0x00), // eye
	}
)

// catFrames are the cat's two animation frames, six pixel rows each
// ('.' is transparent): a tail on the left, the tart body, the head with
// ears on the right, legs below. The tail and legs alternate.
var catFrames = [2][2 * barRows]string{
	{
		"..tttttg.g",
		"..tpmptggg",
		"ggtppptgkg",
		"..tmpptggg",
		"..tttttgg.",
		"...g.g.g.g",
	},
	{
		"..tttttg.g",
		"..tpmptggg",
		"..tppptgkg",
		"ggtmpptggg",
		"..tttttgg.",
		"..g.g.g.g.",
	},
}

// catWidth is the cat's width in cells (one pixel column per cell).
var catWidth = len(catFrames[0][0])

// spinnerFrames is the spinner's animation, one rune per frame.
var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// progressCell is one drawn cell of the bar.
type progressCell struct {
	r      rune
	fg, bg tcell.Color
}

// trailWidth is how many of width columns the rainbow trail covers for
// done of total.
func trailWidth(width, done, total int) int {
	if total <= 0 || done <= 0 || width <= 0 {
		return 0
	}
	if done >= total {
		return width
	}
	return width * done / total
}

// catLeft is the cat's leftmost column: its head sits at the trail's
// end, or at the bar's start while the trail is shorter than the cat.
func catLeft(trail int) int {
	return max(trail-catWidth, 0)
}

// rainbowCells returns the bar as barRows rows of width cells: the
// rainbow trail over the first trailWidth columns, the cat at its head,
// and twinkling stars on the rest of the track. frame selects the cat's
// pose and the stars' positions.
func rainbowCells(width, done, total, frame int) [][]progressCell {
	if width <= 0 {
		return nil
	}
	trail := trailWidth(width, done, total)

	// Pixels first: 2*barRows rows of width columns.
	pixels := make([][]tcell.Color, 2*barRows)
	for y := range pixels {
		pixels[y] = make([]tcell.Color, width)
		for x := range pixels[y] {
			if x < trail {
				pixels[y][x] = rainbowBands[y]
			} else {
				pixels[y][x] = trackColor
			}
		}
	}
	cat := catFrames[frame%len(catFrames)]
	left := -1
	if width >= catWidth {
		left = catLeft(trail)
		for y, row := range cat {
			for i := 0; i < len(row); i++ {
				if c, ok := catColors[row[i]]; ok {
					pixels[y][left+i] = c
				}
			}
		}
	}

	rows := make([][]progressCell, barRows)
	for y := range rows {
		rows[y] = make([]progressCell, width)
		for x := range rows[y] {
			rows[y][x] = progressCell{r: '▀', fg: pixels[2*y][x], bg: pixels[2*y+1][x]}
		}
	}

	// Stars on the open track, in the middle row, never under the cat.
	for x := trail; x < width; x++ {
		if left >= 0 && x >= left && x < left+catWidth {
			continue
		}
		switch (x*7 + frame) % 13 {
		case 0:
			rows[1][x] = progressCell{r: '✦', fg: starColor, bg: trackColor}
		case 6:
			rows[1][x] = progressCell{r: '·', fg: starColor, bg: trackColor}
		}
	}
	return rows
}

// spinnerRune is the spinner's rune for frame.
func spinnerRune(frame int) rune {
	return spinnerFrames[frame%len(spinnerFrames)]
}

// progressCount is the "3 of 10" line, plus the failures when there are
// any. Empty for an uncounted operation.
func progressCount(done, total, failed int) string {
	if total <= 0 {
		return ""
	}
	s := fmt.Sprintf("%d of %d", done, total)
	if failed > 0 {
		s += fmt.Sprintf(" (%d failed)", failed)
	}
	return s
}

// ProgressModal is the blocking progress overlay (spec-wip
// fe-progress-modal): the operation's title, then a rainbow bar with a
// cat for a counted operation on a screen with enough colors, otherwise
// a spinner; a count line, an optional detail text and a footer. It only
// draws: showing it, animating it (Tick) and handling keys belong to the
// app. All methods must run on the UI goroutine.
type ProgressModal struct {
	*tview.Box

	title        string
	done, total  int
	failed       int
	detail       string
	footer       string
	frame        int
	forceSpinner bool

	textColor, footerColor, accentColor tcell.Color
}

// NewProgressModal builds an empty progress modal.
func NewProgressModal() *ProgressModal {
	m := &ProgressModal{Box: tview.NewBox()}
	m.Box.SetBorder(true).SetTitle(" Working ")
	m.ApplyPalette(config.Palette{})
	return m
}

// Reset starts a new operation: title, total (0 for an uncounted one),
// and no progress, detail or footer yet.
func (m *ProgressModal) Reset(title string, total int) {
	m.title = title
	m.total = total
	m.done, m.failed = 0, 0
	m.detail, m.footer = "", ""
	m.frame = 0
}

// SetProgress records how many items are done, failures included, and
// how many of those failed.
func (m *ProgressModal) SetProgress(done, failed int) {
	m.done, m.failed = done, failed
}

// SetDetail sets the extra text shown under the count (up to two rows).
func (m *ProgressModal) SetDetail(text string) { m.detail = text }

// SetFooter sets the last line, e.g. "Esc to cancel".
func (m *ProgressModal) SetFooter(text string) { m.footer = text }

// SetForceSpinner makes counted operations use the spinner too, as on a
// screen without enough colors — for previewing the fallback.
func (m *ProgressModal) SetForceSpinner(on bool) { m.forceSpinner = on }

// Tick advances the animation by one frame.
func (m *ProgressModal) Tick() { m.frame++ }

// rainbow reports whether the modal draws the rainbow bar on screen.
func (m *ProgressModal) rainbow(screen tcell.Screen) bool {
	return m.total > 0 && !m.forceSpinner && screen.Colors() >= rainbowMinColors
}

// Draw paints the modal. Its layout, inside the border:
//
//	row 0     title
//	rows 2–4  rainbow bar, or the spinner (and count) on row 3
//	row 5     count, under the rainbow bar
//	rows 6–7  detail, wrapped
//	row 8     footer
func (m *ProgressModal) Draw(screen tcell.Screen) {
	m.Box.DrawForSubclass(screen, m)
	x, y, width, height := m.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	line := func(text string, row int, align int, color tcell.Color) {
		if row < height {
			tview.Print(screen, tview.Escape(text), x+1, y+row, width-2, align, color)
		}
	}

	line(m.title, 0, tview.AlignCenter, m.textColor)

	if m.rainbow(screen) {
		bar := rainbowCells(width-2, m.done, m.total, m.frame)
		for r, cells := range bar {
			if 2+r >= height {
				break
			}
			for c, cell := range cells {
				screen.SetContent(x+1+c, y+2+r, cell.r, nil,
					tcell.StyleDefault.Foreground(cell.fg).Background(cell.bg))
			}
		}
		line(progressCount(m.done, m.total, m.failed), 5, tview.AlignCenter, m.textColor)
	} else {
		spin := string(spinnerRune(m.frame))
		if count := progressCount(m.done, m.total, m.failed); count != "" {
			spin += "  " + count
		}
		line(spin, 3, tview.AlignCenter, m.accentColor)
	}

	for i, row := range wrapLines(m.detail, width-2, 2) {
		line(row, 6+i, tview.AlignCenter, m.textColor)
	}
	line(m.footer, 8, tview.AlignCenter, m.footerColor)
}

// wrapLines word-wraps text into at most maxRows rows of width
// characters; the last row ends in "…" when text doesn't fit.
func wrapLines(text string, width, maxRows int) []string {
	if text == "" || width <= 0 {
		return nil
	}
	var rows []string
	line := ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len([]rune(line))+1+len([]rune(word)) <= width:
			line += " " + word
		default:
			rows = append(rows, line)
			line = word
		}
	}
	rows = append(rows, line)
	for i, r := range rows {
		if len([]rune(r)) > width {
			rows[i] = string([]rune(r)[:width-1]) + "…"
		}
	}
	if len(rows) > maxRows {
		last := []rune(rows[maxRows-1])
		if len(last) >= width {
			last = last[:width-1]
		}
		rows = append(rows[:maxRows-1], string(last)+"…")
	}
	return rows
}

// ApplyPalette recolors the modal for a live theme switch. The rainbow
// and the cat keep their fixed colors.
func (m *ProgressModal) ApplyPalette(p config.Palette) {
	m.Box.SetBackgroundColor(tcell.GetColor(p.Background))
	m.Box.SetBorderColor(tcell.GetColor(p.Border))
	m.Box.SetTitleColor(tcell.GetColor(p.Border))
	m.textColor = tcell.GetColor(p.Text)
	m.footerColor = tcell.GetColor(p.Label)
	m.accentColor = tcell.GetColor(p.Accent)
}

var _ ui.Themeable = (*ProgressModal)(nil)

// Primitive returns the modal itself, for sizing/embedding.
func (m *ProgressModal) Primitive() tview.Primitive { return m }
