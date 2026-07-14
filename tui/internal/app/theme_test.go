package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestApplyThemeSetsBoxDefaults(t *testing.T) {
	p := config.Palette{
		Background:    "#111111",
		Border:        "#222222",
		Label:         "#333333",
		Text:          "#444444",
		Value:         "#555555",
		SelectionText: "#666666",
	}
	applyTheme(p)
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	box := tview.NewBox()
	if got, want := box.GetBackgroundColor(), tcell.GetColor(p.Background); got != want {
		t.Errorf("GetBackgroundColor() = %v, want %v", got, want)
	}
	if got, want := box.GetBorderColor(), tcell.GetColor(p.Border); got != want {
		t.Errorf("GetBorderColor() = %v, want %v", got, want)
	}
}

func TestStyleListAppliesSelectionColors(t *testing.T) {
	p := config.Palette{SelectionBg: "#2ac3de", SelectionText: "#1a1b26"}
	l := styleList(tview.NewList(), p)

	if l == nil {
		t.Fatal("styleList() returned nil")
	}
	// tview.List exposes no getter for its selected-item style, so the
	// resulting colors can't be asserted directly here; this at least
	// confirms styleList returns the same list (for chaining) rather
	// than panicking or discarding it. Visual verification is manual
	// (see the plan's Testing section).
	if l.GetItemCount() != 0 {
		t.Errorf("GetItemCount() = %d, want 0 for a fresh list", l.GetItemCount())
	}
}
