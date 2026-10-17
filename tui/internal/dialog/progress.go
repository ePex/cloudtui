package dialog

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// ProgressModalWidth is the progress modal's outer width, border
// included. ProgressModalHeight is its outer height for a one-line title
// and up to two detail rows; a longer title or detail makes it taller
// (Height).
const (
	ProgressModalWidth  = 60
	ProgressModalHeight = 11
)

// Limits of the modal's wrapped text: the title gets up to
// progressTitleMaxRows lines, the detail at least progressDetailMinRows
// rows (kept even when empty, so the modal doesn't jump when a detail
// appears) and up to progressDetailMaxRows. Text beyond them ends in "…".
const (
	progressTitleMaxRows  = 4
	progressDetailMinRows = 2
	progressDetailMaxRows = 3
)

// progressTextWidth is the width text gets: inside the border, with one
// column of margin on each side.
const progressTextWidth = ProgressModalWidth - 4

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
	// Night sky behind the trail: exactly xterm color 17, so a
	// 256-color terminal shows the same navy (a nearby RGB value maps to
	// a gray there).
	trackColor = tcell.NewRGBColor(0x00, 0x00, 0x5f)
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

// catTrailEnd is the cat column where the rainbow ends behind it (the
// middle of the tart): transparent pixels from there on show the night
// sky, so the trail doesn't peek out between the ears or legs.
const catTrailEnd = 5

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
				} else if i >= catTrailEnd {
					pixels[y][left+i] = trackColor
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
		// 0 and 3 are six columns apart (7·6 ≡ 3 mod 13), so the two
		// kinds of star don't bunch up.
		switch (x*7 + frame) % 13 {
		case 0:
			rows[1][x] = progressCell{r: '✦', fg: starColor, bg: trackColor}
		case 3:
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

// ProgressModal is the blocking progress overlay (spec/24): the
// operation's title, then a rainbow bar with a
// cat for a counted operation on a screen with enough colors, otherwise
// a spinner; a count line, an optional detail text and a footer. It only
// draws: showing it, animating it (SetFrame) and handling keys belong to
// the app. All methods must run on the UI goroutine.
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

// SetDetail sets the extra text shown under the count (up to three rows).
func (m *ProgressModal) SetDetail(text string) { m.detail = text }

// SetFooter sets the last line, e.g. "Esc to cancel".
func (m *ProgressModal) SetFooter(text string) { m.footer = text }

// SetForceSpinner makes counted operations use the spinner too, as on a
// screen without enough colors — for previewing the fallback.
func (m *ProgressModal) SetForceSpinner(on bool) { m.forceSpinner = on }

// SetFrame sets the animation frame: the cat's pose, the stars and the
// spinner.
func (m *ProgressModal) SetFrame(frame int) { m.frame = frame }

// rainbow reports whether the modal draws the rainbow bar on screen.
func (m *ProgressModal) rainbow(screen tcell.Screen) bool {
	return m.total > 0 && !m.forceSpinner && screen.Colors() >= rainbowMinColors
}

// progressLayout is the modal's wrapped text and where it goes.
type progressLayout struct {
	rainbow bool
	// title is the title's lines; in the spinner look, the spinner's
	// line: the spinner rune, a space, the title, "…" and the count.
	title  []string
	detail []string
}

// titleRows is how many rows the title takes (at least one).
func (l progressLayout) titleRows() int { return max(1, len(l.title)) }

// detailRows is how many rows are kept for the detail.
func (l progressLayout) detailRows() int { return max(progressDetailMinRows, len(l.detail)) }

// innerHeight is the rows the layout needs inside the border: the title,
// a blank row, the bar (or the spinner's area), the count, the detail
// and the footer.
func (l progressLayout) innerHeight() int { return l.titleRows() + 6 + l.detailRows() }

// layout wraps the modal's text for the rainbow or the spinner look.
func (m *ProgressModal) layout(rainbow bool) progressLayout {
	l := progressLayout{rainbow: rainbow}
	if rainbow {
		l.title = wrapLines(m.title, progressTextWidth, progressTitleMaxRows)
	} else {
		text := string(spinnerRune(m.frame)) + " " + m.title + "…"
		if count := progressCount(m.done, m.total, m.failed); count != "" {
			text += "  " + count
		}
		l.title = wrapLines(text, progressTextWidth, progressTitleMaxRows)
	}
	l.detail = wrapLines(m.detail, progressTextWidth, progressDetailMaxRows)
	return l
}

// Height is the modal's outer height, border included, for its current
// text when drawn on screen: ProgressModalHeight for a one-line title,
// taller for a longer title or a three-row detail.
func (m *ProgressModal) Height(screen tcell.Screen) int {
	return m.layout(m.rainbow(screen)).innerHeight() + 2
}

// Draw paints the modal. Its layout inside the border, with t title
// lines (one for a short title) and d detail rows (at least two):
//
//	rows 0…t-1     title, above the rainbow bar
//	rows t+1…t+3   rainbow bar; or from row 3, the spinner line(s)
//	row t+4        count, under the rainbow bar
//	rows t+5…      detail, wrapped
//	row t+5+d      footer
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
	l := m.layout(m.rainbow(screen))
	t := l.titleRows()

	if l.rainbow {
		for i, text := range l.title {
			line(text, i, tview.AlignCenter, m.textColor)
		}
		bar := rainbowCells(width-2, m.done, m.total, m.frame)
		for r, cells := range bar {
			if t+1+r >= height {
				break
			}
			for c, cell := range cells {
				screen.SetContent(x+1+c, y+t+1+r, cell.r, nil,
					tcell.StyleDefault.Foreground(cell.fg).Background(cell.bg))
			}
		}
		line(progressCount(m.done, m.total, m.failed), t+4, tview.AlignCenter, m.textColor)
	} else {
		// "⠹ Loading queues…  3 of 10": the spinner (first rune of the
		// first line) in the accent color, the rest in the text color.
		for i, text := range l.title {
			row := 3 + i
			if i > 0 {
				line(text, row, tview.AlignCenter, m.textColor)
				continue
			}
			runes := []rune(text)
			spin := string(runes[0])
			if hex := m.accentColor.Hex(); hex >= 0 {
				spin = fmt.Sprintf("[#%06x]%s[-]", hex, spin)
			}
			if row < height {
				tview.Print(screen, spin+tview.Escape(string(runes[1:])), x+1, y+row, width-2, tview.AlignCenter, m.textColor)
			}
		}
	}

	for i, text := range l.detail {
		line(text, t+5+i, tview.AlignCenter, m.textColor)
	}
	line(m.footer, t+5+l.detailRows(), tview.AlignCenter, m.footerColor)
}

// wrapLines word-wraps text into at most maxRows rows of width
// characters; the last row ends in "…" when text doesn't fit. A word
// longer than a row (a queue name, a URL) starts a new row and is broken
// into width-long pieces rather than cut.
func wrapLines(text string, width, maxRows int) []string {
	if text == "" || width <= 0 {
		return nil
	}
	var rows []string
	line := ""
	for _, word := range strings.Fields(text) {
		runes := []rune(word)
		for len(runes) > width {
			if line != "" {
				rows = append(rows, line)
				line = ""
			}
			rows = append(rows, string(runes[:width]))
			runes = runes[width:]
		}
		word = string(runes)
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
