package dialog

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SendMessageOverlay is the "Send Message" overlay: JMS Type, Correlation
// ID, Group ID, custom Headers, and Body fields plus Submit/Cancel/Load
// snippet… actions for composing a new message on a queue.
type SendMessageOverlay struct {
	host              ui.Host
	snippetPicker     *SnippetPicker
	confirm           *ConfirmDialog
	form              *tview.Form
	jmsTypeItem       *tview.InputField
	correlationIDItem *tview.InputField
	groupIDItem       *tview.InputField
	headersItem       *tview.TextArea
	bodyItem          *tview.TextArea
	onClose           func()
	visible           bool
}

// NewSendMessageOverlay builds the send-message overlay's widgets.
// snippetPicker backs the "Load snippet…" button; confirm asks before a
// loaded snippet replaces fields the user already filled in.
func NewSendMessageOverlay(host ui.Host, snippetPicker *SnippetPicker, confirm *ConfirmDialog) *SendMessageOverlay {
	sm := &SendMessageOverlay{host: host, snippetPicker: snippetPicker, confirm: confirm}
	sm.form = tview.NewForm()
	sm.form.SetBorder(true).SetTitle(" Send Message ")
	sm.form.
		AddInputField("JMS Type", "", 40, nil, nil).
		AddInputField("Correlation ID", "", 40, nil, nil).
		AddInputField("Group ID", "", 40, nil, nil).
		AddTextArea("Headers (key: value per line)", "", 40, 3, 0, nil).
		AddTextArea("Body", "", 40, 6, 0, nil).
		AddButton("Submit", nil).
		AddButton("Cancel", sm.close).
		AddButton("Load snippet…", sm.openSnippetPicker)
	sm.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			sm.close()
			return nil
		}
		return event
	})

	sm.jmsTypeItem = sm.form.GetFormItem(0).(*tview.InputField)
	sm.correlationIDItem = sm.form.GetFormItem(1).(*tview.InputField)
	sm.groupIDItem = sm.form.GetFormItem(2).(*tview.InputField)
	sm.headersItem = sm.form.GetFormItem(3).(*tview.TextArea)
	sm.bodyItem = sm.form.GetFormItem(4).(*tview.TextArea)

	return sm
}

// Show opens the send-message overlay for the given queue. onClose is
// called on the UI goroutine when the overlay is dismissed.
func (sm *SendMessageOverlay) Show(queueName string, onClose func()) {
	host := sm.host
	sm.onClose = onClose
	sm.jmsTypeItem.SetText("")
	sm.correlationIDItem.SetText(newCorrelationID())
	sm.groupIDItem.SetText("")
	sm.headersItem.SetText("", false)
	sm.bodyItem.SetText("", true)
	sm.form.SetTitle(fmt.Sprintf(" Send Message — %s ", queueName))

	// Wire Submit with the correct closure each time (Cancel is the same
	// sm.close on every open, wired once in NewSendMessageOverlay).
	sm.form.GetButton(0).SetSelectedFunc(func() { sm.doSend(queueName) })

	sm.form.SetFocus(0)
	host.ShowPage("send-message")
	sm.visible = true
	sm.focusForm()
}

// focusForm gives focus and the context hint back to the form — on open,
// and whenever an overlay raised from it (snippet picker, confirmation)
// is dismissed.
func (sm *SendMessageOverlay) focusForm() {
	sm.host.SetFocus(sm.form)
	ac := sm.host.Config().Colors.Accent
	sm.host.SetContextHint(fmt.Sprintf("[%s]<Tab>[-] next field  [%s]<Esc>[-] cancel", ac, ac))
}

// openSnippetPicker opens the snippet picker on top of the form; the
// picked snippet goes through applySnippet.
func (sm *SendMessageOverlay) openSnippetPicker() {
	sm.snippetPicker.Show(sm.applySnippet, sm.focusForm)
}

// applySnippet loads s into the form. When JMS Type or Body already hold
// something, a confirmation asks first; "No" leaves the form unchanged.
func (sm *SendMessageOverlay) applySnippet(s snippet.Snippet) {
	if sm.jmsTypeItem.GetText() == "" && sm.bodyItem.GetText() == "" {
		sm.setFromSnippet(s)
		return
	}
	sm.confirm.ShowWithCancel("Replace JMS Type and Body with the snippet?",
		func() {
			sm.setFromSnippet(s)
			sm.focusForm()
		},
		sm.focusForm)
}

