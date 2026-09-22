package view

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// MessageDetailView shows the full details of a single message.
// It is not a registered ui.View; it is opened via App.OpenMessageDetail
// and returns to "messages" on Esc/Backspace.
type MessageDetailView struct {
	textView    *tview.TextView
	host        ui.Host
	movePicker  *dialog.MovePicker
	confirm     *dialog.ConfirmDialog
	snippetSave *dialog.SnippetSaveDialog
	onBack      func()
	onReload    func()
	queueName   string
	msg         queue.Message
}

var _ ui.Themeable = (*MessageDetailView)(nil)

// ApplyPalette recolors the message detail view for a live theme switch.
func (dv *MessageDetailView) ApplyPalette(p config.Palette) {
	dv.textView.SetBackgroundColor(tcell.GetColor(p.Background))
	dv.textView.SetBorderColor(tcell.GetColor(p.ViewColor("queues")))
	dv.textView.SetTitleColor(tcell.GetColor(p.ViewColor("queues")))
	// Untagged text (the spaces between color tags, or plain log lines)
	// is drawn in the text view's base color, copied at construction.
	dv.textView.SetTextColor(tcell.GetColor(p.Text))
}

func (dv *MessageDetailView) Primitive() tview.Primitive { return dv.textView }

func (dv *MessageDetailView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "m", Description: "move"},
		{Key: "d", Description: "delete"},
		{Key: "S", Description: "save as snippet"},
		{Key: "Esc", Description: "back"},
	}
}

func NewMessageDetailView(a ui.Host, movePicker *dialog.MovePicker, confirm *dialog.ConfirmDialog, snippetSave *dialog.SnippetSaveDialog, onBack func(), onReload func()) *MessageDetailView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Message Details ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)

	dv := &MessageDetailView{textView: tv, host: a, movePicker: movePicker, confirm: confirm, snippetSave: snippetSave, onBack: onBack, onReload: onReload}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == 'm':
			srcQueue := dv.queueName
			msgID := dv.msg.ID
			dv.movePicker.Show(srcQueue, func(target string) {
				err := dv.host.Backend().MoveMessage(context.Background(), srcQueue, msgID, target)
				dv.host.QueueUpdateDraw(func() {
					if err != nil {
						slog.Error("move: failed", "src", srcQueue, "dst", target, "id", msgID, "error", err)
						dv.host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", err))
						return
					}
					dv.onBack()
					dv.onReload()
				})
			}, dv.restoreFocus)
			return nil
		case event.Rune() == 'S':
			sn, err := snippetFromMessage(dv.msg)
			if err != nil {
				dv.host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", err))
				return nil
			}
			dv.snippetSave.Show(sn, dv.restoreFocus)
			return nil
		case event.Rune() == 'd':
			queueName := dv.queueName
			msgID := dv.msg.ID
			dv.confirm.Show(fmt.Sprintf("Delete message from %q?", queueName), func() {
				go func() {
					err := dv.host.Backend().RemoveMessage(context.Background(), queueName, msgID)
					dv.host.QueueUpdateDraw(func() {
						if err != nil {
							slog.Error("message detail: remove failed", "queue", queueName, "id", msgID, "error", err)
							dv.host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", err))
							return
						}
						dv.onBack()
						dv.onReload()
					})
				}()
			})
			return nil
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			dv.onBack()
			return nil
		}
		return event
	})

	return dv
}

// restoreFocus gives focus and the shortcut hint back to the detail view
// after an overlay raised from it (move picker, snippet save) closes.
func (dv *MessageDetailView) restoreFocus() {
	dv.host.SetFocus(dv.textView)
	lines := make([]string, 0, len(dv.Shortcuts()))
	for _, sc := range dv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", dv.host.Config().Colors.Accent, sc.Key, sc.Description))
	}
	dv.host.SetContextHint(strings.Join(lines, "\n"))
}

// snippetFromMessage builds the snippet saved by 'S': the raw (not
// pretty-printed) text body, plus the JMS Type only when it's the
// message's real JMSType header — an inferred "text"/"bytes"/"other" is
// dropped. No other header is kept. A message without a text body (e.g.
// binary) can't be saved.
func snippetFromMessage(msg queue.Message) (snippet.Snippet, error) {
	body, _ := msg.RawFields["text"].(string)
	if body == "" {
		return snippet.Snippet{}, errors.New("message has no text body to save as a snippet")
	}
	sn := snippet.Snippet{Body: body}
	if !msg.JMSTypeInferred {
		sn.JMSType = msg.JMSType
	}
	return sn, nil
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
// a map with a "data" key holding a []any of float64 byte values.
// Those are decoded to a UTF-8 string. All other values fall back to %v.
func decodePropertyValue(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	dataRaw, hasData := m["data"]
	if !hasData {
		return fmt.Sprintf("%v", v)
	}
	items, ok := dataRaw.([]any)
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

// Render builds and displays the detail text for msg in the context of queueName.
func (dv *MessageDetailView) Render(queueName string, msg queue.Message) {
	dv.queueName = queueName
	dv.msg = msg
	dv.textView.SetTitle(fmt.Sprintf(" Message Details — %s ", queueName))
	p := dv.host.Config().Colors
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
			if props, ok := raw.(map[string]any); ok {
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
