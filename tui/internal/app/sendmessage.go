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

// SendMessageOverlay is the "Send Message" overlay: a text area plus
// Submit/Cancel actions for composing a new message on a queue.
type SendMessageOverlay struct {
	host    ui.Host
	flex    *tview.Flex
	area    *tview.TextArea
	list    *tview.List
	onClose func()
	visible bool
}

// NewSendMessageOverlay builds the send-message overlay's widgets.
func NewSendMessageOverlay(host ui.Host) *SendMessageOverlay {
	sm := &SendMessageOverlay{host: host}
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
			host.SetFocus(sm.list)
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
			host.SetFocus(sm.area)
			return nil
		}
		return event
	})
	return sm
}

// Show opens the send-message overlay for the given queue. onClose is
// called on the UI goroutine when the overlay is dismissed.
func (sm *SendMessageOverlay) Show(queueName string, onClose func()) {
	host := sm.host
	sm.onClose = onClose
	sm.area.SetText("", true)
	sm.flex.SetTitle(fmt.Sprintf(" Send Message — %s ", queueName))

	// Wire Submit and Cancel with the correct closure each time.
	sm.list.Clear()
	sm.list.AddItem("Submit", "", 0, func() { sm.doSend(queueName) })
	sm.list.AddItem("Cancel", "", 0, sm.close)

	host.ShowPage("send-message")
	host.SetFocus(sm.area)
	sm.visible = true
	ac := host.Config().Colors.Accent
	host.SetContextHint(fmt.Sprintf("[%s]<Tab>[-] actions  [%s]<Esc>[-] cancel", ac, ac))
}

// doSend reads the body from area, closes the overlay, and sends the
// message asynchronously, reporting the result in the status bar.
func (sm *SendMessageOverlay) doSend(queueName string) {
	host := sm.host
	body := sm.area.GetText()
	sm.close()
	go func() {
		err := host.Backend().SendMessage(context.Background(), queueName, body)
		host.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("send message: failed", "queue", queueName, "error", err)
				host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", err))
				return
			}
			host.SetStatus(fmt.Sprintf("Message sent to %q", queueName))
			host.ReloadAfterSend(queueName)
		})
	}()
}

// close hides the send-message overlay and calls onClose to let the
// caller restore focus and the context panel.
func (sm *SendMessageOverlay) close() {
	sm.host.HidePage("send-message")
	sm.visible = false
	if sm.onClose != nil {
		sm.onClose()
	}
}

// ApplyPalette recolors the send-message overlay for a live theme switch.
func (sm *SendMessageOverlay) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	sm.flex.SetBackgroundColor(bg)
	sm.flex.SetBorderColor(tcell.GetColor(p.Border))
	sm.flex.SetTitleColor(tcell.GetColor(p.Border))
	sm.area.SetBackgroundColor(bg)
	sm.area.SetTextStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Text)).Background(tcell.GetColor(p.Background)))
	sm.area.SetLabelStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Label)))
	ui.StyleList(sm.list, p)
	sm.list.SetBackgroundColor(bg)
}

var _ ui.Themeable = (*SendMessageOverlay)(nil)

// Primitive returns SendMessageOverlay's root widget, for sizing/embedding.
func (sm *SendMessageOverlay) Primitive() tview.Primitive { return sm.flex }

// Visible reports whether SendMessageOverlay is currently shown.
func (sm *SendMessageOverlay) Visible() bool { return sm.visible }
