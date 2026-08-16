package ui

import (
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestNewHelpModalContainsBindings(t *testing.T) {
	text := NewHelpModal(config.Default()).GetText(true)

	for _, want := range []string{"h", "home", "s", "settings", "q", "quit", "?", ":", "command", "esc"} {
		if !strings.Contains(text, want) {
			t.Errorf("help modal text = %q, want it to contain %q", text, want)
		}
	}
}

func TestCenteredWrapsInThreeItems(t *testing.T) {
	inner := tview.NewBox()

	flex, ok := Centered(inner, HelpModalWidth, HelpModalHeight).(*tview.Flex)
	if !ok {
		t.Fatalf("Centered() returned %T, want *tview.Flex", Centered(inner, HelpModalWidth, HelpModalHeight))
	}
	if got, want := flex.GetItemCount(), 3; got != want {
		t.Errorf("Centered() top-level item count = %d, want %d", got, want)
	}
}
