package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// fewColorsScreen is a simulation screen that reports fewer colors, as a
// 16-color terminal does. tcell's simulation screen always says 256.
type fewColorsScreen struct {
	tcell.Screen
	colors int
}

func (s fewColorsScreen) Colors() int { return s.colors }

// drawnCell is one cell of a drawn screen.
type drawnCell struct {
	r      rune
	fg, bg tcell.Color
}

// drawModal draws m at its full size on a simulation screen reporting
// colors colors, and returns the cells row by row.
func drawModal(t *testing.T, m *ProgressModal, colors int) [][]drawnCell {
	t.Helper()
	sim := tcell.NewSimulationScreen("")
	if err := sim.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	defer sim.Fini()
	sim.SetSize(ProgressModalWidth, ProgressModalHeight)
	var screen tcell.Screen = sim
	if colors != 256 {
		screen = fewColorsScreen{Screen: sim, colors: colors}
	}
	m.SetRect(0, 0, ProgressModalWidth, ProgressModalHeight)
	m.Draw(screen)
	sim.Show()

	cells, w, h := sim.GetContents()
	rows := make([][]drawnCell, h)
	for y := range rows {
		rows[y] = make([]drawnCell, w)
		for x := range rows[y] {
			c := cells[y*w+x]
			fg, bg, _ := c.Style.Decompose()
			r := ' '
			if len(c.Runes) > 0 {
				r = c.Runes[0]
			}
			rows[y][x] = drawnCell{r: r, fg: fg, bg: bg}
		}
	}
	return rows
}

// rowText is the runes of one drawn row.
func rowText(row []drawnCell) string {
	var b strings.Builder
	for _, c := range row {
		b.WriteRune(c.r)
	}
	return b.String()
}

// screenText is every drawn row, newline-separated.
func screenText(rows [][]drawnCell) string {
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = rowText(r)
	}
	return strings.Join(lines, "\n")
}

// countBarCells counts the cells drawn as rainbow-bar pixels.
func countBarCells(rows [][]drawnCell) int {
	n := 0
	for _, row := range rows {
		for _, c := range row {
			if c.r == '▀' {
				n++
			}
		}
	}
	return n
}

func TestTrailWidth(t *testing.T) {
	tests := []struct {
		width, done, total, want int
	}{
		{40, 0, 10, 0},
		{40, 5, 10, 20},
		{40, 10, 10, 40},
		{40, 12, 10, 40}, // never past the end
		{40, 3, 0, 0},    // uncounted
		{0, 5, 10, 0},
	}
	for _, tt := range tests {
		if got := trailWidth(tt.width, tt.done, tt.total); got != tt.want {
			t.Errorf("trailWidth(%d, %d, %d) = %d, want %d", tt.width, tt.done, tt.total, got, tt.want)
		}
	}
}

func TestCatLeft(t *testing.T) {
	tests := []struct {
		trail, want int
	}{
		{0, 0},              // no trail yet: cat at the start
		{catWidth - 1, 0},   // trail shorter than the cat
		{20, 20 - catWidth}, // head at the trail's end
	}
	for _, tt := range tests {
		if got := catLeft(tt.trail); got != tt.want {
			t.Errorf("catLeft(%d) = %d, want %d", tt.trail, got, tt.want)
		}
	}
}

func TestRainbowCellsHalfway(t *testing.T) {
	const width = 40
	rows := rainbowCells(width, 5, 10, 0)
	if len(rows) != barRows {
		t.Fatalf("got %d rows, want %d", len(rows), barRows)
	}
	// Column 0 is trail: each text row holds two bands, upper as fg,
	// lower as bg.
	for y := range barRows {
		got := rows[y][0]
		if got.r != '▀' || got.fg != rainbowBands[2*y] || got.bg != rainbowBands[2*y+1] {
			t.Errorf("row %d col 0 = %+v, want bands %d/%d", y, got, 2*y, 2*y+1)
		}
	}
	// The cat's head sits at the trail's end: its eye ('k', pixel row 2,
	// column 8 of frame 0) is the upper half of text row 1.
	left := 20 - catWidth
	if got := rows[1][left+8].fg; got != catColors['k'] {
		t.Errorf("eye pixel fg = %v, want %v", got, catColors['k'])
	}
	// Past the trail and the cat, only night sky and stars.
	for y := range barRows {
		for x := 20; x < width; x++ {
			c := rows[y][x]
			if c.r == '▀' && (c.fg != trackColor || c.bg != trackColor) {
				t.Fatalf("track cell (%d,%d) = %+v, want night sky", x, y, c)
			}
		}
	}
}

func TestRainbowCellsEndsTheTrailBehindTheCat(t *testing.T) {
	rows := rainbowCells(40, 5, 10, 0)
	left := catLeft(trailWidth(40, 5, 10))
	// Frame 0's top row is "..tttttg.g": the gap between the ears
	// (column 8) shows the night sky, the gap before the tail (column 0)
	// the rainbow.
	if got := rows[0][left+8].fg; got != trackColor {
		t.Errorf("between the ears: fg = %v, want the night sky", got)
	}
	if got := rows[0][left].fg; got != rainbowBands[0] {
		t.Errorf("behind the tail: fg = %v, want the rainbow", got)
	}
}

