package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// press sends keys through p's real input handling (input capture
// included): a string types its runes, a tcell.Key presses that key.
func press(p tview.Primitive, keys ...any) {
	h := p.InputHandler()
	for _, k := range keys {
		switch k := k.(type) {
		case string:
			for _, r := range k {
				h(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), func(tview.Primitive) {})
			}
		case tcell.Key:
			h(tcell.NewEventKey(k, 0, tcell.ModNone), func(tview.Primitive) {})
		}
	}
}

// navigateTo presses Down until the arrow keys have highlighted want,
// failing the test if it never comes up.
func navigateTo(t *testing.T, field *tview.InputField, ac *jmsTypeAutocomplete, want string) {
	t.Helper()
	for range 10 {
		press(field, tcell.KeyDown)
		if ac.highlighted == want {
			return
		}
	}
	t.Fatalf("never highlighted %q (last: %q)", want, ac.highlighted)
}

// testPalette returns the embedded theme name's palette.
func testPalette(t *testing.T, name string) config.Palette {
	t.Helper()
	p, ok := config.PaletteForTheme(name)
	if !ok {
		t.Fatalf("no embedded theme %q", name)
	}
	return p
}

// dropDownRow is one drawn drop-down row: its text (trimmed) and the
// backgrounds of its non-blank cells.
type dropDownRow struct {
	text string
	bgs  map[tcell.Color]bool
}

// drawDropDown focuses field, draws it with its drop-down on a simulation
// screen, and returns the drop-down's rows, failing the test if no
// drop-down is drawn.
func drawDropDown(t *testing.T, field *tview.InputField) []dropDownRow {
	t.Helper()
	const width, height = 60, 6
	field.Focus(func(tview.Primitive) {})
	field.SetRect(0, 0, width, 1)
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)
	field.Draw(screen)
	screen.Show()

	cells, w, h := screen.GetContents()
	var rows []dropDownRow
	for y := 1; y < h; y++ { // the drop-down opens below the field's row
		var text strings.Builder
		bgs := map[tcell.Color]bool{}
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 || c.Runes[0] == ' ' {
				text.WriteRune(' ')
				continue
			}
			text.WriteRune(c.Runes[0])
			_, bg, _ := c.Style.Decompose()
			bgs[bg] = true
		}
		if row := strings.TrimSpace(text.String()); row != "" {
			rows = append(rows, dropDownRow{text: row, bgs: bgs})
		}
	}
	if len(rows) == 0 {
		t.Fatal("no drop-down drawn")
	}
	return rows
}

// highlightedEntries returns the drop-down rows drawn in p's selection
// background (see drawDropDown).
func highlightedEntries(t *testing.T, field *tview.InputField, p config.Palette) []string {
	t.Helper()
	var highlighted []string
	for _, row := range drawDropDown(t, field) {
		if row.bgs[tcell.GetColor(p.SelectionBg)] {
			highlighted = append(highlighted, row.text)
		}
	}
	return highlighted
}

// newTestAutocomplete wires a bare field with TypeA/TypeC plus the scan
// entry, styled from the "dark" theme, an open drop-down, and a done func
// recording keys.
func newTestAutocomplete(t *testing.T) (*tview.InputField, *jmsTypeAutocomplete, *int, *[]tcell.Key) {
	t.Helper()
	field := tview.NewInputField()
	scans := 0
	ac := newJMSTypeAutocomplete(field, testPalette(t, "dark"), func(string) []string {
		return []string{"TypeA", "TypeC", jmsTypeScanSentinel}
	}, func() { scans++ })
	var done []tcell.Key
	field.SetDoneFunc(func(k tcell.Key) { done = append(done, k) })
	field.Autocomplete()
	return field, ac, &scans, &done
}

func TestJMSTypeAutocompleteEnterAndTabKeepTypedTextWithoutAChoice(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyTab} {
		for _, typed := range []string{"", "TypeB", "Type"} {
			field, _, scans, done := newTestAutocomplete(t)

			press(field, typed, key)

			if got := field.GetText(); got != typed {
				t.Errorf("%s on %q: text = %q, want it kept", tcell.KeyNames[key], typed, got)
			}
			if *scans != 0 {
				t.Errorf("%s on %q: started a scan", tcell.KeyNames[key], typed)
			}
			if len(*done) != 1 || (*done)[0] != key {
				t.Errorf("%s on %q: done func got %v, want the key to reach it", tcell.KeyNames[key], typed, *done)
			}
		}
	}
}

