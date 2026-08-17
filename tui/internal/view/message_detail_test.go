package view

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

func newTestMessageDetailView(t *testing.T) (*fakeViewHost, *dialog.MovePicker, *dialog.ConfirmDialog, *MessageDetailView) {
	t.Helper()
	host := newFakeViewHost()
	movePicker := dialog.NewMovePicker(host)
	confirm := dialog.NewConfirmDialog(host)
	return host, movePicker, confirm, NewMessageDetailView(host, movePicker, confirm, func() {}, func() {})
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
