package app

import (
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestNewHelpModalContainsBindings(t *testing.T) {
	text := newHelpModal(config.Default()).GetText(true)

	for _, want := range []string{"h", "home", "s", "settings", "q", "quit", "?", "/", "filter", ":", "command", "esc"} {
		if !strings.Contains(text, want) {
			t.Errorf("help modal text = %q, want it to contain %q", text, want)
		}
	}
}

func TestCenteredWrapsInThreeItems(t *testing.T) {
	inner := tview.NewBox()

	flex, ok := centered(inner, helpModalWidth, helpModalHeight).(*tview.Flex)
	if !ok {
		t.Fatalf("centered() returned %T, want *tview.Flex", centered(inner, helpModalWidth, helpModalHeight))
	}
	if got, want := flex.GetItemCount(), 3; got != want {
		t.Errorf("centered() top-level item count = %d, want %d", got, want)
	}
}
