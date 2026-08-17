package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
)

func TestDatadogLogDetailViewRenderShowsMessage(t *testing.T) {
	a := New(config.Default())
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	a.datadogLogDetailV.render(datadoglogs.LogEvent{
		Timestamp: ts,
		Service:   "bar-proxy",
		Env:       "testt",
		Status:    "error",
		Host:      "host-1",
		Tags:      []string{"env:testt"},
		Message:   "boom: something failed",
	})

	text := a.datadogLogDetailV.textView.GetText(true)
	for _, want := range []string{"bar-proxy", "testt", "error", "host-1", "env:testt", "boom: something failed"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text = %q, want it to contain %q", text, want)
		}
	}
}

func TestDatadogLogDetailViewShortcutsAlwaysIncludeCopy(t *testing.T) {
	a := New(config.Default())

	found := false
	for _, sc := range a.datadogLogDetailV.Shortcuts() {
		if sc.Key == "c" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"c\" (copy) — log events are never masked, copy should always be available")
	}
}

func TestOpenDatadogLogDetailSwitchesPage(t *testing.T) {
	a := New(config.Default())

	a.OpenDatadogLogDetail(datadoglogs.LogEvent{Message: "hello"})

	if name, _ := a.pages.GetFrontPage(); name != "datadog-log-detail" {
		t.Errorf("front page = %q, want %q", name, "datadog-log-detail")
	}
}

func TestDatadogLogDetailViewCopyWritesMessageToClipboard(t *testing.T) {
	a := New(config.Default())
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	a.screen = screen
	a.OpenDatadogLogDetail(datadoglogs.LogEvent{Message: "hello"})

	capture := a.datadogLogDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := string(screen.GetClipboardData()); got != "hello" {
		t.Errorf("clipboard = %q, want %q", got, "hello")
	}
}

func TestDatadogLogDetailViewEscReturnsToDatadogLogs(t *testing.T) {
	a := New(config.Default())
	a.OpenDatadogLogDetail(datadoglogs.LogEvent{Message: "hello"})

	capture := a.datadogLogDetailV.textView.GetInputCapture()
	if capture == nil {
		t.Fatal("datadogLogDetailV.textView has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if name, _ := a.pages.GetFrontPage(); name != "datadog-logs" {
		t.Errorf("front page after Esc = %q, want %q", name, "datadog-logs")
	}
}

func TestDatadogLogDetailViewShortcutsIncludeGoToCloudWatch(t *testing.T) {
	a := New(config.Default())

	found := false
	for _, sc := range a.datadogLogDetailV.Shortcuts() {
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

func TestDatadogLogDetailViewGoToCloudWatchWithCorrelationID(t *testing.T) {
	a := New(config.Default())
	a.OpenDatadogLogDetail(datadoglogs.LogEvent{
		Message: "something happened CorrelationID: 1745d042-94e8-49f0-b223-8900ed9e951e",
	})

	capture := a.datadogLogDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))

	// Quoted (see the 'g' handler's comment): CloudWatch's filter-pattern
	// syntax otherwise tokenizes on the UUID's internal hyphens.
	if want := `"1745d042-94e8-49f0-b223-8900ed9e951e"`; a.pendingCloudWatchPattern != want {
		t.Errorf("pendingCloudWatchPattern = %q, want %q", a.pendingCloudWatchPattern, want)
	}
	if name, _ := a.pages.GetFrontPage(); name != "cloudwatch-logs" {
		t.Errorf("front page after 'g' = %q, want %q", name, "cloudwatch-logs")
	}
}

func TestDatadogLogDetailViewGoToCloudWatchWithoutCorrelationID(t *testing.T) {
	a := New(config.Default())
	a.OpenDatadogLogDetail(datadoglogs.LogEvent{Message: "no correlation id here"})

	capture := a.datadogLogDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))

	if a.pendingCloudWatchPattern != "" {
		t.Errorf("pendingCloudWatchPattern = %q, want empty", a.pendingCloudWatchPattern)
	}
	if name, _ := a.pages.GetFrontPage(); name != "datadog-log-detail" {
		t.Errorf("front page after 'g' with no CorrelationID = %q, want unchanged %q", name, "datadog-log-detail")
	}
}
