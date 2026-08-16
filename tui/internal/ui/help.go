package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

const (
	HelpModalWidth  = 40
	HelpModalHeight = 11
)

// NewHelpModal renders the '?' overlay's bordered keybinding list.
func NewHelpModal(cfg config.Config) *tview.TextView {
	key := func(k, desc string) string {
		return fmt.Sprintf("[%s]%-6s[-] [%s]%s[-]", cfg.Colors.Accent, k, cfg.Colors.Value, desc)
	}
	lines := []string{
		key("h", "home"),
		key("l", "log"),
		key("s", "settings"),
		key("q", "quit"),
		key("?", "toggle this help"),
		key(":", "command prompt"),
		key("esc", "close / cancel"),
	}

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetText(strings.Join(lines, "\n"))
	tv.SetBorder(true).SetTitle(" Help ")
	return tv
}

// Centered wraps p in a fixed-size box, centered within the available
// space — the standard tview nested-Flex pattern for a modal overlay.
func Centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, false).
			AddItem(nil, 0, 1, false), width, 1, false).
		AddItem(nil, 0, 1, false)
}
