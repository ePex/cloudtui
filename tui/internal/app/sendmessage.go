package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// sendMessageOverlay is the "Send Message" overlay: a text area plus
// Submit/Cancel actions for composing a new message on a queue.
type sendMessageOverlay struct {
	app     *App
	flex    *tview.Flex
	area    *tview.TextArea
	list    *tview.List
	onClose func()
	visible bool
}

// newSendMessageOverlay builds the send-message overlay's widgets.
func newSendMessageOverlay(a *App) *sendMessageOverlay {
	sm := &sendMessageOverlay{app: a}
	sm.area = tview.NewTextArea()
	sm.list = tview.NewList().ShowSecondaryText(false)
	sm.list.AddItem("Submit", "", 0, nil)
	sm.list.AddItem("Cancel", "", 0, nil)
	sm.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(sm.area, 0, 1, true).
		AddItem(sm.list, 2, 0, false)
	sm.flex.SetBorder(true).SetTitle(" Send Message ")

	sm.area.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			a.tv.SetFocus(sm.list)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			sm.close()
			return nil
		}
		return event
	})
	sm.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Key() == tcell.KeyEscape:
			a.tv.SetFocus(sm.area)
			return nil
		}
		return event
	})
	return sm
}

// show opens the send-message overlay for the given queue. onClose is
// called on the UI goroutine when the overlay is dismissed.
func (sm *sendMessageOverlay) show(queueName string, onClose func()) {
	a := sm.app
	sm.onClose = onClose
	sm.area.SetText("", true)
	sm.flex.SetTitle(fmt.Sprintf(" Send Message — %s ", queueName))

	// Wire Submit and Cancel with the correct closure each time.
	sm.list.Clear()
	sm.list.AddItem("Submit", "", 0, func() { sm.doSend(queueName) })
	sm.list.AddItem("Cancel", "", 0, sm.close)

	a.rootPages.ShowPage("send-message")
	a.tv.SetFocus(sm.area)
	sm.visible = true
	ac := a.cfg.Colors.Accent
	a.contextPanel.SetText(fmt.Sprintf("[%s]<Tab>[-] actions  [%s]<Esc>[-] cancel", ac, ac))
}

// doSend reads the body from area, closes the overlay, and sends the
// message asynchronously, reporting the result in the status bar.
func (sm *sendMessageOverlay) doSend(queueName string) {
	a := sm.app
	body := sm.area.GetText()
	sm.close()
	go func() {
		err := a.backend.SendMessage(context.Background(), queueName, body)
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("send message: failed", "queue", queueName, "error", err)
				a.statusBar.SetText(fmt.Sprintf("[red]Error: %s[-]", err))
				return
			}
			a.statusBar.SetText(fmt.Sprintf("Message sent to %q", queueName))
			a.ReloadAfterSend(queueName)
		})
	}()
}

// close hides the send-message overlay and calls onClose to let the
// caller restore focus and the context panel.
func (sm *sendMessageOverlay) close() {
	sm.app.rootPages.HidePage("send-message")
	sm.visible = false
	if sm.onClose != nil {
		sm.onClose()
	}
}

// ApplyPalette recolors the send-message overlay for a live theme switch.
func (sm *sendMessageOverlay) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	sm.flex.SetBackgroundColor(bg)
	sm.flex.SetBorderColor(tcell.GetColor(p.Border))
	sm.flex.SetTitleColor(tcell.GetColor(p.Border))
	sm.area.SetBackgroundColor(bg)
	sm.area.SetTextStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Text)).Background(tcell.GetColor(p.Background)))
	sm.area.SetLabelStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Label)))
	styleList(sm.list, p)
	sm.list.SetBackgroundColor(bg)
}

var _ ui.Themeable = (*sendMessageOverlay)(nil)
