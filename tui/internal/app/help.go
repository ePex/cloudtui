package app

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

const (
	helpModalWidth  = 40
	helpModalHeight = 11
)

// newHelpModal renders the '?' overlay's bordered keybinding list.
func newHelpModal(cfg config.Config) *tview.TextView {
	key := func(k, desc string) string {
		return fmt.Sprintf("[%s]%-6s[-] [%s]%s[-]", cfg.Colors.Accent, k, cfg.Colors.Value, desc)
	}
	lines := []string{
		key("h", "home"),
		key("s", "settings"),
		key("q", "quit"),
		key("?", "toggle this help"),
		key("/", "filter (if the view supports it)"),
		key(":", "command prompt"),
		key("esc", "close / cancel"),
	}

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetText(strings.Join(lines, "\n"))
	tv.SetBorder(true).SetTitle(" Help ")
	return tv
}

// centered wraps p in a fixed-size box, centered within the available
// space — the standard tview nested-Flex pattern for a modal overlay.
func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, false).
			AddItem(nil, 0, 1, false), width, 1, false).
		AddItem(nil, 0, 1, false)
}