func TestJMSTypeAutocompleteChosenTypeIsAccepted(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyEnter, tcell.KeyTab} {
		field, ac, scans, done := newTestAutocomplete(t)

		navigateTo(t, field, ac, "TypeC")
		press(field, key)

		if got := field.GetText(); got != "TypeC" {
			t.Errorf("%s: text = %q, want the chosen TypeC", tcell.KeyNames[key], got)
		}
		if *scans != 0 || len(*done) != 0 {
			t.Errorf("%s: scans=%d done=%v, want no scan and the key used for the choice", tcell.KeyNames[key], *scans, *done)
		}
	}
}

func TestJMSTypeAutocompleteChosenScanEntryRunsTheScan(t *testing.T) {
	field, ac, scans, _ := newTestAutocomplete(t)
	press(field, "Ty")

	navigateTo(t, field, ac, jmsTypeScanSentinel)
	press(field, tcell.KeyEnter)

	if *scans != 1 {
		t.Errorf("scans = %d, want 1", *scans)
	}
	if got := field.GetText(); got != "Ty" {
		t.Errorf("text = %q, want the typed %q (the scan entry never goes into the field)", got, "Ty")
	}
}

// TestJMSTypeAutocompleteArrowOnASingleEntryChoosesIt: with no types to
// suggest, the scan entry is the drop-down's only entry. Down can't move
// the highlight in a one-entry list, so tview reports no navigation — yet
// it's exactly when the user needs the scan (found live).
func TestJMSTypeAutocompleteArrowOnASingleEntryChoosesIt(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyDown, tcell.KeyUp} {
		field := tview.NewInputField()
		scans := 0
		newJMSTypeAutocomplete(field, testPalette(t, "dark"), func(string) []string {
			return []string{jmsTypeScanSentinel}
		}, func() { scans++ })
		field.Autocomplete()

		press(field, key, tcell.KeyEnter)

		if scans != 1 {
			t.Errorf("%s then Enter on the only entry: scans = %d, want 1", tcell.KeyNames[key], scans)
		}
	}
}

func TestJMSTypeAutocompleteNavigationKeepsTheDropDownAndText(t *testing.T) {
	field, ac, _, _ := newTestAutocomplete(t)

	seen := map[string]bool{}
	for range 3 {
		press(field, tcell.KeyDown)
		seen[ac.highlighted] = true
	}

	if len(seen) != 3 {
		t.Errorf("3 Downs highlighted %v, want all 3 entries (the drop-down collapsed?)", seen)
	}
	if got := field.GetText(); got != "" {
		t.Errorf("text after navigating = %q, want it untouched", got)
	}
}

func TestJMSTypeAutocompleteTypingAfterNavigatingDropsTheChoice(t *testing.T) {
	field, ac, scans, done := newTestAutocomplete(t)
	navigateTo(t, field, ac, jmsTypeScanSentinel)

	press(field, "TypeB", tcell.KeyEnter)

	if *scans != 0 {
		t.Error("Enter after typing started a scan from the earlier arrow-key choice")
	}
	if got := field.GetText(); got != "TypeB" || len(*done) != 1 {
		t.Errorf("text = %q, done = %v; want TypeB kept and Enter passed on", got, *done)
	}
}

// TestJMSTypeAutocompleteNothingChosenHighlightsNothing: tview always has
// a current entry, but while nothing is chosen it must look like the
// others — the highlight means "Enter takes this" (spec.md).
func TestJMSTypeAutocompleteNothingChosenHighlightsNothing(t *testing.T) {
	field, _, _, _ := newTestAutocomplete(t)

	if got := highlightedEntries(t, field, testPalette(t, "dark")); len(got) != 0 {
		t.Errorf("highlighted %q with nothing chosen, want none", got)
	}
}

