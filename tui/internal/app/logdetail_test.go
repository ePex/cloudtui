package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestLogDetailViewRenderShowsMessage(t *testing.T) {
	a := New(config.Default())
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	a.logDetailV.render(awslogs.LogEvent{Timestamp: ts, LogStream: "stream-1", Message: "boom: something failed"})

	text := a.logDetailV.textView.GetText(true)
	for _, want := range []string{"stream-1", "boom: something failed"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text = %q, want it to contain %q", text, want)
		}
	}
}

func TestLogDetailViewShortcutsAlwaysIncludeCopy(t *testing.T) {
	a := New(config.Default())

	found := false
	for _, sc := range a.logDetailV.Shortcuts() {
		if sc.Key == "c" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"c\" (copy) — log events are never masked, copy should always be available")
	}
}

func TestOpenLogEventDetailSwitchesPage(t *testing.T) {
	a := New(config.Default())

	a.OpenLogEventDetail(awslogs.LogEvent{Message: "hello"})

	if name, _ := a.pages.GetFrontPage(); name != "log-event-detail" {
		t.Errorf("front page = %q, want %q", name, "log-event-detail")
	}
}

func TestLogDetailViewCopyWritesMessageToClipboard(t *testing.T) {
	a := New(config.Default())
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	a.screen = screen
	a.OpenLogEventDetail(awslogs.LogEvent{Message: "hello"})

	capture := a.logDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := string(screen.GetClipboardData()); got != "hello" {
		t.Errorf("clipboard = %q, want %q", got, "hello")
	}
}

func TestLogDetailViewEscReturnsToLogSearch(t *testing.T) {
	a := New(config.Default())
	a.logSearchV.logGroupName = "/aws/lambda/foo"
	a.OpenLogEventDetail(awslogs.LogEvent{Message: "hello"})

	capture := a.logDetailV.textView.GetInputCapture()
	if capture == nil {
		t.Fatal("logDetailV.textView has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if name, _ := a.pages.GetFrontPage(); name != "log-search" {
		t.Errorf("front page after Esc = %q, want %q", name, "log-search")
	}
}
