package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// readyStatusText renders the bottom bar's idle-state hotkey legend from
// cfg — only the global hotkeys App.onGlobalKey actually wires up today.
func readyStatusText(cfg config.Config) string {
	key := func(k, desc string) string {
		return fmt.Sprintf("[%s]%s[-]: %s", cfg.Colors.Accent, k, desc)
	}
	return strings.Join([]string{
		key("?", "Help"),
		key("h", "Home"),
		key("s", "Settings"),
		key("q", "Quit"),
		key("/", "Filter"),
		key(":", "Command"),
	}, "  ")
}

// newStatusBar builds the bottom row: a single-line strip on the
// statusBarBg background, showing readyStatusText(cfg) at idle and
// transient status (loading indicators, errors) via setStatus otherwise.
func newStatusBar(cfg config.Config) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.GetColor(cfg.Colors.StatusBarText)).
		SetText(readyStatusText(cfg))
	tv.SetBackgroundColor(tcell.GetColor(cfg.Colors.StatusBarBg))
	return tv
}