// TestJMSTypeAutocompleteThemeSwitchKeepsNothingHighlighted: a live
// switch while the dialog is closed (the field unfocused, its drop-down
// cached) draws every row in the new theme's plain row colors.
func TestJMSTypeAutocompleteThemeSwitchKeepsNothingHighlighted(t *testing.T) {
	field, ac, _, _ := newTestAutocomplete(t)
	cyber := testPalette(t, "cyberpunk")

	ac.applyPalette(cyber)
	field.Autocomplete()

	_, main, _ := ui.AutocompleteStyles(cyber)
	_, rowBg, _ := main.Decompose()
	for _, row := range drawDropDown(t, field) {
		if len(row.bgs) != 1 || !row.bgs[rowBg] {
			t.Errorf("row %q drawn on %v, want only the new theme's row background %v", row.text, row.bgs, rowBg)
		}
	}
}

// TestJMSTypeAutocompleteResetDropsTheHighlight: a choice's look (here
// built by a theme switch while chosen) doesn't survive the dialog
// reopening.
func TestJMSTypeAutocompleteResetDropsTheHighlight(t *testing.T) {
	field, ac, _, _ := newTestAutocomplete(t)
	dark := testPalette(t, "dark")
	navigateTo(t, field, ac, "TypeC")
	field.Blur()
	ac.applyPalette(dark)
	field.Autocomplete()
	if got := highlightedEntries(t, field, dark); len(got) != 1 {
		t.Fatalf("highlighted %q while chosen, want one entry (test setup)", got)
	}

	ac.reset()
	field.Autocomplete()

	if got := highlightedEntries(t, field, dark); len(got) != 0 {
		t.Errorf("highlighted %q after reset, want none", got)
	}
}

// TestJMSTypeAutocompleteFirstArrowHighlightsTheCurrentEntry: the first
// arrow key shows the highlight on the entry tview has current (the first
// one starting with the typed text, else the first) instead of moving an
// invisible one, and Enter takes exactly that entry.
func TestJMSTypeAutocompleteFirstArrowHighlightsTheCurrentEntry(t *testing.T) {
	for _, tt := range []struct {
		typed string
		key   tcell.Key
		want  string
	}{
		{"", tcell.KeyDown, "TypeA"},
		{"", tcell.KeyUp, "TypeA"},
		{"", tcell.KeyPgDn, "TypeA"},
		{"TypeC", tcell.KeyDown, "TypeC"},
		{"Zzz", tcell.KeyDown, "TypeA"},
	} {
		name := fmt.Sprintf("%s on %q", tcell.KeyNames[tt.key], tt.typed)
		field, ac, _, done := newTestAutocomplete(t)
		dark := testPalette(t, "dark")
		press(field, tt.typed, tt.key)

		got := highlightedEntries(t, field, dark)
		if len(got) != 1 || got[0] != tt.want {
			t.Errorf("%s: highlighted %q, want only %q", name, got, tt.want)
		}
		if ac.highlighted != tt.want {
			t.Errorf("%s: recorded %q as chosen, want %q", name, ac.highlighted, tt.want)
		}

		press(field, tcell.KeyEnter)
		if text := field.GetText(); text != tt.want || len(*done) != 0 {
			t.Errorf("%s then Enter: text = %q, done = %v; want %q taken from the drop-down", name, text, *done, tt.want)
		}
	}
}

func TestJMSTypeAutocompleteLaterArrowsMoveTheHighlight(t *testing.T) {
	field, ac, _, _ := newTestAutocomplete(t)
	dark := testPalette(t, "dark")

	press(field, tcell.KeyDown, tcell.KeyDown)

	if got := highlightedEntries(t, field, dark); len(got) != 1 || got[0] != "TypeC" {
		t.Errorf("highlighted %q after two Downs, want only TypeC", got)
	}
	if ac.highlighted != "TypeC" {
		t.Errorf("recorded %q as chosen, want TypeC", ac.highlighted)
	}
}

