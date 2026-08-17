package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

func newTestMessageDetailView(t *testing.T) *messageDetailView {
	t.Helper()
	a := New(config.Default())
	return newMessageDetailView(a, a.movePicker, a.confirm, func() {}, func() {})
}

func TestMessageDetailViewTitle(t *testing.T) {
	dv := newTestMessageDetailView(t)
	if got, want := dv.textView.GetTitle(), " Message Details "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestMessageDetailViewShortcutEscPresent(t *testing.T) {
	dv := newTestMessageDetailView(t)
	for _, s := range dv.Shortcuts() {
		if s.Key == "Esc" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"Esc\"")
}

func TestMessageDetailViewRenderNilRawFields(t *testing.T) {
	dv := newTestMessageDetailView(t)
	// Must not panic when RawFields is nil.
	dv.render("test-queue", queue.Message{
		ID:        "ID:test:1:1",
		JMSType:   "text",
		Timestamp: time.Now(),
		Preview:   "hello",
		RawFields: nil,
	})
}

func TestMessageDetailViewRenderWithRawFields(t *testing.T) {
	dv := newTestMessageDetailView(t)
	dv.render("test-queue", queue.Message{
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
}

func TestMessageDetailViewEscReturnsToMessages(t *testing.T) {
	a := New(config.Default())
	a.OpenMessageDetail("orders", queue.Message{ID: "ID:test:1:1", Timestamp: time.Now()})

	capture := a.messageDetailV.textView.GetInputCapture()
	if capture == nil {
		t.Fatal("messageDetailV.textView has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if name, _ := a.pages.GetFrontPage(); name != "messages" {
		t.Errorf("front page after Esc = %q, want %q", name, "messages")
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
	a := New(config.Default())
	a.OpenMessageDetail("orders", queue.Message{ID: "ID:test:1:1", Timestamp: time.Now()})

	capture := a.messageDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'm', tcell.ModNone))

	if !a.movePicker.Visible() {
		t.Error("'m' should open the move picker")
	}
}

func TestMessageDetailViewDeleteOpensConfirmWithPrompt(t *testing.T) {
	a := New(config.Default())
	a.OpenMessageDetail("orders", queue.Message{ID: "ID:test:1:1", Timestamp: time.Now()})

	capture := a.messageDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))

	if !a.confirm.Visible() {
		t.Fatal("'d' should open the confirm dialog")
	}
	want := `Delete message from "orders"?`
	if got := renderedScreenText(t, a.confirm.Primitive(), 60, 8); !strings.Contains(got, want) {
		t.Errorf("rendered confirm dialog = %q, want it to contain %q", got, want)
	}
}
