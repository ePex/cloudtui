package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestNewStatusBar(t *testing.T) {
	cfg := config.Default()
	tv := newStatusBar(cfg)

	if got, want := tv.GetText(true), readyStatusText(cfg); got != want {
		t.Errorf("status bar text = %q, want %q", got, want)
	}
	if got, want := tv.GetBackgroundColor(), tcell.GetColor(cfg.Colors.StatusBarBg); got != want {
		t.Errorf("status bar background color = %v, want %v", got, want)
	}
}

func TestReadyStatusTextContainsGlobalHotkeys(t *testing.T) {
	text := readyStatusText(config.Default())

	for _, want := range []string{"Help", "Home", "Settings", "Quit", "Filter", "Command"} {
		if !strings.Contains(text, want) {
			t.Errorf("readyStatusText() = %q, want it to contain %q", text, want)
		}
	}
}
