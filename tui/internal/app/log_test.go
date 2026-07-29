package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

func TestLogViewName(t *testing.T) {
	a := New(config.Default())
	if got := a.logV.Name(); got != "log" {
		t.Errorf("Name() = %q, want %q", got, "log")
	}
}

func TestLogViewImplementsShortcuttable(t *testing.T) {
	a := New(config.Default())
	_, ok := ui.View(a.logV).(ui.Shortcuttable)
	if !ok {
		t.Error("logView does not implement ui.Shortcuttable")
	}
}

func TestLogViewShortcutsIncludeR(t *testing.T) {
	a := New(config.Default())
	for _, s := range a.logV.Shortcuts() {
		if s.Key == "r" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"r\"")
}

func TestLogViewActivateWithMissingFile(t *testing.T) {
	a := New(config.Default())
	lv := newLogViewWithPath(a, filepath.Join(t.TempDir(), "nonexistent.log"))
	lv.Activate()
	if got := lv.textView.GetText(true); !strings.Contains(got, "No log file") {
		t.Errorf("Activate() with missing file: text = %q, want 'No log file' message", got)
	}
}

func TestLogViewActivateLoadsFile(t *testing.T) {
	a := New(config.Default())
	f, err := os.CreateTemp(t.TempDir(), "cloudtui-*.log")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	const content = "time=2026-07-30 level=INFO msg=startup config=config.yaml theme=dark\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	lv := newLogViewWithPath(a, f.Name())
	lv.Activate()
	if got := lv.textView.GetText(true); !strings.Contains(got, "startup") {
		t.Errorf("Activate() textView text = %q, want log content", got)
	}
}
