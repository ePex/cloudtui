package view

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
)

func newTestDatadogLogDetailView(t *testing.T) (*fakeViewHost, *DatadogLogDetailView) {
	t.Helper()
	host := newFakeViewHost()
	return host, NewDatadogLogDetailView(host, func() {})
}

func TestDatadogLogDetailViewRenderShowsMessage(t *testing.T) {
	_, dv := newTestDatadogLogDetailView(t)
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	dv.Render(datadoglogs.LogEvent{
		Timestamp: ts,
		Service:   "bar-proxy",
		Env:       "testt",
		Status:    "error",
		Host:      "host-1",
		Tags:      []string{"env:testt"},
		Message:   "boom: something failed",
	})

	text := dv.textView.GetText(true)
	for _, want := range []string{"bar-proxy", "testt", "error", "host-1", "env:testt", "boom: something failed"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text = %q, want it to contain %q", text, want)
		}
	}
}

func TestDatadogLogDetailViewShortcutsAlwaysIncludeCopy(t *testing.T) {
	_, dv := newTestDatadogLogDetailView(t)

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

func TestDatadogLogDetailViewCopyWritesMessageToClipboard(t *testing.T) {
	host, dv := newTestDatadogLogDetailView(t)
	dv.Render(datadoglogs.LogEvent{Message: "hello"})

	capture := dv.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := host.copiedData; got != "hello" {
		t.Errorf("copied data = %q, want %q", got, "hello")
	}
}

func TestDatadogLogDetailViewShortcutsIncludeGoToCloudWatch(t *testing.T) {
	_, dv := newTestDatadogLogDetailView(t)

	found := false
	for _, sc := range dv.Shortcuts() {
		if sc.Key == "g" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"g\" (go to CloudWatch)")
	}
}

func TestExtractCorrelationID(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
		wantOk  bool
	}{
		{
			name:    "confirmed real format",
			message: "some log line CorrelationID: 1745d042-94e8-49f0-b223-8900ed9e951e trailing text",
			want:    "1745d042-94e8-49f0-b223-8900ed9e951e",
			wantOk:  true,
		},
		{
			name:    "case-insensitive label",
			message: "correlationid: 1745d042-94e8-49f0-b223-8900ed9e951e",
			want:    "1745d042-94e8-49f0-b223-8900ed9e951e",
			wantOk:  true,
		},
		{
			name:    "no correlation id present",
			message: "just a plain log message",
			wantOk:  false,
		},
		{
			name:    "malformed uuid does not match",
			message: "CorrelationID: not-a-real-uuid",
			wantOk:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := extractCorrelationID(c.message)
			if ok != c.wantOk {
				t.Fatalf("extractCorrelationID(%q) ok = %v, want %v", c.message, ok, c.wantOk)
			}
			if ok && got != c.want {
				t.Errorf("extractCorrelationID(%q) = %q, want %q", c.message, got, c.want)
			}
		})
	}
}
