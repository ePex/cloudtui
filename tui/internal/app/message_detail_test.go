package app

import (
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

func newTestMessageDetailView(t *testing.T) *messageDetailView {
	t.Helper()
	a := New(config.Default())
	return newMessageDetailView(a)
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
