package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// renderedCell is one drawn screen cell: its rune and colors.
type renderedCell struct {
	r      rune
	fg, bg tcell.Color
}

// renderCells draws prim into a width×height simulation screen and
// returns every cell's rune and colors, row by row.
func renderCells(t *testing.T, prim tview.Primitive, width, height int) [][]renderedCell {
	t.Helper()
	prim.SetRect(0, 0, width, height)
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)
	prim.Draw(screen)
	screen.Show()

	cells, w, h := screen.GetContents()
	rows := make([][]renderedCell, h)
	for y := 0; y < h; y++ {
		rows[y] = make([]renderedCell, w)
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			fg, bg, _ := c.Style.Decompose()
			var r rune = ' '
			if len(c.Runes) > 0 {
				r = c.Runes[0]
			}
			rows[y][x] = renderedCell{r: r, fg: fg, bg: bg}
		}
	}
	return rows
}

// findText returns the cells of the first on-screen occurrence of text,
// failing the test if it isn't drawn anywhere.
func findText(t *testing.T, rows [][]renderedCell, text string) []renderedCell {
	t.Helper()
	want := []rune(text)
	for _, row := range rows {
		for x := 0; x+len(want) <= len(row); x++ {
			match := true
			for i, r := range want {
				if row[x+i].r != r {
					match = false
					break
				}
			}
			if match {
				return row[x : x+len(want)]
			}
		}
	}
	t.Fatalf("%q is not drawn on screen", text)
	return nil
}

// assertCellColors fails unless every cell has fg on bg (palette hex values).
func assertCellColors(t *testing.T, what string, cells []renderedCell, fg, bg string) {
	t.Helper()
	for _, c := range cells {
		if c.fg != tcell.GetColor(fg) || c.bg != tcell.GetColor(bg) {
			t.Errorf("%s: cell %q drawn %v on %v, want %s on %s", what, c.r, c.fg, c.bg, fg, bg)
			return
		}
	}
}

// newThemedTestList builds a list while tview.Styles holds the "dark"
// theme — as every list in the app is built at startup — with item 0
// selected and item 1 unselected.
func newThemedTestList(t *testing.T) *tview.List {
	t.Helper()
	withTviewStyles(t, mustPalette(t, "dark"))
	l := tview.NewList()
	l.AddItem("first", "sub", 'a', nil)
	l.AddItem("second", "", 0, nil)
	return l
}

func TestStyleListRecolorsEveryItemStyle(t *testing.T) {
	l := newThemedTestList(t)
	cyber := mustPalette(t, "cyberpunk")

	if got := StyleList(l, cyber); got != l {
		t.Fatal("StyleList did not return the same list for chaining")
	}
	rows := renderCells(t, l, 30, 6)

	assertCellColors(t, "unselected main text", findText(t, rows, "second"), cyber.Text, cyber.Background)
	assertCellColors(t, "secondary text", findText(t, rows, "sub"), cyber.Label, cyber.Background)
	assertCellColors(t, "shortcut", findText(t, rows, "(a)"), cyber.Value, cyber.Background)
	assertCellColors(t, "selected main text", findText(t, rows, "first"), cyber.SelectionText, cyber.SelectionBg)
}

// assertSameCells fails at the first cell where live and restarted differ
// in rune or color.
func assertSameCells(t *testing.T, live, restarted [][]renderedCell) {
	t.Helper()
	for y := range live {
		for x := range live[y] {
			got, want := live[y][x], restarted[y][x]
			if got != want {
				t.Fatalf("cell (%d,%d): live switch drew %q %v on %v, restart draws %q %v on %v",
					x, y, got.r, got.fg, got.bg, want.r, want.fg, want.bg)
			}
		}
	}
}

// TestStyleListFreshListMatchesRestart checks that a live switch (build
// under dark, StyleList with cyberpunk) draws the whole list exactly as a
// list built under cyberpunk from the start does — i.e. as after a
// restart.
func TestStyleListFreshListMatchesRestart(t *testing.T) {
	cyber := mustPalette(t, "cyberpunk")
	live := StyleList(newThemedTestList(t), cyber)
	live.SetBackgroundColor(tcell.GetColor(cyber.Background)) // done by each dialog's own ApplyPalette
	liveRows := renderCells(t, live, 30, 6)

	withTviewStyles(t, cyber)
	restarted := StyleList(tview.NewList(), cyber)
	restarted.AddItem("first", "sub", 'a', nil)
	restarted.AddItem("second", "", 0, nil)

	assertSameCells(t, liveRows, renderCells(t, restarted, 30, 6))
}

// TestStyleListControlWithoutRestyleKeepsOldColors is the control for the
// tests above: without StyleList, the unselected row really does keep
// the construction-time (dark) colors, so those tests can tell the
// difference.
func TestStyleListControlWithoutRestyleKeepsOldColors(t *testing.T) {
	dark := mustPalette(t, "dark")
	l := newThemedTestList(t)
	ApplyTviewStyles(mustPalette(t, "cyberpunk")) // live switch without restyling the list

	rows := renderCells(t, l, 30, 6)
	assertCellColors(t, "unselected main text", findText(t, rows, "second"), dark.Text, dark.Background)
}

