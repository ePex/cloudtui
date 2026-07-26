package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// topBar is the app's top row: a "info"/"prompt" Pages on the left
// (connection info, replaced by the command prompt while active),
// a single-char divider, a context panel (view-specific shortcuts,
// empty by default), and the logo.
type topBar struct {
	root         *tview.Flex
	left         *tview.Pages
	info         *tview.TextView
	divider      *tview.TextView
	contextPanel *tview.TextView
	logo         *tview.TextView
	height       int
}

// newTopBar builds the top bar. prompt is added as the left Pages' "prompt"
// page so the app can switch to it while a command is being typed.
func newTopBar(cfg config.Config, prompt *tview.InputField) *topBar {
	info := newInfoPanel(cfg)
	left := tview.NewPages().
		AddPage("info", info, true, true).
		AddPage("prompt", prompt, true, false)

	contextPanel := tview.NewTextView().SetDynamicColors(true)
	logoPanel := newLogoPanel(cfg)
	height := maxInt(1, len(cfg.Logo))
	divider := newDivider(cfg, height)

	root := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(left, 0, 1, false).
		AddItem(divider, 1, 0, false).
		AddItem(contextPanel, 0, 1, false).
		AddItem(logoPanel, logoWidth(cfg.Logo), 0, false)

	return &topBar{
		root:         root,
		left:         left,
		info:         info,
		divider:      divider,
		contextPanel: contextPanel,
		logo:         logoPanel,
		height:       height,
	}
}

func newDivider(cfg config.Config, height int) *tview.TextView {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = "│"
	}
	return tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[%s]%s[-]", cfg.Colors.Border, strings.Join(lines, "\n")))
}

// infoPanelText renders the connection-info panel's content from cfg.
// The panel is a placeholder for now; an AWS profile feature will add a second
// line. The active theme name is the sole connection-context item for now.
func infoPanelText(cfg config.Config) string {
	return fmt.Sprintf("[%s]Theme:[-] [%s]%s[-]", cfg.Colors.Label, cfg.Colors.Value, cfg.Theme)
}

func newInfoPanel(cfg config.Config) *tview.TextView {
	return tview.NewTextView().
		SetDynamicColors(true).
		SetText(infoPanelText(cfg))
}

// newLogoPanel renders the configured ASCII logo. Dynamic colors are left off
// since arbitrary logo art may contain literal "[" characters that would
// otherwise be misparsed as color tags.
func newLogoPanel(cfg config.Config) *tview.TextView {
	return tview.NewTextView().
		SetTextAlign(tview.AlignRight).
		SetText(strings.Join(cfg.Logo, "\n"))
}

// logoWidth returns the display width (in terminal cells) of the widest line
// in logo, so the top bar's right column doesn't clip a custom logo.
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
