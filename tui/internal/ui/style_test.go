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

// newFilterInput builds a " / filter: " input holding "abc" while
// tview.Styles holds theme, styled with StyleFilterInput at construction
// the way every view and picker builds its filter input.
func newFilterInput(t *testing.T, theme string) *tview.InputField {
	t.Helper()
	p := mustPalette(t, theme)
	withTviewStyles(t, p)
	i := tview.NewInputField().SetLabel(" / filter: ").SetText("abc")
	return StyleFilterInput(i, p)
}

func TestStyleFilterInputRecolorsLabelBackgroundAndField(t *testing.T) {
	cyber := mustPalette(t, "cyberpunk")
	i := newFilterInput(t, "dark")

	if got := StyleFilterInput(i, cyber); got != i {
		t.Fatal("StyleFilterInput did not return the same field for chaining")
	}
	rows := renderCells(t, i, 30, 1)

	assertCellColors(t, "label", findText(t, rows, "/ filter:"), cyber.Label, cyber.Background)
	assertCellColors(t, "field text", findText(t, rows, "abc"), cyber.SelectionText, cyber.SelectionBg)
}

// TestStyleFilterInputMatchesRestart checks a live switch draws the whole
// input exactly as one built under cyberpunk from the start does.
func TestStyleFilterInputMatchesRestart(t *testing.T) {
	cyber := mustPalette(t, "cyberpunk")
	live := StyleFilterInput(newFilterInput(t, "dark"), cyber)
	liveRows := renderCells(t, live, 30, 1)

	assertSameCells(t, liveRows, renderCells(t, newFilterInput(t, "cyberpunk"), 30, 1))
}

// TestStyleFilterInputControlWithoutRestyleKeepsOldLabelBackground is the
// control: the three setters the views used before this fix leave the
// label on the construction-time (dark) background.
func TestStyleFilterInputControlWithoutRestyleKeepsOldLabelBackground(t *testing.T) {
	dark, cyber := mustPalette(t, "dark"), mustPalette(t, "cyberpunk")
	i := newFilterInput(t, "dark")
	ApplyTviewStyles(cyber)
	i.SetLabelColor(tcell.GetColor(cyber.Label))
	i.SetFieldBackgroundColor(tcell.GetColor(cyber.SelectionBg))
	i.SetFieldTextColor(tcell.GetColor(cyber.SelectionText))

	assertCellColors(t, "label", findText(t, renderCells(t, i, 30, 1), "/ filter:"), cyber.Label, dark.Background)
}

// dropDownState puts a freshly built dropdown into the state a test
// renders it in: unfocused, focused (closed), or open (popup list shown).
type dropDownState int

const (
	ddUnfocused dropDownState = iota
	ddFocused
	ddOpen
)

// focusTree focuses p the way tview.Application does, recursing into
// whatever p delegates focus to.
func focusTree(p tview.Primitive) {
	var focus func(tview.Primitive)
	focus = func(p tview.Primitive) { p.Focus(focus) }
	focus(p)
}

// newStandaloneDropDown builds a " Env: " dropdown (prod/dev, prod
// selected) while tview.Styles holds theme, styled with StyleDropDown at
// construction the way the Datadog view builds its filters, then puts it
// into state.
func newStandaloneDropDown(t *testing.T, theme string, state dropDownState) *tview.DropDown {
	t.Helper()
	p := mustPalette(t, theme)
	withTviewStyles(t, p)
	dd := tview.NewDropDown().SetLabel(" Env: ").SetOptions([]string{"prod", "dev"}, nil)
	dd.SetCurrentOption(0)
	StyleDropDown(dd, p)
	setDropDownState(dd, state)
	return dd
}

func setDropDownState(dd *tview.DropDown, state dropDownState) {
	if state == ddUnfocused {
		return
	}
	focusTree(dd)
	if state == ddOpen {
		dd.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { focusTree(p) })
	}
}

func TestStyleDropDownRecolorsLabelAndField(t *testing.T) {
	cyber := mustPalette(t, "cyberpunk")
	dd := newStandaloneDropDown(t, "dark", ddUnfocused)

	if got := StyleDropDown(dd, cyber); got != dd {
		t.Fatal("StyleDropDown did not return the same dropdown for chaining")
	}
	rows := renderCells(t, dd, 30, 4)

	assertCellColors(t, "label", findText(t, rows, "Env:"), cyber.Label, cyber.Background)
	assertCellColors(t, "field", findText(t, rows, "prod"), cyber.SelectionText, cyber.SelectionBg)
}

// TestStyleDropDownMatchesRestart checks a live switch draws a
// stand-alone dropdown — unfocused, focused, and open — exactly as one
// built under cyberpunk from the start does.
func TestStyleDropDownMatchesRestart(t *testing.T) {
	cyber := mustPalette(t, "cyberpunk")
	for _, tt := range []struct {
		name  string
		state dropDownState
	}{
		{"unfocused", ddUnfocused},
		{"focused", ddFocused},
		{"open", ddOpen},
	} {
		t.Run(tt.name, func(t *testing.T) {
			live := StyleDropDown(newStandaloneDropDown(t, "dark", tt.state), cyber)
			liveRows := renderCells(t, live, 30, 4)
			if tt.state == ddOpen {
				findText(t, liveRows, "dev") // only drawn while the popup list is open
			}
			assertSameCells(t, liveRows, renderCells(t, newStandaloneDropDown(t, "cyberpunk", tt.state), 30, 4))
		})
	}
}

