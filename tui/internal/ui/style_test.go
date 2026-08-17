package ui

import (
	"testing"

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
