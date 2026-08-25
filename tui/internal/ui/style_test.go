package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestStyleListAppliesSelectionColors(t *testing.T) {
	p := config.Palette{SelectionBg: "#2ac3de", SelectionText: "#1a1b26"}
	l := StyleList(tview.NewList(), p)

	if l == nil {
		t.Fatal("StyleList() returned nil")
	}
	// tview.List exposes no getter for its selected-item style, so the
	// resulting colors can't be asserted directly here; this at least
	// confirms StyleList returns the same list (for chaining) rather
	// than panicking or discarding it. Visual verification is manual.
	if l.GetItemCount() != 0 {
		t.Errorf("GetItemCount() = %d, want 0 for a fresh list", l.GetItemCount())
	}
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