// TestJMSTypeAutocompleteOtherKeysDropTheHighlight: after a choice, any
// key but Enter/Tab leaves no highlighted entry, and Enter then keeps the
// typed text.
func TestJMSTypeAutocompleteOtherKeysDropTheHighlight(t *testing.T) {
	for _, tt := range []struct {
		name string
		keys []any
	}{
		{"typing", []any{"x"}},
		{"Backspace", []any{tcell.KeyBackspace2}},
		{"Left", []any{tcell.KeyLeft}},
		{"Right", []any{tcell.KeyRight}},
	} {
		field, _, _, done := newTestAutocomplete(t)
		dark := testPalette(t, "dark")
		press(field, "Type", tcell.KeyDown)
		press(field, tt.keys...)
		typed := field.GetText()

		if got := highlightedEntries(t, field, dark); len(got) != 0 {
			t.Errorf("%s after a choice: highlighted %q, want none", tt.name, got)
		}
		press(field, tcell.KeyEnter)
		if text := field.GetText(); text != typed || len(*done) != 1 {
			t.Errorf("%s then Enter: text = %q, done = %v; want %q kept and Enter passed on", tt.name, text, *done, typed)
		}
	}
}

// TestJMSTypeAutocompleteEscAfterAChoiceOnlyClosesTheDropDown: Esc still
// closes the drop-down first, as before, and a second Esc reaches the
// done func.
func TestJMSTypeAutocompleteEscAfterAChoiceOnlyClosesTheDropDown(t *testing.T) {
	field, _, _, done := newTestAutocomplete(t)
	press(field, tcell.KeyDown, tcell.KeyEscape)
	if len(*done) != 0 {
		t.Fatalf("first Esc reached the done func (%v), want it to close the drop-down", *done)
	}

	press(field, tcell.KeyEscape)

	if len(*done) != 1 || (*done)[0] != tcell.KeyEscape {
		t.Errorf("second Esc: done = %v, want Esc", *done)
	}
}

// TestJMSTypeAutocompleteScanEntryUsesUpTheChoice: accepting the scan
// entry leaves the text alone, yet the choice is gone: Esc right after it
// reaches the done func instead of reopening and closing a drop-down.
func TestJMSTypeAutocompleteScanEntryUsesUpTheChoice(t *testing.T) {
	field, ac, scans, done := newTestAutocomplete(t)
	navigateTo(t, field, ac, jmsTypeScanSentinel)
	press(field, tcell.KeyEnter)
	if *scans != 1 {
		t.Fatalf("scans = %d, want 1 (test setup)", *scans)
	}

	press(field, tcell.KeyEscape)

	if len(*done) != 1 || (*done)[0] != tcell.KeyEscape {
		t.Errorf("Esc after the scan entry: done = %v, want Esc", *done)
	}
}

func TestJMSTypeAutocompleteSingleEntryHighlightedOnlyAfterAnArrow(t *testing.T) {
	dark := testPalette(t, "dark")
	field := tview.NewInputField()
	newJMSTypeAutocomplete(field, dark, func(string) []string {
		return []string{jmsTypeScanSentinel}
	}, func() {})
	field.Autocomplete()
	if got := highlightedEntries(t, field, dark); len(got) != 0 {
		t.Errorf("highlighted %q before an arrow key, want none", got)
	}

	press(field, tcell.KeyDown)

	if got := highlightedEntries(t, field, dark); len(got) != 1 || got[0] != jmsTypeScanSentinel {
		t.Errorf("highlighted %q after Down, want only the scan entry", got)
	}
}

// TestJMSTypeAutocompleteThemeSwitchKeepsTheChoiceHighlighted: a choice's
// drop-down rebuilt after a live switch (the theme picker had focus, so
// the field is unfocused) is highlighted in the new theme's colors.
func TestJMSTypeAutocompleteThemeSwitchKeepsTheChoiceHighlighted(t *testing.T) {
	field, ac, _, _ := newTestAutocomplete(t)
	cyber := testPalette(t, "cyberpunk")
	press(field, tcell.KeyDown)
	field.Blur()

	ac.applyPalette(cyber)
	field.Autocomplete()

	if got := highlightedEntries(t, field, cyber); len(got) != 1 || got[0] != "TypeA" {
		t.Errorf("highlighted %q in the new theme, want only TypeA", got)
	}
}
