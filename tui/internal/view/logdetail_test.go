package view

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
)

func newTestLogDetailView(t *testing.T) (*fakeViewHost, *LogDetailView) {
	t.Helper()
	host := newFakeViewHost()
	return host, NewLogDetailView(host, func() {})
}

func TestLogDetailViewRenderShowsMessage(t *testing.T) {
	_, dv := newTestLogDetailView(t)
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	dv.Render(awslogs.LogEvent{Timestamp: ts, LogStream: "stream-1", Message: "boom: something failed"})

	text := dv.textView.GetText(true)
	for _, want := range []string{"stream-1", "boom: something failed"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text = %q, want it to contain %q", text, want)
		}
	}
}

func TestLogDetailViewShortcutsAlwaysIncludeCopy(t *testing.T) {
	_, dv := newTestLogDetailView(t)

	found := false
	for _, sc := range dv.Shortcuts() {
		if sc.Key == "c" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"c\" (copy) — log events are never masked, copy should always be available")
	}
}

func TestLogDetailViewCopyWritesMessageToClipboard(t *testing.T) {
	host, dv := newTestLogDetailView(t)
	dv.Render(awslogs.LogEvent{Message: "hello"})

	capture := dv.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := host.copiedData; got != "hello" {
		t.Errorf("copied data = %q, want %q", got, "hello")
	}
}
