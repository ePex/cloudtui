package app

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

const (
	browseLimit       = 100
	actionModalWidth  = 50
	actionModalHeight = 7
)

// queuesView is the Queues screen: a list of queues with a browse/detail
// pane and send/purge/move actions. Like settings, it needs live backend
// access, so it lives here instead of internal/ui/views.
type queuesView struct {
	root *tview.Pages
	app  *App
}

func (v *queuesView) Name() string               { return "queues" }
func (v *queuesView) Title() string              { return "Queues" }
func (v *queuesView) Primitive() tview.Primitive { return v.root }

// activate reloads the queue list — called whenever this view becomes
// the active one (see switchTo), since the list otherwise only loads
// once at startup and would go stale.
func (v *queuesView) activate() { v.app.loadQueues() }

// newQueuesView builds the Queues view (a "list"/"detail" Pages) wired to
// backend, and wires the App fields the view's action handlers need.
func newQueuesView(a *App, backend queue.Backend) ui.View {
	a.backend = backend

	a.queuesList = styleList(tview.NewList().ShowSecondaryText(true), a.cfg.Colors)
	a.queuesList.SetBorder(true).SetTitle(" Queues ")
	a.queuesList.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		a.showQueueDetail(mainText)
	})

	a.messagesList = styleList(tview.NewList().ShowSecondaryText(true), a.cfg.Colors)
	a.messagesList.SetBorder(true)
	a.messagesList.SetInputCapture(a.onMessagesKey)

	accent := a.cfg.Colors.Accent
	hint := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[%s]a[-] send  [%s]d[-] purge  [%s]v[-] move  [%s]esc[-] back",
			accent, accent, accent, accent))

	detail := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.messagesList, 0, 1, true).
		AddItem(hint, 1, 0, false)

	a.queuesRoot = tview.NewPages().
		AddPage("list", a.queuesList, true, true).
		AddPage("detail", detail, true, false)

	return &queuesView{root: a.queuesRoot, app: a}
}

// loadQueues fetches the queue list in the background and applies it to
// queuesList without blocking the render loop.
func (a *App) loadQueues() {
	a.setStatus("Loading queues…")
	go func() {
		summaries, err := a.backend.List(context.Background())
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.setStatus(fmt.Sprintf("error loading queues: %v", err))
				return
			}
			a.queuesList.Clear()
			for _, s := range summaries {
				a.queuesList.AddItem(s.Name,
					fmt.Sprintf("pending: %d  consumers: %d", s.PendingCount, s.ConsumerCount), 0, nil)
			}
			a.setStatus(a.readyText())
		})
	}()
}

// showQueueDetail switches to the detail pane for queueName and loads its
// messages.
func (a *App) showQueueDetail(queueName string) {
	a.currentQueueName = queueName
	a.queuesRoot.SwitchToPage("detail")
	a.tv.SetFocus(a.messagesList)
	a.loadMessages(queueName)
}

// loadMessages fetches queueName's messages in the background and applies
// them to messagesList without blocking the render loop.
func (a *App) loadMessages(queueName string) {
	a.setStatus("Loading messages…")
	go func() {
		messages, err := a.backend.Browse(context.Background(), queueName, browseLimit)
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.setStatus(fmt.Sprintf("error loading messages: %v", err))
				return
			}
			a.messagesList.Clear()
			for _, m := range messages {
				a.messagesList.AddItem(m.Body, m.ID, 0, nil)
			}
			a.messagesList.SetTitle(fmt.Sprintf(" %s ", queueName))
			a.setStatus(a.readyText())
		})
	}()
}

// onMessagesKey handles the detail pane's actions. It's installed via
// SetInputCapture on messagesList (not a global hotkey) since 's' is
// already claimed globally for the Settings view.
func (a *App) onMessagesKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEscape {
		a.queuesRoot.SwitchToPage("list")
		a.tv.SetFocus(a.queuesList)
		return nil
	}
	switch event.Rune() {
	case 'a':
		a.openSendModal()
		return nil
	case 'd':
		a.openPurgeConfirm()
		return nil
	case 'v':
		a.openMoveModal()
		return nil
	}
	return event
}

// openSendModal shows a small form for the message body, sending on submit.
func (a *App) openSendModal() {
	form := tview.NewForm()
	form.AddInputField("Message body", "", 40, nil, nil)
	form.AddButton("Send", func() {
		body := form.GetFormItem(0).(*tview.InputField).GetText()
		queueName := a.currentQueueName
		a.closeActionModal()
		a.sendMessage(queueName, body)
	})
	form.AddButton("Cancel", a.closeActionModal)
	form.SetBorder(true).SetTitle(" Send Message ")

	a.showActionModal(form)
}

func (a *App) sendMessage(queueName, body string) {
	a.setStatus("Sending…")
	go func() {
		err := a.backend.Send(context.Background(), queueName, body, nil)
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.setStatus(fmt.Sprintf("error sending: %v", err))
				return
			}
			a.setStatus(a.readyText())
			a.loadMessages(queueName)
		})
	}()
}

// openPurgeConfirm shows a yes/no confirmation, purging on confirm.
func (a *App) openPurgeConfirm() {
	queueName := a.currentQueueName
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Purge all messages on %q?", queueName)).
		AddButtons([]string{"Purge", "Cancel"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		a.closeActionModal()
		if buttonLabel == "Purge" {
			a.purgeQueue(queueName)
		}
	})

	a.showActionModal(modal)
}

func (a *App) purgeQueue(queueName string) {
	a.setStatus("Purging…")
	go func() {
		err := a.backend.Purge(context.Background(), queueName)
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.setStatus(fmt.Sprintf("error purging: %v", err))
				return
			}
			a.setStatus(a.readyText())
			a.loadMessages(queueName)
			a.loadQueues()
		})
	}()
}

// openMoveModal shows a small form for the target queue name, moving on submit.
func (a *App) openMoveModal() {
	form := tview.NewForm()
	form.AddInputField("Target queue", "", 40, nil, nil)
	form.AddButton("Move", func() {
		target := form.GetFormItem(0).(*tview.InputField).GetText()
		sourceQueueName := a.currentQueueName
		a.closeActionModal()
		a.moveMessages(sourceQueueName, target)
	})
	form.AddButton("Cancel", a.closeActionModal)
	form.SetBorder(true).SetTitle(" Move Messages ")

	a.showActionModal(form)
}

func (a *App) moveMessages(sourceQueueName, targetQueueName string) {
	a.setStatus("Moving…")
	go func() {
		err := a.backend.Move(context.Background(), sourceQueueName, targetQueueName, nil)
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				a.setStatus(fmt.Sprintf("error moving: %v", err))
				return
			}
			a.setStatus(a.readyText())
			a.loadMessages(sourceQueueName)
			a.loadQueues()
		})
	}()
}

// showActionModal displays prim (a send/move form, or a purge confirm
// modal) as a centered overlay on rootPages.
func (a *App) showActionModal(prim tview.Primitive) {
	a.rootPages.AddPage("action", centered(prim, actionModalWidth, actionModalHeight), true, false)
	a.rootPages.ShowPage("action")
	a.tv.SetFocus(prim)
}

// closeActionModal hides and discards the action overlay, returning focus
// to the messages list.
func (a *App) closeActionModal() {
	a.rootPages.HidePage("action")
	a.rootPages.RemovePage("action")
	a.tv.SetFocus(a.messagesList)
}