// setFromSnippet writes s's JMS Type (clearing the field when s has none)
// and Body into the form. Correlation ID, Group ID, and Headers are never
// touched — a snippet only carries those two.
func (sm *SendMessageOverlay) setFromSnippet(s snippet.Snippet) {
	sm.jmsTypeItem.SetText(s.JMSType)
	sm.bodyItem.SetText(s.Body, false)
}

// doSend validates the form's fields, closes the overlay, and sends the
// message asynchronously, reporting the result in the status bar. On a
// validation error, the status bar reports it and the form stays open for
// correction, mirroring MessageFilter.apply's pattern.
func (sm *SendMessageOverlay) doSend(queueName string) {
	host := sm.host
	req, err := buildSendMessageRequest(
		sm.jmsTypeItem.GetText(),
		sm.correlationIDItem.GetText(),
		sm.groupIDItem.GetText(),
		sm.headersItem.GetText(),
		sm.bodyItem.GetText(),
		newCorrelationID,
	)
	if err != nil {
		host.SetStatus(fmt.Sprintf("[red]%s[-]", err))
		return
	}
	sm.close()
	go func() {
		err := host.Backend().SendMessage(context.Background(), queueName, req)
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

// buildSendMessageRequest validates and assembles a SendMessageRequest from
// the overlay's raw field values — split out from doSend so it's directly
// testable without driving tview, the same pattern this codebase already
// uses for QueuesView.doPurge/doMoveAll and MessageFilter.handleScanResult.
// A blank jmsType or body is rejected; a blank correlationID falls back to
// newID() rather than being sent blank (spec-wip/fe-send-message-metadata).
func buildSendMessageRequest(jmsType, correlationID, groupID, headersText, body string, newID func() string) (queue.SendMessageRequest, error) {
	jmsType = strings.TrimSpace(jmsType)
	if jmsType == "" {
		return queue.SendMessageRequest{}, errors.New("JMS Type is required")
	}
	if strings.TrimSpace(body) == "" {
		return queue.SendMessageRequest{}, errors.New("Body is required")
	}
	headers, err := parseHeaders(headersText)
	if err != nil {
		return queue.SendMessageRequest{}, err
	}
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		correlationID = newID()
	}
	return queue.SendMessageRequest{
		JMSType:       jmsType,
		Body:          body,
		CorrelationID: correlationID,
		GroupID:       strings.TrimSpace(groupID),
		Headers:       headers,
	}, nil
}

// parseHeaders parses the Headers field's "key: value" per line text into a
// map. Blank lines are skipped; a line with no ':' separator, or an empty
// key, is a validation error. Returns a nil map (not an error) when text
// has no non-blank lines — Headers is optional.
func parseHeaders(text string) (map[string]string, error) {
	headers := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid header line %q (expected \"key: value\")", line)
		}
		headers[key] = strings.TrimSpace(value)
	}
	if len(headers) == 0 {
		return nil, nil
	}
	return headers, nil
}

// newCorrelationID returns a random RFC 4122 v4 UUID string, used to
// pre-fill the Correlation ID field and as doSend's fallback when it's left
// blank at submit time.
func newCorrelationID() string {
	var b [16]byte
	// crypto/rand.Read only fails if the OS CSPRNG is unavailable, an
	// unrecoverable environment failure a fallback ID wouldn't meaningfully
	// handle better — deliberately not treated as an error case here.
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
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
	sm.form.SetBackgroundColor(bg)
	sm.form.SetBorderColor(tcell.GetColor(p.Border))
	sm.form.SetTitleColor(tcell.GetColor(p.Border))
	for _, area := range []*tview.TextArea{sm.headersItem, sm.bodyItem} {
		area.SetBackgroundColor(bg)
		area.SetTextStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Text)).Background(bg))
		area.SetLabelStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Label)))
	}
}

var _ ui.Themeable = (*SendMessageOverlay)(nil)

// Primitive returns SendMessageOverlay's root widget, for sizing/embedding.
func (sm *SendMessageOverlay) Primitive() tview.Primitive { return sm.form }

// Visible reports whether SendMessageOverlay is currently shown.
func (sm *SendMessageOverlay) Visible() bool { return sm.visible }
