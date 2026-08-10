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
		Service:   "fibuproxy",
		Status:    "error",
		Host:      "host-1",
		Tags:      []string{"env:testt"},
		Message:   "boom: something failed",
	})

	text := a.datadogLogDetailV.textView.GetText(true)
	for _, want := range []string{"fibuproxy", "error", "host-1", "env:testt", "boom: something failed"} {
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

	a.openDatadogLogDetail(datadoglogs.LogEvent{Message: "hello"})

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
	a.openDatadogLogDetail(datadoglogs.LogEvent{Message: "hello"})

	capture := a.datadogLogDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := string(screen.GetClipboardData()); got != "hello" {
		t.Errorf("clipboard = %q, want %q", got, "hello")
	}
}

func TestDatadogLogDetailViewEscReturnsToDatadogLogs(t *testing.T) {
	a := New(config.Default())
	a.openDatadogLogDetail(datadoglogs.LogEvent{Message: "hello"})

	capture := a.datadogLogDetailV.textView.GetInputCapture()
	if capture == nil {
		t.Fatal("datadogLogDetailV.textView has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if name, _ := a.pages.GetFrontPage(); name != "datadog-logs" {
		t.Errorf("front page after Esc = %q, want %q", name, "datadog-logs")
	}
}
