package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// ConfirmDialog is the Yes/No confirmation overlay used before destructive
// actions (delete, purge, ...) across every view. "No" is item 0 (default
// focus) to prevent accidental actions.
type ConfirmDialog struct {
	host    ui.Host
	flex    *tview.Flex
	text    *tview.TextView
	list    *tview.List
	visible bool
}

// NewConfirmDialog builds the confirm overlay's widgets. Shared across
// every caller of Show — its content is rebuilt each time.
func NewConfirmDialog(host ui.Host) *ConfirmDialog {
	c := &ConfirmDialog{host: host}
	c.text = tview.NewTextView().SetWrap(true)
	c.list = tview.NewList().ShowSecondaryText(false)
	c.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(c.text, 2, 0, false).
		AddItem(c.list, 0, 1, true)
	c.flex.SetBorder(true).SetTitle(" Confirm ")
	return c
}

// Show presents a confirmation dialog with the given question. onConfirm
// is called when the user selects "Yes"; "No"/Esc returns focus to the
// main view.
func (c *ConfirmDialog) Show(question string, onConfirm func()) {
	c.ShowWithCancel(question, onConfirm, nil)
}

// ShowWithCancel is Show with a cancel callback: when onCancel is non-nil,
// "No"/Esc call it instead of returning focus to the main view — for a
// confirmation raised on top of another overlay that stays open and must
// get focus back.
func (c *ConfirmDialog) ShowWithCancel(question string, onConfirm, onCancel func()) {
	c.text.SetText(question)
	c.list.Clear()

	dismiss := func() {
		c.close(onCancel == nil)
		if onCancel != nil {
			onCancel()
		}
	}

	c.list.AddItem("No", "", 0, dismiss)
	c.list.AddItem("Yes", "", 0, func() {
		c.close(true)
		onConfirm()
	})

	c.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			dismiss()
			return nil
		}
		return event
	})

	c.host.ShowPage("confirm")
	c.host.SetFocus(c.list)
	c.visible = true
}

// close hides the confirmation dialog, restoring focus to the main view
// when focusMain is set (otherwise the caller's onCancel restores it).
func (c *ConfirmDialog) close(focusMain bool) {
	c.host.HidePage("confirm")
	if focusMain {
		c.host.FocusMain()
	}
	c.visible = false
}

// ApplyPalette recolors the confirm dialog for a live theme switch.
func (c *ConfirmDialog) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	c.flex.SetBackgroundColor(bg)
	c.flex.SetBorderColor(tcell.GetColor(p.Border))
	c.flex.SetTitleColor(tcell.GetColor(p.Border))
	c.text.SetBackgroundColor(bg)
	c.text.SetTextColor(tcell.GetColor(p.Text))
	ui.StyleList(c.list, p)
	c.list.SetBackgroundColor(bg)
}

var _ ui.Themeable = (*ConfirmDialog)(nil)

// Primitive returns ConfirmDialog's root widget, for sizing/embedding.
func (c *ConfirmDialog) Primitive() tview.Primitive { return c.flex }

// Visible reports whether ConfirmDialog is currently shown.
func (c *ConfirmDialog) Visible() bool { return c.visible }
