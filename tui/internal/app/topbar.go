package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// topBar is the app's top row: a "info"/"prompt"/"filter" Pages on the
// left (connection info, replaced by the command prompt or the filter
// input while either is active) and a shortcuts+logo panel on the right.
type topBar struct {
	root   *tview.Flex
	left   *tview.Pages
	info   *tview.TextView
	height int
}

// newTopBar builds the top bar. prompt and filterInput are added as the
// left Pages' "prompt"/"filter" pages so the app can switch to either
// while a command or a filter query is being typed.
func newTopBar(cfg config.Config, prompt, filterInput *tview.InputField) *topBar {
	info := newInfoPanel(cfg)
	left := tview.NewPages().
		AddPage("info", info, true, true).
		AddPage("prompt", prompt, true, false).
		AddPage("filter", filterInput, true, false)

	right := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(newShortcutsPanel(cfg), 0, 1, false).
		AddItem(newLogoPanel(cfg), logoWidth(cfg.Logo), 0, false)

	root := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(left, 0, 1, false).
		AddItem(right, 0, 1, false)

	return &topBar{
		root:   root,
		left:   left,
		info:   info,
		height: maxInt(2, 3, len(cfg.Logo)),
	}
}

// infoPanelText renders the connection-info panel's content from cfg —
// shared between the initial render and later refreshes (e.g. after the
// AWS profile changes).
func infoPanelText(cfg config.Config) string {
	line := func(label, value string) string {
		return fmt.Sprintf("[%s]%s:[-] [%s]%s[-]", cfg.Colors.Label, label, cfg.Colors.Value, value)
	}

	profile := cfg.AWS.Profile
	if profile == "" {
		profile = "(not configured)"
	}

	return strings.Join([]string{
		line("Profile", profile),
		line("Queue Broker", "(not configured)"),
	}, "\n")
}

func newInfoPanel(cfg config.Config) *tview.TextView {
	return tview.NewTextView().
		SetDynamicColors(true).
		SetText(infoPanelText(cfg))
}

func newShortcutsPanel(cfg config.Config) *tview.TextView {
	key := func(k string) string {
		return fmt.Sprintf("[%s]%s[-]", cfg.Colors.Accent, k)
	}
	text := strings.Join([]string{
		key(":") + " command",
		key("q") + "/" + key("quit") + " quit",
		key("esc") + " cancel",
	}, "\n")

	return tview.NewTextView().
		SetDynamicColors(true).
		SetText(text)
}

// newLogoPanel renders the configured ASCII logo. Dynamic colors are left
// off since arbitrary logo art may contain literal "[" characters that
// would otherwise be misparsed as color tags.
func newLogoPanel(cfg config.Config) *tview.TextView {
	return tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetText(strings.Join(cfg.Logo, "\n"))
}

// logoWidth returns the display width (in terminal cells) of the widest
// line in logo, so the top bar's right column doesn't clip a custom logo.
func logoWidth(logo []string) int {
	width := 0
	for _, line := range logo {
		if n := utf8.RuneCountInString(line); n > width {
			width = n
		}
	}
	return width
}

func maxInt(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