func TestRainbowCellsEmptyAndFull(t *testing.T) {
	const width = 40
	empty := rainbowCells(width, 0, 10, 0)
	for _, row := range empty {
		for x, c := range row {
			for _, band := range rainbowBands {
				if c.r == '▀' && (c.fg == band || c.bg == band) {
					t.Fatalf("0%%: column %d has rainbow color %v", x, band)
				}
			}
		}
	}
	full := rainbowCells(width, 10, 10, 0)
	if got := full[0][0].fg; got != rainbowBands[0] {
		t.Errorf("100%%: column 0 fg = %v, want red", got)
	}
	// The cat is clamped to the right end: its rightmost ear is drawn in
	// the last column.
	if got := full[0][width-1].fg; got != catColors['g'] {
		t.Errorf("100%%: last column fg = %v, want the cat's ear", got)
	}
}

func TestRainbowCellsAnimatesTheCat(t *testing.T) {
	a := rainbowCells(40, 5, 10, 0)
	b := rainbowCells(40, 5, 10, 1)
	left := catLeft(trailWidth(40, 5, 10))
	same := true
	for y := range a {
		for x := left; x < left+catWidth; x++ {
			if a[y][x] != b[y][x] {
				same = false
			}
		}
	}
	if same {
		t.Error("frames 0 and 1 draw the same cat, want its tail and legs to move")
	}
}

func TestProgressModalUsesThePaletteForItsText(t *testing.T) {
	p := config.Palette{Background: "#101010", Border: "#202020", Text: "#30a030", Label: "#a03030", Accent: "#3030a0"}
	m := NewProgressModal()
	m.ApplyPalette(p)
	m.Reset("Loading queues", 10)
	m.SetFooter("Esc to cancel")

	rows := drawModal(t, m, 256)
	colorOf := func(row []drawnCell, text string) tcell.Color {
		i := strings.Index(rowText(row), text)
		if i < 0 {
			t.Fatalf("%q not drawn in %q", text, rowText(row))
		}
		return row[len([]rune(rowText(row)[:i]))].fg
	}
	if got := colorOf(rows[1], "Loading queues"); got != tcell.GetColor(p.Text) {
		t.Errorf("title color = %v, want the palette's text color", got)
	}
	if got := colorOf(rows[9], "Esc to cancel"); got != tcell.GetColor(p.Label) {
		t.Errorf("footer color = %v, want the palette's label color", got)
	}

	m.Reset("Loading queues", 0)
	rows = drawModal(t, m, 256)
	spinner := strings.IndexRune(rowText(rows[4]), spinnerRune(0))
	if spinner < 0 {
		t.Fatalf("no spinner in %q", rowText(rows[4]))
	}
	spinnerCol := len([]rune(rowText(rows[4])[:spinner]))
	if got := rows[4][spinnerCol].fg; got != tcell.GetColor(p.Accent) {
		t.Errorf("spinner color = %v, want the palette's accent color", got)
	}
	if got := colorOf(rows[4], "Loading queues"); got != tcell.GetColor(p.Text) {
		t.Errorf("spinner row title color = %v, want the palette's text color", got)
	}
}

func TestProgressModalSpinnerRowNamesTheOperation(t *testing.T) {
	m := NewProgressModal()
	m.Reset("Loading queues", 0)
	rows := drawModal(t, m, 256)
	want := string(spinnerRune(0)) + " Loading queues…"
	if got := rowText(rows[4]); !strings.Contains(got, want) {
		t.Errorf("spinner row = %q, want it to contain %q", got, want)
	}
	if got := rowText(rows[1]); strings.TrimSpace(strings.Trim(got, "│")) != "" {
		t.Errorf("title row = %q, want it empty with the title in the spinner row", got)
	}
}

func TestRainbowCellsStarsStayOffTheTrailAndCat(t *testing.T) {
	const width = 60
	// done 0 and 1 leave the trail shorter than the cat, so the cat
	// reaches past the trail onto the track.
	for _, done := range []int{0, 1, 3} {
		for frame := range 13 {
			rows := rainbowCells(width, done, 10, frame)
			trail := trailWidth(width, done, 10)
			left := catLeft(trail)
			for x, c := range rows[1] {
				if c.r != '✦' && c.r != '·' {
					continue
				}
				if x < trail || (x >= left && x < left+catWidth) {
					t.Errorf("done %d, frame %d: star at column %d, on the trail or the cat", done, frame, x)
				}
			}
		}
	}
}

func TestRainbowCellsNarrowerThanTheCat(t *testing.T) {
	rows := rainbowCells(5, 1, 2, 0)
	if len(rows) != barRows || len(rows[0]) != 5 {
		t.Fatalf("got %dx%d, want %dx5", len(rows), len(rows[0]), barRows)
	}
}

