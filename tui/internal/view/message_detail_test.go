package view

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/snippet"
)

func newTestMessageDetailView(t *testing.T) (*fakeViewHost, *dialog.MovePicker, *dialog.ConfirmDialog, *MessageDetailView) {
	t.Helper()
	host := newFakeViewHost()
	movePicker := dialog.NewMovePicker(host)
	confirm := dialog.NewConfirmDialog(host)
	snippetSave := dialog.NewSnippetSaveDialog(host, snippet.NewStore(t.TempDir()), confirm)
	return host, movePicker, confirm, NewMessageDetailView(host, movePicker, confirm, snippetSave, func() {}, func() {})
}

func TestMessageDetailViewTitle(t *testing.T) {
	_, _, _, dv := newTestMessageDetailView(t)
	if got, want := dv.textView.GetTitle(), " Message Details "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestMessageDetailViewShortcutEscPresent(t *testing.T) {
	_, _, _, dv := newTestMessageDetailView(t)
	for _, s := range dv.Shortcuts() {
		if s.Key == "Esc" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"Esc\"")
}

func TestMessageDetailViewCopyShortcutPresent(t *testing.T) {
	_, _, _, dv := newTestMessageDetailView(t)
	for _, s := range dv.Shortcuts() {
		if s.Key == "c" && s.Description == "copy message" {
			return
		}
	}
	t.Error(`Shortcuts() missing {c, "copy message"}`)
}

func TestMessageClipboardTextIncludesQueueSummaryHeadersPropertiesAndFullBody(t *testing.T) {
	timestamp := time.Date(2026, 9, 22, 12, 34, 56, 0, time.Local)
	body := `{"orderId":42}`
	msg := queue.Message{
		ID:        "ID:broker:1:2",
		JMSType:   "OrderCreated",
		Timestamp: timestamp,
		Preview:   `{"orderId":`,
		RawFields: map[string]any{
			"text":             body,
			"jMSCorrelationID": "corr-123",
			"jMSDeliveryMode":  2,
			"jMSDestination":   "queue://orders",
			"jMSExpiration":    0,
			"jMSRedelivered":   false,
			"jMSReplyTo":       "queue://reply",
			"groupID":          "group-1",
			"groupSequence":    3,
			"userID":           "alice",
			"jMSPriority":      4,
			"properties": map[string]any{
				"zeta":  "last",
				"alpha": map[string]any{"data": []any{float64('A')}},
			},
		},
	}
	want := "Queue: orders\n" +
		"ID: ID:broker:1:2\n" +
		"Type: OrderCreated\n" +
		"Timestamp: 2026-09-22 12:34:56\n\n" +
		"Headers:\n" +
		"JMSCorrelationID: corr-123\n" +
		"JMSDeliveryMode: 2\n" +
		"JMSDestination: queue://orders\n" +
		"JMSExpiration: 0\n" +
		"JMSRedelivered: false\n" +
		"JMSReplyTo: queue://reply\n" +
		"JMSXGroupID: group-1\n" +
		"JMSXGroupSeq: 3\n" +
		"JMSXUserID: alice\n" +
		"Priority: 4\n" +
		"PropertiesText:\n" +
		"  alpha: A\n" +
		"  zeta: last\n\n" +
		"Body:\n{\n  \"orderId\": 42\n}"
	if got := messageClipboardText("orders", msg); got != want {
		t.Errorf("messageClipboardText() =\n%s\nwant\n%s", got, want)
	}
}

func TestMessageClipboardTextUsesBinaryPlaceholder(t *testing.T) {
	got := messageClipboardText("orders", queue.Message{})
	if !strings.HasSuffix(got, "Body:\n(binary)") {
		t.Errorf("binary message clipboard ends with %q, want Body: (binary)", got)
	}
	if !strings.Contains(got, "JMSCorrelationID: <nil>") {
		t.Errorf("missing nil header placeholders: %q", got)
	}
	if !strings.Contains(got, "PropertiesText: <nil>") {
		t.Errorf("missing nil PropertiesText placeholder: %q", got)
	}
}

func TestMessageDetailViewCopyWritesCompleteMessageAndStaysOpen(t *testing.T) {
	host, _, _, dv := newTestMessageDetailView(t)
	msg := queue.Message{
		ID:        "ID:test:1:1",
		JMSType:   "OrderCreated",
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local),
		Preview:   "short",
		RawFields: map[string]any{
			"text":             "the complete body",
			"jMSCorrelationID": "corr-1",
		},
	}
	dv.Render("orders", msg)
	detailBefore := dv.textView.GetText(false)

	dv.textView.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	for _, want := range []string{"Queue: orders", "JMSCorrelationID: corr-1", "the complete body"} {
		if !strings.Contains(host.copiedData, want) {
			t.Errorf("clipboard data %q does not contain %q", host.copiedData, want)
		}
	}
	if strings.Contains(host.copiedData, "short") {
		t.Errorf("clipboard contains preview instead of full body: %q", host.copiedData)
	}
	if host.status != "Copied message from orders to clipboard" {
		t.Errorf("status = %q, want copy confirmation", host.status)
	}
	if got := dv.textView.GetText(false); got != detailBefore {
		t.Errorf("copy changed the detail view text: got %q, want %q", got, detailBefore)
	}
}

