package view

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

func TestLogViewActivateWithMissingFile(t *testing.T) {
	lv := NewLogViewWithPath(filepath.Join(t.TempDir(), "nonexistent.log"))
	lv.Activate()
	if got := lv.textView.GetText(true); !strings.Contains(got, "No log file") {
		t.Errorf("Activate() with missing file: text = %q, want 'No log file' message", got)
	}
}

func TestLogViewActivateLoadsFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "cloudtui-*.log")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	const content = "time=2026-07-30 level=INFO msg=startup config=config.yaml theme=dark\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	f.Close()

	lv := NewLogViewWithPath(f.Name())
	lv.Activate()
	if got := lv.textView.GetText(true); !strings.Contains(got, "startup") {
		t.Errorf("Activate() textView text = %q, want log content", got)
	}
}

func TestLogLevelColor(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{`time=2026-08-10 level=ERROR msg="ssm parameters: failed to list parameters" error="boom"`, "red"},
		{`time=2026-08-10 level=WARN msg="awsprofile: skipping unparseable profile"`, "yellow"},
		{`time=2026-08-10 level=INFO msg=startup config=config.yaml theme=dark`, "aqua"},
		{`time=2026-08-10 level=DEBUG msg="verbose detail"`, ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := logLevelColor(c.line); got != c.want {
			t.Errorf("logLevelColor(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestColorizeLogWrapsRecognizedLevels(t *testing.T) {
	data := `time=1 level=ERROR msg=oops
time=2 level=WARN msg=careful
time=3 level=INFO msg=fyi
time=4 level=DEBUG msg=verbose`

	got := strings.Split(colorizeLog(data), "\n")

	want := []string{
		"[red]time=1 level=ERROR msg=oops[-]",
		"[yellow]time=2 level=WARN msg=careful[-]",
		"[aqua]time=3 level=INFO msg=fyi[-]",
		"time=4 level=DEBUG msg=verbose", // no color tag: unrecognized level
	}
	if len(got) != len(want) {
		t.Fatalf("colorizeLog(...) produced %d lines, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestColorizeLogEscapesLiteralBrackets guards against logged content
// containing "[" (e.g. a slice's %v formatting) being misread as a
// tview color tag once dynamic colors are enabled — colorizeLog must
// run every line through tview.Escape, same as the rest of the app's
// dynamic-color views (message/param/secret detail).
func TestColorizeLogEscapesLiteralBrackets(t *testing.T) {
	line := `time=1 level=INFO msg="values=[a b c]"`
	want := fmt.Sprintf("[aqua]%s[-]", tview.Escape(line))

	if got := colorizeLog(line); got != want {
		t.Errorf("colorizeLog(%q) = %q, want %q", line, got, want)
	}
}
