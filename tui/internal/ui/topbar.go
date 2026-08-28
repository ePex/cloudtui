package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// ShortcutPanelRows is the maximum number of shortcut lines any view shows
// — bump this whenever a view's Shortcuts() grows past it, or entries
// silently clip off the bottom of the context panel (the top bar's height
// is fixed, not scrollable). Currently messagesView, at 12.
const ShortcutPanelRows = 12

// TopBar is the app's top row: a "info"/"prompt" Pages on the left
// (connection info, replaced by the command prompt while active),
// a single-char divider, a context panel (view-specific shortcuts,
// empty by default), and the logo.
type TopBar struct {
	Root         *tview.Flex
	Left         *tview.Pages
	Info         *tview.TextView
	Divider      *tview.TextView
	ContextPanel *tview.TextView
	Logo         *tview.TextView
	Height       int
}

// NewTopBar builds the top bar. prompt is added as the left Pages' "prompt"
// page so the app can switch to it while a command is being typed.
func NewTopBar(cfg config.Config, prompt *tview.InputField) *TopBar {
	info := newInfoPanel(cfg)
	left := tview.NewPages().
		AddPage("info", info, true, true).
		AddPage("prompt", prompt, true, false)

	contextPanel := tview.NewTextView().SetDynamicColors(true)
	logoPanel := newLogoPanel(cfg)
	// Ensure the top bar is tall enough to display all view shortcuts.
	height := maxInt(ShortcutPanelRows, len(cfg.Logo))
	divider := newDivider(cfg, height)

	root := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(left, 0, 1, false).
		AddItem(divider, 1, 0, false).
		AddItem(contextPanel, 0, 1, false).
		AddItem(logoPanel, logoWidth(cfg.Logo), 0, false)

	return &TopBar{
		Root:         root,
		Left:         left,
		Info:         info,
		Divider:      divider,
		ContextPanel: contextPanel,
		Logo:         logoPanel,
		Height:       height,
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

// InfoPanelText renders the connection-info panel's content from cfg.
func InfoPanelText(cfg config.Config) string {
	awsProfile := cfg.ActiveAWSProfile
	if awsProfile == "" {
		awsProfile = "(none)"
	}
	conn := cfg.ActiveConn()
	connLine := conn.Name
	// A secret-backed connection's password profile is independent of
	// ActiveAWSProfile below (spec/12-named-connections) — surfaced here
	// too so it's clear at a glance which account a connection using AWS
	// Secret actually authenticates against, without having to open the
	// connection editor.
	if secretProfile := conn.SecretAWSProfile(); secretProfile != "" {
		connLine = fmt.Sprintf("%s [%s](AWS: %s)[-]", conn.Name, cfg.Colors.Label, secretProfile)
	}
	return fmt.Sprintf(
		"[%s]Theme:[-] [%s]%s[-]\n[%s]AMQ Connection:[-] [%s]%s[-]\n[%s]AWS Profile:[-] [%s]%s[-]",
		cfg.Colors.Label, cfg.Colors.Value, cfg.Theme,
		cfg.Colors.Label, cfg.Colors.Value, connLine,
		cfg.Colors.Label, cfg.Colors.Value, awsProfile,
	)
}

func newInfoPanel(cfg config.Config) *tview.TextView {
	return tview.NewTextView().
		SetDynamicColors(true).
		SetText(InfoPanelText(cfg))
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
