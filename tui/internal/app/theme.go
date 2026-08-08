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

	// Status bar — recolor only; don't touch its text. It's either blank
	// (idle) or showing a transient message, and there's no longer a
	// default legend to restore (see newStatusBar's doc comment).
	a.statusBar.SetBackgroundColor(tcell.GetColor(p.StatusBarBg))
	a.statusBar.SetTextColor(tcell.GetColor(p.StatusBarText))

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

	// Settings list
	if a.settingsList != nil {
		a.settingsList.SetBackgroundColor(bg)
		a.settingsList.SetBorderColor(tcell.GetColor(p.ViewColor("settings")))
		a.settingsList.SetTitleColor(tcell.GetColor(p.ViewColor("settings")))
		styleList(a.settingsList, p)
	}

	// Theme picker overlay
	if a.themePickerFlex != nil {
		a.themePickerFlex.SetBackgroundColor(bg)
		a.themePickerFlex.SetBorderColor(tcell.GetColor(p.Border))
		a.themePickerFlex.SetTitleColor(tcell.GetColor(p.Border))
	}
	if a.themePickerList != nil {
		styleList(a.themePickerList, p)
		a.themePickerList.SetBackgroundColor(bg)
	}

	// Connection manager overlay
	if a.connManagerFlex != nil {
		a.connManagerFlex.SetBackgroundColor(bg)
		a.connManagerFlex.SetBorderColor(tcell.GetColor(p.Border))
		a.connManagerFlex.SetTitleColor(tcell.GetColor(p.Border))
	}
	if a.connManagerList != nil {
		styleList(a.connManagerList, p)
		a.connManagerList.SetBackgroundColor(bg)
	}
	if a.connManagerHints != nil {
		a.connManagerHints.SetBackgroundColor(bg)
		a.connManagerHints.SetTextColor(tcell.GetColor(p.Text))
	}

	// Connection editor overlay
	if a.connEditorForm != nil {
		a.connEditorForm.SetBackgroundColor(bg)
		a.connEditorForm.SetBorderColor(tcell.GetColor(p.Border))
		a.connEditorForm.SetTitleColor(tcell.GetColor(p.Border))
		if dd, ok := a.connEditorForm.GetFormItem(2).(*tview.DropDown); ok {
			styleDropDown(dd, p)
		}
	}

	// AWS Profiles overlay
	if a.awsProfilesFlex != nil {
		a.awsProfilesFlex.SetBackgroundColor(bg)
		a.awsProfilesFlex.SetBorderColor(tcell.GetColor(p.Border))
		a.awsProfilesFlex.SetTitleColor(tcell.GetColor(p.Border))
	}
	if a.awsProfilesTable != nil {
		a.awsProfilesTable.SetBackgroundColor(bg)
		a.awsProfilesTable.SetBorderColor(tcell.GetColor(p.Border))
		a.awsProfilesTable.SetTitleColor(tcell.GetColor(p.Border))
	}
	if a.awsProfilesHints != nil {
		a.awsProfilesHints.SetBackgroundColor(bg)
		a.awsProfilesHints.SetTextColor(tcell.GetColor(p.Text))
	}

	// Log text view
	if a.logV != nil {
		a.logV.textView.SetBackgroundColor(bg)
		a.logV.textView.SetBorderColor(tcell.GetColor(p.ViewColor("log")))
		a.logV.textView.SetTitleColor(tcell.GetColor(p.ViewColor("log")))
	}

	// Queues table + filter input
	if a.queuesV != nil {
		a.queuesV.table.SetBackgroundColor(bg)
		a.queuesV.table.SetBorderColor(tcell.GetColor(p.ViewColor("queues")))
		a.queuesV.table.SetTitleColor(tcell.GetColor(p.ViewColor("queues")))
		a.queuesV.filterInput.SetLabelColor(tcell.GetColor(p.Label))
		a.queuesV.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
		a.queuesV.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
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

	// SSM Parameters table + filter input
	if a.ssmParamsV != nil {
		a.ssmParamsV.table.SetBackgroundColor(bg)
		a.ssmParamsV.table.SetBorderColor(tcell.GetColor(p.ViewColor("ssm-parameters")))
		a.ssmParamsV.table.SetTitleColor(tcell.GetColor(p.ViewColor("ssm-parameters")))
		a.ssmParamsV.filterInput.SetLabelColor(tcell.GetColor(p.Label))
		a.ssmParamsV.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
		a.ssmParamsV.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
	}

	// Parameter detail text view
	if a.paramDetailV != nil {
		a.paramDetailV.textView.SetBackgroundColor(bg)
		a.paramDetailV.textView.SetBorderColor(tcell.GetColor(p.ViewColor("ssm-parameters")))
		a.paramDetailV.textView.SetTitleColor(tcell.GetColor(p.ViewColor("ssm-parameters")))
	}

	// Secrets Manager table + filter input
	if a.secretsV != nil {
		a.secretsV.table.SetBackgroundColor(bg)
		a.secretsV.table.SetBorderColor(tcell.GetColor(p.ViewColor("secrets-manager")))
		a.secretsV.table.SetTitleColor(tcell.GetColor(p.ViewColor("secrets-manager")))
		a.secretsV.filterInput.SetLabelColor(tcell.GetColor(p.Label))
		a.secretsV.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
		a.secretsV.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
	}

	// Secret detail text view
	if a.secretDetailV != nil {
		a.secretDetailV.textView.SetBackgroundColor(bg)
		a.secretDetailV.textView.SetBorderColor(tcell.GetColor(p.ViewColor("secrets-manager")))
		a.secretDetailV.textView.SetTitleColor(tcell.GetColor(p.ViewColor("secrets-manager")))
	}

	// Move-picker
	if a.movePickerFlex != nil {
		a.movePickerFlex.SetBackgroundColor(bg)
		a.movePickerFlex.SetBorderColor(tcell.GetColor(p.Border))
		a.movePickerFlex.SetTitleColor(tcell.GetColor(p.Border))
	}
	if a.movePickerList != nil {
		styleList(a.movePickerList, p)
		a.movePickerList.SetBackgroundColor(bg)
	}
	if a.movePickerSearch != nil {
		a.movePickerSearch.SetBackgroundColor(bg)
		a.movePickerSearch.SetLabelColor(tcell.GetColor(p.Label))
		a.movePickerSearch.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
		a.movePickerSearch.SetFieldTextColor(tcell.GetColor(p.SelectionText))
	}

	// Confirm dialog
	if a.confirmFlex != nil {
		a.confirmFlex.SetBackgroundColor(bg)
		a.confirmFlex.SetBorderColor(tcell.GetColor(p.Border))
		a.confirmFlex.SetTitleColor(tcell.GetColor(p.Border))
	}
	if a.confirmText != nil {
		a.confirmText.SetBackgroundColor(bg)
		a.confirmText.SetTextColor(tcell.GetColor(p.Text))
	}
	if a.confirmList != nil {
		styleList(a.confirmList, p)
		a.confirmList.SetBackgroundColor(bg)
	}

	// Send-message overlay
	if a.sendMessageFlex != nil {
		a.sendMessageFlex.SetBackgroundColor(bg)
		a.sendMessageFlex.SetBorderColor(tcell.GetColor(p.Border))
		a.sendMessageFlex.SetTitleColor(tcell.GetColor(p.Border))
	}
	if a.sendMessageArea != nil {
		a.sendMessageArea.SetBackgroundColor(bg)
		a.sendMessageArea.SetTextStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Text)).Background(tcell.GetColor(p.Background)))
		a.sendMessageArea.SetLabelStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Label)))
	}
	if a.sendMessageList != nil {
		styleList(a.sendMessageList, p)
		a.sendMessageList.SetBackgroundColor(bg)
	}

}

// Note: no explicit a.tv.Draw() call — tview redraws automatically after
// every event handler returns when the event loop is running. An explicit
// Draw() here would block in test environments where no loop is started.
