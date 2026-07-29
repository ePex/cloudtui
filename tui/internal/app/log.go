package app

import (
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/ui"
)

// logView is the Log screen: a scrollable read-only tview.TextView that
// displays the contents of ~/.cloudtui/cloudtui.log. It reloads on Activate
// (navigation) and on the 'r' shortcut.
type logView struct {
	textView *tview.TextView
	app      *App
	path     string
}

var _ ui.View = (*logView)(nil)
var _ ui.Shortcuttable = (*logView)(nil)

func (lv *logView) Name() string               { return "log" }
func (lv *logView) Title() string              { return "Log" }
func (lv *logView) Primitive() tview.Primitive { return lv.textView }

func (lv *logView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
	}
}

// newLogView constructs the log view, defaulting to ~/.cloudtui/cloudtui.log.
func newLogView(a *App) *logView {
	home, _ := os.UserHomeDir()
	return newLogViewWithPath(a, filepath.Join(home, ".cloudtui", "cloudtui.log"))
}

// newLogViewWithPath constructs the log view reading from path. Used by tests.
func newLogViewWithPath(a *App, path string) *logView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Log ")
	tv.SetScrollable(true).SetDynamicColors(false).SetWrap(false)

	lv := &logView{textView: tv, app: a, path: path}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'r' {
			lv.load()
			return nil
		}
		return event
	})

	return lv
}

// Activate reloads the log file. Called by switchTo each time the log view
// becomes active.
func (lv *logView) Activate() {
	lv.load()
}

// load reads lv.path and displays its contents, or a fallback message when
// the file is absent or unreadable.
func (lv *logView) load() {
	data, err := os.ReadFile(lv.path)
	if err != nil {
		if os.IsNotExist(err) {
			lv.textView.SetText("No log file found.")
			return
		}
		lv.textView.SetText("Error reading log: " + err.Error())
		return
	}
	lv.textView.SetText(string(data))
	lv.textView.ScrollToEnd()
}