// newFormWithDropDown builds a form holding a "Backend" dropdown
// (jolokia/proxy) while tview.Styles holds theme, styled the way the
// connection editor builds it (StyleForm + StyleFormDropDown), then puts
// the dropdown into state.
func newFormWithDropDown(t *testing.T, theme string, state dropDownState) *tview.Form {
	t.Helper()
	p := mustPalette(t, theme)
	withTviewStyles(t, p)
	f := tview.NewForm().AddDropDown("Backend", []string{"jolokia", "proxy"}, 0, nil)
	f.SetBackgroundColor(tcell.GetColor(p.Background))
	StyleForm(f, p)
	dd := f.GetFormItem(0).(*tview.DropDown)
	StyleFormDropDown(dd, p)
	if state != ddUnfocused {
		focusTree(f)
		if state == ddOpen {
			f.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { focusTree(p) })
		}
	}
	return f
}

// TestStyleFormDropDownMatchesRestart checks the in-form case: StyleForm
// plus StyleFormDropDown draw the dropdown — unfocused, focused, and open
// — exactly as a restart does.
func TestStyleFormDropDownMatchesRestart(t *testing.T) {
	cyber := mustPalette(t, "cyberpunk")
	for _, tt := range []struct {
		name  string
		state dropDownState
	}{
		{"unfocused", ddUnfocused},
		{"focused", ddFocused},
		{"open", ddOpen},
	} {
		t.Run(tt.name, func(t *testing.T) {
			live := newFormWithDropDown(t, "dark", tt.state)
			live.SetBackgroundColor(tcell.GetColor(cyber.Background))
			StyleForm(live, cyber)
			StyleFormDropDown(live.GetFormItem(0).(*tview.DropDown), cyber)
			liveRows := renderCells(t, live, 40, 6)
			if tt.state == ddOpen {
				findText(t, liveRows, "proxy") // only drawn while the popup list is open
			}
			assertSameCells(t, liveRows, renderCells(t, newFormWithDropDown(t, "cyberpunk", tt.state), 40, 6))
		})
	}
}

// TestStyleDropDownControlWithoutRestyleKeepsOldColors is the control: the
// setters the Datadog view used before this fix (label and field colors
// only at construction, then just the popup list styles on a switch)
// leave the label on the construction-time (dark) background and the
// field in dark's colors.
func TestStyleDropDownControlWithoutRestyleKeepsOldColors(t *testing.T) {
	dark, cyber := mustPalette(t, "dark"), mustPalette(t, "cyberpunk")
	dd := newStandaloneDropDown(t, "dark", ddUnfocused)
	ApplyTviewStyles(cyber)
	StyleFormDropDown(dd, cyber) // what StyleDropDown used to be: list styles only

	rows := renderCells(t, dd, 30, 4)
	assertCellColors(t, "label", findText(t, rows, "Env:"), dark.Label, dark.Background)
	assertCellColors(t, "field", findText(t, rows, "prod"), dark.SelectionText, dark.SelectionBg)
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

// newAutocompleteField builds an input field with a two-entry autocomplete
// while tview.Styles holds theme, styled and wired the way the app does
// (styles first, then SetAutocompleteFunc, which builds the drop-down's
// list right away).
func newAutocompleteField(t *testing.T, theme string) *tview.InputField {
	t.Helper()
	p := mustPalette(t, theme)
	withTviewStyles(t, p)
	i := tview.NewInputField().SetLabel("Type: ")
	StyleInputFieldAutocomplete(i, p)
	i.SetAutocompleteFunc(func(string) []string { return []string{"alpha", "beta"} })
	return i
}

// openDropDown focuses i and looks up suggestions, so its drop-down draws.
func openDropDown(i *tview.InputField) *tview.InputField {
	focusTree(i)
	i.Autocomplete()
	return i
}

// TestStyleInputFieldAutocompleteRecolorsCachedDropDown checks a live
// switch recolors an autocomplete drop-down whose list tview already
// built (SetAutocompleteFunc builds it immediately): once opened, it
// draws exactly like one built under cyberpunk from the start.
func TestStyleInputFieldAutocompleteRecolorsCachedDropDown(t *testing.T) {
	cyber := mustPalette(t, "cyberpunk")
	live := newAutocompleteField(t, "dark")
	ApplyTviewStyles(cyber)
	StyleInputFieldAutocomplete(live, cyber)
	liveRows := renderCells(t, openDropDown(live), 30, 4)
	findText(t, liveRows, "beta") // the drop-down is really open

	// Only the drop-down's entries: the field itself (and the blank cells
	// beside the drop-down) are styled by a form or StyleFilterInput, not
	// by this helper.
	restartedRows := renderCells(t, openDropDown(newAutocompleteField(t, "cyberpunk")), 30, 4)
	for _, entry := range []string{"alpha", "beta"} {
		assertSameCells(t, [][]renderedCell{findText(t, liveRows, entry)}, [][]renderedCell{findText(t, restartedRows, entry)})
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