func TestProgressModalDrawsTheRainbowWithEnoughColors(t *testing.T) {
	m := NewProgressModal()
	m.Reset(`Deleting 10 message(s) from "orders"`, 10)
	m.SetProgress(3, 1)
	m.SetFooter("Esc to cancel")

	rows := drawModal(t, m, 256)
	text := screenText(rows)
	for _, want := range []string{`Deleting 10 message(s) from "orders"`, "3 of 10 (1 failed)", "Esc to cancel"} {
		if !strings.Contains(text, want) {
			t.Errorf("modal doesn't show %q:\n%s", want, text)
		}
	}
	// The bar fills the three bar rows across the inner width.
	if got, want := countBarCells(rows), barRows*(ProgressModalWidth-4); got < want*8/10 {
		t.Errorf("drew %d bar cells, want about %d", got, want)
	}
	if strings.ContainsRune(text, spinnerRune(0)) {
		t.Error("rainbow modal also draws the spinner")
	}
}

func TestProgressModalFallsBackToTheSpinner(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		colors    int
		force     bool
		wantCount string
	}{
		{"16-color terminal", 10, 16, false, "3 of 10"},
		{"uncounted operation", 0, 256, false, ""},
		{"forced spinner", 10, 256, true, "3 of 10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewProgressModal()
			m.Reset("Loading queues", tt.total)
			m.SetProgress(3, 0)
			m.SetForceSpinner(tt.force)

			rows := drawModal(t, m, tt.colors)
			if n := countBarCells(rows); n != 0 {
				t.Errorf("drew %d bar cells, want the spinner only", n)
			}
			line := rowText(rows[4]) // inner row 3, below the top border
			if !strings.ContainsRune(line, spinnerRune(0)) {
				t.Errorf("spinner row = %q, want the spinner", line)
			}
			if tt.wantCount != "" && !strings.Contains(line, tt.wantCount) {
				t.Errorf("spinner row = %q, want %q next to the spinner", line, tt.wantCount)
			}
			if tt.wantCount == "" && strings.Contains(line, " of ") {
				t.Errorf("spinner row = %q, want no count for an uncounted operation", line)
			}
		})
	}
}

func TestProgressModalSpinnerAnimates(t *testing.T) {
	m := NewProgressModal()
	m.Reset("Loading queues", 0)
	first := rowText(drawModal(t, m, 256)[4])
	m.SetFrame(1)
	second := rowText(drawModal(t, m, 256)[4])
	if first == second {
		t.Errorf("spinner row didn't change with the frame: %q", first)
	}
}

func TestProgressModalWrapsTheDetail(t *testing.T) {
	m := NewProgressModal()
	m.Reset("Loading queues", 0)
	m.SetDetail("AWS SSO session expired — opening browser to log in. Enter the code ABCD-EFGH at https://device.sso.example.com/ to continue, then wait here while it finishes")

	rows := drawModal(t, m, 256)
	first, second := rowText(rows[7]), rowText(rows[8]) // inner rows 6 and 7
	if !strings.Contains(first, "AWS SSO session expired") {
		t.Errorf("detail row 1 = %q", first)
	}
	if !strings.Contains(second, "ABCD-EFGH") || !strings.Contains(second, "…") {
		t.Errorf("detail row 2 = %q, want the code and a trailing … for the rest", second)
	}
}

func TestProgressModalShowsBracketsLiterally(t *testing.T) {
	m := NewProgressModal()
	m.Reset("Purging [orders]", 0)
	if text := screenText(drawModal(t, m, 256)); !strings.Contains(text, "Purging [orders]") {
		t.Errorf("title with brackets not shown literally:\n%s", text)
	}
}

func TestProgressModalResetClearsTheLastOperation(t *testing.T) {
	m := NewProgressModal()
	m.Reset("Deleting", 10)
	m.SetProgress(9, 2)
	m.SetDetail("old detail")
	m.SetFooter("Stopping after the current message…")
	m.Reset("Loading queues", 0)

	text := screenText(drawModal(t, m, 256))
	for _, stale := range []string{"old detail", "Stopping", "9 of 10"} {
		if strings.Contains(text, stale) {
			t.Errorf("modal still shows %q after Reset", stale)
		}
	}
}

func TestWrapLines(t *testing.T) {
	tests := []struct {
		text    string
		width   int
		maxRows int
		want    []string
	}{
		{"", 10, 2, nil},
		{"short", 10, 2, []string{"short"}},
		{"one two three", 7, 2, []string{"one two", "three"}},
		{"one two three four", 7, 2, []string{"one two", "three…"}},
		{"abcdefghijkl", 5, 2, []string{"abcd…"}},
	}
	for _, tt := range tests {
		got := wrapLines(tt.text, tt.width, tt.maxRows)
		if strings.Join(got, "|") != strings.Join(tt.want, "|") {
			t.Errorf("wrapLines(%q, %d, %d) = %q, want %q", tt.text, tt.width, tt.maxRows, got, tt.want)
		}
	}
}
