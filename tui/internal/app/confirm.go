package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// confirmDialog is the Yes/No confirmation overlay used before destructive
// actions (delete, purge, ...) across every view. "No" is item 0 (default
// focus) to prevent accidental actions.
type confirmDialog struct {
	app     *App
	flex    *tview.Flex
	text    *tview.TextView
	list    *tview.List
	visible bool
}

// newConfirmDialog builds the confirm overlay's widgets. Shared across
// every caller of show — its content is rebuilt each time.
func newConfirmDialog(a *App) *confirmDialog {
	c := &confirmDialog{app: a}
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

	c.app.rootPages.ShowPage("confirm")
	c.app.tv.SetFocus(c.list)
	c.visible = true
}

// close hides the confirmation dialog and restores focus.
func (c *confirmDialog) close() {
	c.app.rootPages.HidePage("confirm")
	c.app.tv.SetFocus(c.app.pages)
	c.visible = false
}
