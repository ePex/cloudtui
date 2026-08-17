package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// confirmDialog is the Yes/No confirmation overlay used before destructive
// actions (delete, purge, ...) across every view. "No" is item 0 (default
// focus) to prevent accidental actions.
type confirmDialog struct {
	host    ui.Host
	flex    *tview.Flex
	text    *tview.TextView
	list    *tview.List
	visible bool
}

// newConfirmDialog builds the confirm overlay's widgets. Shared across
// every caller of show — its content is rebuilt each time.
func newConfirmDialog(host ui.Host) *confirmDialog {
	c := &confirmDialog{host: host}
	c.text = tview.NewTextView().SetWrap(true)
	c.list = tview.NewList().ShowSecondaryText(false)
	c.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(c.text, 2, 0, false).
		AddItem(c.list, 0, 1, true)
	c.flex.SetBorder(true).SetTitle(" Confirm ")
	return c
}

// show presents a confirmation dialog with the given question. onConfirm
// is called when the user selects "Yes".
func (c *confirmDialog) show(question string, onConfirm func()) {
	c.text.SetText(question)
	c.list.Clear()

	dismiss := func() { c.close() }

	c.list.AddItem("No", "", 0, dismiss)
	c.list.AddItem("Yes", "", 0, func() {
		c.close()
		onConfirm()
	})

	c.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			c.close()
			return nil
		}
		return event
	})

	c.host.ShowPage("confirm")
	c.host.SetFocus(c.list)
	c.visible = true
}

// close hides the confirmation dialog and restores focus.
func (c *confirmDialog) close() {
	c.host.HidePage("confirm")
	c.host.FocusMain()
	c.visible = false
}

// ApplyPalette recolors the confirm dialog for a live theme switch.
func (c *confirmDialog) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	c.flex.SetBackgroundColor(bg)
	c.flex.SetBorderColor(tcell.GetColor(p.Border))
	c.flex.SetTitleColor(tcell.GetColor(p.Border))
	c.text.SetBackgroundColor(bg)
	c.text.SetTextColor(tcell.GetColor(p.Text))
	styleList(c.list, p)
	c.list.SetBackgroundColor(bg)
}

var _ ui.Themeable = (*confirmDialog)(nil)
