package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestNewStatusBarBlankAtIdle(t *testing.T) {
	cfg := config.Default()
	tv := newStatusBar(cfg)

	if got := tv.GetText(false); got != "" {
		t.Errorf("status bar text = %q, want empty (no default legend — see Home's context panel instead)", got)
	}
	if got, want := tv.GetBackgroundColor(), tcell.GetColor(cfg.Colors.StatusBarBg); got != want {
		t.Errorf("status bar background color = %v, want %v", got, want)
	}
}