func TestMessageDetailViewRenderNilRawFields(t *testing.T) {
	_, _, _, dv := newTestMessageDetailView(t)
	// Must not panic when RawFields is nil.
	dv.Render("test-queue", queue.Message{
		ID:        "ID:test:1:1",
		JMSType:   "text",
		Timestamp: time.Now(),
		Preview:   "hello",
		RawFields: nil,
	})
}

func TestMessageDetailViewRenderWithRawFields(t *testing.T) {
	_, _, _, dv := newTestMessageDetailView(t)
	dv.Render("test-queue", queue.Message{
		ID:        "ID:test:1:1",
		JMSType:   "MYTYPE",
		Timestamp: time.Now(),
		RawFields: map[string]interface{}{
			"text":             "hello world",
			"jMSCorrelationID": "corr-123",
			"jMSPriority":      float64(4),
		},
	})
	text := dv.textView.GetText(false)
	if text == "" {
		t.Error("rendered text is empty")
	}
	if got, want := dv.textView.GetTitle(), " Message Details — test-queue "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

// TestMessageDetailViewMoveOpensPickerWithSourceQueue and
// TestMessageDetailViewDeleteOpensConfirmWithPrompt only cover the
// synchronous half of the 'm'/'d' handlers (which dialog opens, with what
// prompt) — the success path itself runs inside a goroutine +
// QueueUpdateDraw, which (like every other goroutine+QueueUpdateDraw path
// in this app — see handleSearchResult's doc comment in logsearch.go)
// needs a running tview event loop to ever complete, so it isn't
// synchronously testable. That path is covered by live verification
// instead — see tasks.md.
func TestMessageDetailViewMoveOpensPickerWithSourceQueue(t *testing.T) {
	_, movePicker, _, dv := newTestMessageDetailView(t)
	dv.Render("orders", queue.Message{ID: "ID:test:1:1", Timestamp: time.Now()})

	capture := dv.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'm', tcell.ModNone))

	if !movePicker.Visible() {
		t.Error("'m' should open the move picker")
	}
}

func TestMessageDetailViewDeleteOpensConfirmWithPrompt(t *testing.T) {
	_, _, confirm, dv := newTestMessageDetailView(t)
	dv.Render("orders", queue.Message{ID: "ID:test:1:1", Timestamp: time.Now()})

	capture := dv.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))

	if !confirm.Visible() {
		t.Fatal("'d' should open the confirm dialog")
	}
	want := `Delete message from "orders"?`
	if got := renderedScreenText(t, confirm.Primitive(), 60, 8); !strings.Contains(got, want) {
		t.Errorf("rendered confirm dialog = %q, want it to contain %q", got, want)
	}
}

func TestSnippetFromMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     queue.Message
		want    snippet.Snippet
		wantErr bool
	}{
		{
			name: "header JMS type is kept",
			msg: queue.Message{
				JMSType:   "OrderCreated",
				RawFields: map[string]any{"text": `{"id":1}`, "jMSCorrelationID": "corr-1"},
			},
			want: snippet.Snippet{JMSType: "OrderCreated", Body: `{"id":1}`},
		},
		{
			name: "inferred JMS type is dropped",
			msg: queue.Message{
				JMSType:         "text",
				JMSTypeInferred: true,
				RawFields:       map[string]any{"text": "hello"},
			},
			want: snippet.Snippet{Body: "hello"},
		},
		{
			name: "body is raw, not pretty-printed",
			msg: queue.Message{
				JMSType:   "T",
				RawFields: map[string]any{"text": `{"a":1,"b":[1,2]}`},
			},
			want: snippet.Snippet{JMSType: "T", Body: `{"a":1,"b":[1,2]}`},
		},
		{
			name:    "empty body is an error",
			msg:     queue.Message{JMSType: "T", RawFields: map[string]any{"text": ""}},
			wantErr: true,
		},
		{
			name:    "binary (non-string) body is an error",
			msg:     queue.Message{JMSType: "bytes", JMSTypeInferred: true, RawFields: map[string]any{"text": nil}},
			wantErr: true,
		},
		{
			name:    "nil RawFields is an error",
			msg:     queue.Message{JMSType: "T"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := snippetFromMessage(tt.msg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("snippetFromMessage = %#v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("snippetFromMessage: unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("snippetFromMessage = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMessageDetailViewSaveSnippetOpensDialog(t *testing.T) {
	host, _, _, dv := newTestMessageDetailView(t)
	dv.Render("orders", queue.Message{
		ID:        "ID:test:1:1",
		JMSType:   "OrderCreated",
		Timestamp: time.Now(),
		RawFields: map[string]any{"text": "{}"},
	})

	dv.textView.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone))

	if !dv.snippetSave.Visible() {
		t.Fatal("'S' should open the save-as-snippet dialog")
	}
	if host.focused == dv.textView {
		t.Error("focus stayed on the detail view")
	}
}

func TestMessageDetailViewSaveSnippetWithoutBodyShowsError(t *testing.T) {
	host, _, _, dv := newTestMessageDetailView(t)
	dv.Render("orders", queue.Message{ID: "ID:test:1:1", JMSType: "bytes", JMSTypeInferred: true, Timestamp: time.Now()})

	dv.textView.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone))

	if dv.snippetSave.Visible() {
		t.Error("'S' opened the save dialog for a message without a text body")
	}
	if !strings.Contains(host.status, "[red]") || !strings.Contains(host.status, "no text body") {
		t.Errorf("status = %q, want a red no-text-body error", host.status)
	}
}

func TestMessageDetailViewShortcutSavePresent(t *testing.T) {
	_, _, _, dv := newTestMessageDetailView(t)
	for _, s := range dv.Shortcuts() {
		if s.Key == "S" && s.Description == "save as snippet" {
			return
		}
	}
	t.Error(`Shortcuts() missing {S, "save as snippet"}`)
}

func TestMessageDetailViewRestoreFocus(t *testing.T) {
	host, _, _, dv := newTestMessageDetailView(t)
	host.focused = nil
	dv.restoreFocus()

	if host.focused != dv.textView {
		t.Error("restoreFocus did not focus the detail view")
	}
	for _, key := range []string{"<c>", "<m>", "<d>", "<S>", "<Esc>"} {
		if !strings.Contains(host.contextHint, key) {
			t.Errorf("context hint %q is missing %s", host.contextHint, key)
		}
	}
}
