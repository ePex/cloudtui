package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
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

	// Status bar — recolor only; don't touch its text. It's either blank
	// (idle) or showing a transient message, and there's no longer a
	// default legend to restore (see newStatusBar's doc comment).
	a.statusBar.SetBackgroundColor(tcell.GetColor(p.StatusBarBg))
	a.statusBar.SetTextColor(tcell.GetColor(p.StatusBarText))

	// Info panel — rebuildtext to show the new theme name
	a.infoPanel.SetBackgroundColor(bg)
	a.infoPanel.SetText(ui.InfoPanelText(a.cfg))

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

	// Settings list
	if a.settingsList != nil {
		a.settingsList.SetBackgroundColor(bg)
		a.settingsList.SetBorderColor(tcell.GetColor(p.ViewColor("settings")))
		a.settingsList.SetTitleColor(tcell.GetColor(p.ViewColor("settings")))
		styleList(a.settingsList, p)
	}

	// Every other view/overlay recolors itself via ui.Themeable.
	for _, t := range a.themables {
		t.ApplyPalette(p)
	}
}

// Note: no explicit a.tv.Draw() call — tview redraws automatically after
// every event handler returns when the event loop is running. An explicit
// Draw() here would block in test environments where no loop is started.
