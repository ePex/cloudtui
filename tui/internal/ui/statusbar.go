package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// NewStatusBar builds the bottom row: a single-line strip on the
// statusBarBg background, blank at idle and showing transient status
// (loading indicators, errors, confirmations) via SetText elsewhere. It
// used to default to a global-hotkey legend, but that duplicated (and,
// once any transient message overwrote it, silently stopped showing) what
// Home's context panel now shows reliably — see
// spec/30-bugfix-home-context-panel-shortcuts and
// spec/31-bugfix-status-bar-duplicate-legend.
func NewStatusBar(cfg config.Config) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.GetColor(cfg.Colors.StatusBarText))
	tv.SetBackgroundColor(tcell.GetColor(cfg.Colors.StatusBarBg))
	return tv
}
