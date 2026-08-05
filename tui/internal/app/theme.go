package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui/views"
)

// applyTheme sets tview's package-level default styles from p. tview
// primitives (Box, List, TextView, Form, ...) read tview.Styles once, at
// construction time, not on every draw — so this must run before any
// primitive is constructed (see App.New(), which calls this first).
func applyTheme(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	tview.Styles.PrimitiveBackgroundColor = bg
	tview.Styles.ContrastBackgroundColor = bg
	tview.Styles.MoreContrastBackgroundColor = bg
	tview.Styles.BorderColor = tcell.GetColor(p.Border)
	tview.Styles.TitleColor = tcell.GetColor(p.Border)
	tview.Styles.GraphicsColor = tcell.GetColor(p.Border)
	tview.Styles.PrimaryTextColor = tcell.GetColor(p.Text)
	tview.Styles.SecondaryTextColor = tcell.GetColor(p.Value)
	tview.Styles.TertiaryTextColor = tcell.GetColor(p.Label)
	tview.Styles.InverseTextColor = tcell.GetColor(p.SelectionText)
	tview.Styles.ContrastSecondaryTextColor = tcell.GetColor(p.Value)
}

// styleList applies p's selection colors to l. tview.List's own computed
// default selection style inverts body text (background/text swapped), which
// doesn't produce the palette's highlight look — so selection is wired
// explicitly here rather than riding on applyTheme.
//
// Note: tview.List exposes no getter for its selected-item style, so the
// result cannot be unit-tested directly. Verified manually instead (see
// plan.md § Testing).
func styleList(l *tview.List, p config.Palette) *tview.List {
	return l.
		SetSelectedBackgroundColor(tcell.GetColor(p.SelectionBg)).
		SetSelectedTextColor(tcell.GetColor(p.SelectionText))
}

// reapplyTheme updates tview.Styles and all already-constructed shell
// primitives to reflect palette p, then calls tv.Draw() to repaint. This is
// the runtime theme-switch path; applyTheme handles startup.
func reapplyTheme(a *App, p config.Palette) {
	applyTheme(p)

	bg := tcell.GetColor(p.Background)

	// Status bar
	a.statusBar.SetBackgroundColor(tcell.GetColor(p.StatusBarBg))
	a.statusBar.SetTextColor(tcell.GetColor(p.StatusBarText))
	a.statusBar.SetText(readyStatusText(a.cfg))

	// Info panel — rebuildtext to show the new theme name
	a.infoPanel.SetBackgroundColor(bg)
	a.infoPanel.SetText(infoPanelText(a.cfg))

	// Divider — rebuild color-tagged text to pick up the new border color
	lines := make([]string, a.topBarHeight)
	for i := range lines {
		lines[i] = "│"
	}
	a.divider.SetText(fmt.Sprintf("[%s]%s[-]", p.Border, strings.Join(lines, "\n")))
	a.divider.SetBackgroundColor(bg)

	// Context panel — background only; text is managed by switchTo/updateContextPanel
	a.contextPanel.SetBackgroundColor(bg)
	// Re-render shortcuts with new accent color if a Shortcuttable view is active.
	if av := a.activeView(); av != nil {
		a.updateContextPanel(av)
	}

	// Logo panel
	a.logoPanel.SetBackgroundColor(bg)

	// Home table
	views.RepaintHomeTable(a.homeTable, a.homeSections, p.Label, p.Text, p.Border, p.SelectionBg, p.SelectionText)
	a.homeTable.SetBackgroundColor(bg)
	a.homeTable.SetBorderColor(tcell.GetColor(p.ViewColor("home")))
	a.homeTable.SetTitleColor(tcell.GetColor(p.ViewColor("home")))

	// Settings form
	if a.settingsForm != nil {
		a.settingsForm.SetBackgroundColor(bg)
		a.settingsForm.SetBorderColor(tcell.GetColor(p.ViewColor("settings")))
		a.settingsForm.SetTitleColor(tcell.GetColor(p.ViewColor("settings")))
	}

	// Log text view
	if a.logV != nil {
		a.logV.textView.SetBackgroundColor(bg)
		a.logV.textView.SetBorderColor(tcell.GetColor(p.ViewColor("log")))
		a.logV.textView.SetTitleColor(tcell.GetColor(p.ViewColor("log")))
	}

	// Queues table
	if a.queuesV != nil {
		a.queuesV.table.SetBackgroundColor(bg)
		a.queuesV.table.SetBorderColor(tcell.GetColor(p.ViewColor("queues")))
		a.queuesV.table.SetTitleColor(tcell.GetColor(p.ViewColor("queues")))
	}

	// Messages table
	if a.messagesV != nil {
		a.messagesV.table.SetBackgroundColor(bg)
		a.messagesV.table.SetBorderColor(tcell.GetColor(p.ViewColor("queues")))
		a.messagesV.table.SetTitleColor(tcell.GetColor(p.ViewColor("queues")))
	}

	// Message detail text view
	if a.messageDetailV != nil {
		a.messageDetailV.textView.SetBackgroundColor(bg)
		a.messageDetailV.textView.SetBorderColor(tcell.GetColor(p.ViewColor("queues")))
		a.messageDetailV.textView.SetTitleColor(tcell.GetColor(p.ViewColor("queues")))
	}

}
// Note: no explicit a.tv.Draw() call — tview redraws automatically after
// every event handler returns when the event loop is running. An explicit
// Draw() here would block in test environments where no loop is started.