// newThemedTestForm builds a form while tview.Styles holds the "dark"
// theme: a "Name" field holding "abc", then Save/Cancel buttons, with the
// Save button focused (so it draws in the activated style).
func newThemedTestForm(t *testing.T) *tview.Form {
	t.Helper()
	withTviewStyles(t, mustPalette(t, "dark"))
	f := tview.NewForm().
		AddInputField("Name", "abc", 10, nil, nil).
		AddButton("Save", nil).
		AddButton("Cancel", nil)
	f.SetFocus(1) // Save: items first, then buttons
	var focus func(tview.Primitive)
	focus = func(p tview.Primitive) { p.Focus(focus) }
	f.Focus(focus)
	return f
}

func TestStyleFormRecolorsLabelFieldAndButtons(t *testing.T) {
	f := newThemedTestForm(t)
	cyber := mustPalette(t, "cyberpunk")
	f.SetBackgroundColor(tcell.GetColor(cyber.Background)) // done by each dialog's own ApplyPalette

	if got := StyleForm(f, cyber); got != f {
		t.Fatal("StyleForm did not return the same form for chaining")
	}
	rows := renderCells(t, f, 40, 6)

	for _, c := range findText(t, rows, "Name") {
		if c.fg != tcell.GetColor(cyber.Value) {
			t.Errorf("label %q drawn in %v, want %s", c.r, c.fg, cyber.Value)
			break
		}
	}
	assertCellColors(t, "field text", findText(t, rows, "abc"), cyber.Text, cyber.Background)
	assertCellColors(t, "unfocused button", findText(t, rows, "Cancel"), cyber.Text, cyber.Background)
	assertCellColors(t, "focused (activated) button", findText(t, rows, "Save"), cyber.Background, cyber.Text)
}

// TestStyleFormMatchesRestart checks a live switch draws the form's
// label, field, and buttons exactly as a form built under cyberpunk from
// the start does.
func TestStyleFormMatchesRestart(t *testing.T) {
	cyber := mustPalette(t, "cyberpunk")
	live := newThemedTestForm(t)
	live.SetBackgroundColor(tcell.GetColor(cyber.Background))
	StyleForm(live, cyber)
	liveRows := renderCells(t, live, 40, 6)

	withTviewStyles(t, cyber)
	restarted := tview.NewForm().
		AddInputField("Name", "abc", 10, nil, nil).
		AddButton("Save", nil).
		AddButton("Cancel", nil)
	restarted.SetFocus(1)
	var focus func(tview.Primitive)
	focus = func(p tview.Primitive) { p.Focus(focus) }
	restarted.Focus(focus)
	assertSameCells(t, liveRows, renderCells(t, restarted, 40, 6))
}

// TestStyleFormControlWithoutRestyleKeepsOldColors is the control: without
// StyleForm, the field keeps the construction-time (dark) colors.
func TestStyleFormControlWithoutRestyleKeepsOldColors(t *testing.T) {
	dark := mustPalette(t, "dark")
	f := newThemedTestForm(t)
	ApplyTviewStyles(mustPalette(t, "cyberpunk"))

	assertCellColors(t, "field text", findText(t, renderCells(t, f, 40, 6), "abc"), dark.Text, dark.Background)
}

func TestStyleInputFieldAutocompleteReturnsField(t *testing.T) {
	p := config.Palette{
		Background:    "#1a1b26",
		Text:          "#c0caf5",
		Accent:        "#ff79c6",
		SelectionBg:   "#2ac3de",
		SelectionText: "#1a1b26",
	}
	i := StyleInputFieldAutocomplete(tview.NewInputField(), p)

	// tview.InputField exposes no getter for its autocomplete styles, so
	// the resulting colors can't be asserted directly here; this at least
	// confirms StyleInputFieldAutocomplete returns the same field (for
	// chaining) rather than panicking or discarding it. Visual
	// verification is manual.
	if i == nil {
		t.Fatal("StyleInputFieldAutocomplete() returned nil")
	}
}

func TestBlendColors(t *testing.T) {
	background := tcell.NewRGBColor(0x1a, 0x1b, 0x26)
	accent := tcell.NewRGBColor(0xff, 0x79, 0xc6)

	tests := []struct {
		name string
		a, b tcell.Color
		t    float64
		want tcell.Color
	}{
		{
			name: "t=0 returns a unchanged",
			a:    background, b: accent, t: 0,
			want: background,
		},
		{
			name: "t=1 returns b unchanged",
			a:    background, b: accent, t: 1,
			want: accent,
		},
		{
			name: "midpoint blends component-wise",
			a:    tcell.NewRGBColor(0, 0, 0), b: tcell.NewRGBColor(100, 200, 50), t: 0.5,
			want: tcell.NewRGBColor(50, 100, 25),
		},
		{
			name: "invalid b returns a unchanged",
			a:    background, b: tcell.ColorDefault, t: 0.15,
			want: background,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BlendColors(tt.a, tt.b, tt.t); got != tt.want {
				gr, gg, gb := got.RGB()
				wr, wg, wb := tt.want.RGB()
				t.Errorf("BlendColors() = rgb(%d,%d,%d), want rgb(%d,%d,%d)", gr, gg, gb, wr, wg, wb)
			}
		})
	}
}
