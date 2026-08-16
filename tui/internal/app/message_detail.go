package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// messageDetailView shows the full details of a single message.
// It is not a registered ui.View; it is opened via App.openMessageDetail
// and returns to "messages" on Esc/Backspace.
type messageDetailView struct {
	textView  *tview.TextView
	app       *App
	queueName string
	msg       queue.Message
}

func (dv *messageDetailView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "m", Description: "move"},
		{Key: "d", Description: "delete"},
		{Key: "Esc", Description: "back"},
	}
}

func newMessageDetailView(a *App) *messageDetailView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Message Details ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)

	dv := &messageDetailView{textView: tv, app: a}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == 'm':
			srcQueue := dv.queueName
			msgID := dv.msg.ID
			restoreDetail := func() {
				a.tv.SetFocus(a.pages)
				lines := make([]string, 0, len(dv.Shortcuts()))
				for _, sc := range dv.Shortcuts() {
					lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
				}
				a.contextPanel.SetText(strings.Join(lines, "\n"))
			}
			a.movePicker.show(srcQueue, func(target string) {
				err := a.backend.MoveMessage(context.Background(), srcQueue, msgID, target)
				a.tv.QueueUpdateDraw(func() {
					if err != nil {
						slog.Error("move: failed", "src", srcQueue, "dst", target, "id", msgID, "error", err)
						a.statusBar.SetText(fmt.Sprintf("[red]Error: %s[-]", err))
						return
					}
					a.pages.SwitchToPage("messages")
					a.tv.SetFocus(a.messagesV.table)
					lines := make([]string, 0, len(a.messagesV.Shortcuts()))
					for _, sc := range a.messagesV.Shortcuts() {
						lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
					}
					a.contextPanel.SetText(strings.Join(lines, "\n"))
					a.messagesV.load()
				})
			}, restoreDetail)
			return nil
		case event.Rune() == 'd':
			queueName := dv.queueName
			msgID := dv.msg.ID
			a.confirm.show(fmt.Sprintf("Delete message from %q?", queueName), func() {
				go func() {
					err := a.backend.RemoveMessage(context.Background(), queueName, msgID)
					a.tv.QueueUpdateDraw(func() {
						if err != nil {
							slog.Error("message detail: remove failed", "queue", queueName, "id", msgID, "error", err)
							a.statusBar.SetText(fmt.Sprintf("[red]Error: %s[-]", err))
							return
						}
						// Return to messages list and reload.
						a.pages.SwitchToPage("messages")
						a.tv.SetFocus(a.messagesV.table)
						lines := make([]string, 0, len(a.messagesV.Shortcuts()))
						for _, sc := range a.messagesV.Shortcuts() {
							lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
						}
						a.contextPanel.SetText(strings.Join(lines, "\n"))
						a.messagesV.load()
					})
				}()
			})
			return nil
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			a.pages.SwitchToPage("messages")
			a.tv.SetFocus(a.messagesV.table)
			lines := make([]string, 0, len(a.messagesV.Shortcuts()))
			for _, sc := range a.messagesV.Shortcuts() {
				lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
			}
			a.contextPanel.SetText(strings.Join(lines, "\n"))
			return nil
		}
		return event
	})

	return dv
}

// prettyJSON returns an indented version of s if s is valid JSON, otherwise "".
func prettyJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return ""
	}
	return buf.String()
}

// decodePropertyValue converts a Jolokia property value to a human-readable
// string. ActiveMQ serialises string properties as ByteSequence objects:
// a map with a "data" key holding a []interface{} of float64 byte values.
// Those are decoded to a UTF-8 string. All other values fall back to %v.
func decodePropertyValue(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	dataRaw, hasData := m["data"]
	if !hasData {
		return fmt.Sprintf("%v", v)
	}
	items, ok := dataRaw.([]interface{})
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	bs := make([]byte, len(items))
	for i, item := range items {
		f, ok := item.(float64)
		if !ok {
			return fmt.Sprintf("%v", v)
		}
		bs[i] = byte(f)
	}
	return string(bs)
}

// render builds and displays the detail text for msg in the context of queueName.
func (dv *messageDetailView) render(queueName string, msg queue.Message) {
	dv.queueName = queueName
	dv.msg = msg
	p := dv.app.cfg.Colors
	accent := p.Label
	text := p.Text

	var b strings.Builder

	// — Summary section —
	line := func(label, value string) {
		fmt.Fprintf(&b, "[%s]%s:[-] [%s]%s[-]\n", accent, label, text, tview.Escape(value))
	}
	line("Queue", queueName)
	line("ID", msg.ID)
	line("Type", msg.JMSType)
	line("Timestamp", msg.Timestamp.Local().Format("2006-01-02 15:04:05"))

	// — Headers section —
	fmt.Fprintf(&b, "\n[%s]Headers:[-]\n", accent)

	type headerField struct {
		label string
		key   string
	}
	fields := []headerField{
		{"JMSCorrelationID", "jMSCorrelationID"},
		{"JMSDeliveryMode", "jMSDeliveryMode"},
		{"JMSDestination", "jMSDestination"},
		{"JMSExpiration", "jMSExpiration"},
		{"JMSRedelivered", "jMSRedelivered"},
		{"JMSReplyTo", "jMSReplyTo"},
		{"JMSXGroupID", "groupID"},
		{"JMSXGroupSeq", "groupSequence"},
		{"JMSXUserID", "userID"},
		{"Priority", "jMSPriority"},
		{"PropertiesText", "properties"},
	}

	for _, f := range fields {
		if msg.RawFields == nil {
			fmt.Fprintf(&b, "[%s]%s:[-] [%s]<nil>[-]\n", accent, f.label, text)
			continue
		}
		raw := msg.RawFields[f.key]
		if f.key == "properties" {
			// properties is a map of property name → value (possibly ByteSequence).
			fmt.Fprintf(&b, "[%s]%s:[-]\n", accent, f.label)
			if props, ok := raw.(map[string]interface{}); ok {
				keys := make([]string, 0, len(props))
				for k := range props {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					decoded := decodePropertyValue(props[k])
					fmt.Fprintf(&b, "  [%s]%s:[-] [%s]%s[-]\n", accent, k, text, tview.Escape(decoded))
				}
			}
		} else {
			fmt.Fprintf(&b, "[%s]%s:[-] [%s]%s[-]\n", accent, f.label, text, tview.Escape(fmt.Sprintf("%v", raw)))
		}
	}

	// — Body section —
	fmt.Fprintf(&b, "\n[%s]Body:[-]\n", accent)
	var body string
	if msg.RawFields != nil {
		body, _ = msg.RawFields["text"].(string)
	}
	if body == "" {
		body = "(binary)"
	} else if pretty := prettyJSON(body); pretty != "" {
		body = pretty
	}
	fmt.Fprintf(&b, "[%s]%s[-]", text, tview.Escape(body))

	dv.textView.SetText(b.String())
	dv.textView.ScrollToBeginning()
}
