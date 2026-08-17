package view

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// LogView is the Log screen: a scrollable read-only tview.TextView that
// displays the contents of ~/.cloudtui/cloudtui.log. It reloads on Activate
// (navigation) and on the 'r' shortcut.
type LogView struct {
	textView *tview.TextView
	path     string
}

var _ ui.View = (*LogView)(nil)
var _ ui.Shortcuttable = (*LogView)(nil)
var _ ui.Themeable = (*LogView)(nil)

func (lv *LogView) Name() string               { return "log" }
func (lv *LogView) Title() string              { return "Log" }
func (lv *LogView) Primitive() tview.Primitive { return lv.textView }

func (lv *LogView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
	}
}

// NewLogView constructs the log view, defaulting to ~/.cloudtui/cloudtui.log.
func NewLogView() *LogView {
	home, _ := os.UserHomeDir()
	return NewLogViewWithPath(filepath.Join(home, ".cloudtui", "cloudtui.log"))
}

// NewLogViewWithPath constructs the log view reading from path. Used by tests.
func NewLogViewWithPath(path string) *LogView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Log ")
	tv.SetScrollable(true).SetDynamicColors(true).SetWrap(true).SetWordWrap(true)

	lv := &LogView{textView: tv, path: path}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'r' {
			lv.load()
			return nil
		}
		return event
	})

	return lv
}

// Activate reloads the log file. Called by SwitchTo each time the log view
// becomes active.
func (lv *LogView) Activate() {
	lv.load()
}

// load reads lv.path and displays its contents, or a fallback message when
// the file is absent or unreadable.
func (lv *LogView) load() {
	data, err := os.ReadFile(lv.path)
	if err != nil {
		if os.IsNotExist(err) {
			lv.textView.SetText("No log file found.")
			return
		}
		lv.textView.SetText("Error reading log: " + err.Error())
		return
	}
	lv.textView.SetText(colorizeLog(string(data)))
	lv.textView.ScrollToEnd()
}

// colorizeLog wraps each line in a tview color tag matching its slog
// level — error red, warn yellow, info aqua (tcell has no separate
// "cyan" name; aqua is the same 0x00FFFF value, and is what "cyan" means
// in tview color tags). Lines with no recognized level (e.g. a
// multi-line value continuing the previous entry) are left in the
// default color. Each line is escaped first so a literal "[" in logged
// content (e.g. a Go slice's %v formatting) isn't misread as a color
// tag — see queues.go's updateTitle for the same concern elsewhere.
func colorizeLog(data string) string {
	lines := strings.Split(data, "\n")
	for i, line := range lines {
		color := logLevelColor(line)
		if color == "" {
			lines[i] = tview.Escape(line)
			continue
		}
		lines[i] = fmt.Sprintf("[%s]%s[-]", color, tview.Escape(line))
	}
	return strings.Join(lines, "\n")
}

// ApplyPalette recolors the log view for a live theme switch.
func (lv *LogView) ApplyPalette(p config.Palette) {
	lv.textView.SetBackgroundColor(tcell.GetColor(p.Background))
	lv.textView.SetBorderColor(tcell.GetColor(p.ViewColor("log")))
	lv.textView.SetTitleColor(tcell.GetColor(p.ViewColor("log")))
}

func logLevelColor(line string) string {
	switch {
	case strings.Contains(line, "level=ERROR"):
		return "red"
	case strings.Contains(line, "level=WARN"):
		return "yellow"
	case strings.Contains(line, "level=INFO"):
		return "aqua"
	default:
		return ""
	}
}
