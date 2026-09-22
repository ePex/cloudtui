package dialog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/snippet"
)

// newSaveFixture builds an open save dialog over an empty temp library,
// ready to save sn. closed counts onClose calls.
func newSaveFixture(t *testing.T, sn snippet.Snippet) (sd *SnippetSaveDialog, host *testHost, root string, closed *int) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "snippets")
	host = newTestHost()
	sd = NewSnippetSaveDialog(host, snippet.NewStore(root), NewConfirmDialog(host))
	closed = new(int)
	sd.Show(sn, func() { *closed++ })
	return sd, host, root, closed
}

// pressEnterInForm sends Enter through the form's input handler the way
// tview.Application does, including a setFocus that — like the real one —
// records the new focus and focuses that primitive. Focus-stealing by
// tview.Form's own "finished" handler would show up in host.focused.
func pressEnterInForm(sd *SnippetSaveDialog, host *testHost) {
	var setFocus func(tview.Primitive)
	setFocus = func(p tview.Primitive) {
		host.focused = p
		p.Focus(setFocus)
	}
	sd.form.Focus(setFocus)
	sd.form.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)
}

func readSnippet(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(data)
}

func TestSnippetSaveShow(t *testing.T) {
	sd, host, _, _ := newSaveFixture(t, snippet.Snippet{Body: "b"})
	sd.nameItem.SetText("left over")
	sd.Show(snippet.Snippet{Body: "b"}, func() {})

	if sd.nameItem.GetText() != "" {
		t.Error("Show did not clear the Name field")
	}
	if !sd.Visible() || host.focused != sd.form {
		t.Error("Show did not make the dialog visible and focused")
	}
	if host.shownPages[len(host.shownPages)-1] != "snippet-save" {
		t.Errorf("shownPages = %q, want snippet-save last", host.shownPages)
	}
	if !strings.Contains(host.contextHint, "subfolders") {
		t.Errorf("context hint = %q, want the subfolder tip", host.contextHint)
	}
}

func TestSnippetSaveWritesFile(t *testing.T) {
	sn := snippet.Snippet{JMSType: "OrderCreated", Body: `{"id":1}`}
	sd, host, root, closed := newSaveFixture(t, sn)
	sd.nameItem.SetText("orders/created.json")

	pressEnterInForm(sd, host)

	if got, want := readSnippet(t, root, "orders/created.json"), string(snippet.Format(sn)); got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	if *closed != 1 || sd.Visible() {
		t.Errorf("closed = %d, visible = %v; want 1, false", *closed, sd.Visible())
	}
	if !strings.Contains(host.status, "orders/created.json") || strings.Contains(host.status, "[red]") {
		t.Errorf("status = %q, want a success message naming the snippet", host.status)
	}
}

func TestSnippetSaveButtonSaves(t *testing.T) {
	sd, _, root, _ := newSaveFixture(t, snippet.Snippet{Body: "b"})
	sd.nameItem.SetText("ping.txt")
	sd.form.GetButton(sd.form.GetButtonIndex("Save")).InputHandler()(
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if got := readSnippet(t, root, "ping.txt"); got != "b" {
		t.Errorf("file = %q, want %q", got, "b")
	}
}

func TestSnippetSaveValidationErrorStaysOpen(t *testing.T) {
	for _, name := range []string{"", "../x", "/abs", ".hidden", "orders/"} {
		t.Run(name, func(t *testing.T) {
			sd, host, root, closed := newSaveFixture(t, snippet.Snippet{Body: "b"})
			sd.nameItem.SetText(name)

			pressEnterInForm(sd, host)

			if *closed != 0 || !sd.Visible() {
				t.Errorf("closed = %d, visible = %v; want 0, true", *closed, sd.Visible())
			}
			if !strings.Contains(host.status, "[red]") {
				t.Errorf("status = %q, want a red error", host.status)
			}
			if item, button := sd.form.GetFocusedItemIndex(); host.focused != sd.form || item != 0 || button != -1 {
				t.Errorf("focus = %T (item %d, button %d), want the form on its Name field", host.focused, item, button)
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("snippets folder was touched: %v", err)
			}
		})
	}
}

func TestSnippetSaveExistingAsksBeforeOverwrite(t *testing.T) {
	newExisting := func(t *testing.T) (*SnippetSaveDialog, *testHost, string, *int) {
		t.Helper()
		sd, host, root, closed := newSaveFixture(t, snippet.Snippet{Body: "new"})
		writeSnippetFile(t, root, "ping.txt", "old")
		sd.nameItem.SetText("ping.txt")
		pressEnterInForm(sd, host)
		if !sd.confirm.Visible() {
			t.Fatal("no overwrite confirmation for an existing snippet")
		}
		if host.focused != sd.confirm.list {
			t.Fatalf("focus = %T, want the confirmation (not stolen back by the form)", host.focused)
		}
		return sd, host, root, closed
	}

	t.Run("No keeps the file and refocuses Name", func(t *testing.T) {
		sd, host, root, closed := newExisting(t)
		selectConfirmItem(sd.confirm, 0)

		if got := readSnippet(t, root, "ping.txt"); got != "old" {
			t.Errorf("file = %q, want it untouched", got)
		}
		if *closed != 0 || !sd.Visible() {
			t.Errorf("closed = %d, visible = %v; want 0, true", *closed, sd.Visible())
		}
		if host.focused != sd.form || host.focusMainCalls != 0 {
			t.Errorf("focus = %T, FocusMain calls = %d; want the form, 0", host.focused, host.focusMainCalls)
		}
		if sd.nameItem.GetText() != "ping.txt" {
			t.Errorf("Name = %q, want the typed name kept for editing", sd.nameItem.GetText())
		}
	})

	t.Run("Yes overwrites and closes", func(t *testing.T) {
		sd, host, root, closed := newExisting(t)
		selectConfirmItem(sd.confirm, 1)

		if got := readSnippet(t, root, "ping.txt"); got != "new" {
			t.Errorf("file = %q, want %q", got, "new")
		}
		if *closed != 1 || sd.Visible() {
			t.Errorf("closed = %d, visible = %v; want 1, false", *closed, sd.Visible())
		}
		if !strings.Contains(host.status, "ping.txt") || strings.Contains(host.status, "[red]") {
			t.Errorf("status = %q, want a success message", host.status)
		}
	})
}

func TestSnippetSaveWriteErrorStaysOpen(t *testing.T) {
	sd, host, root, closed := newSaveFixture(t, snippet.Snippet{Body: "b"})
	// A file where the subfolder should be makes MkdirAll fail.
	writeSnippetFile(t, root, "orders", "not a folder")
	sd.nameItem.SetText("orders/created.json")

	pressEnterInForm(sd, host)

	if *closed != 0 || !sd.Visible() || !strings.Contains(host.status, "[red]") {
		t.Errorf("closed = %d, visible = %v, status = %q; want 0, true, a red error", *closed, sd.Visible(), host.status)
	}
}

func TestSnippetSaveEscCancels(t *testing.T) {
	sd, _, root, closed := newSaveFixture(t, snippet.Snippet{Body: "b"})
	sd.nameItem.SetText("ping.txt")
	sd.form.GetInputCapture()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if *closed != 1 || sd.Visible() {
		t.Errorf("closed = %d, visible = %v; want 1, false", *closed, sd.Visible())
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Esc wrote something: %v", err)
	}
}

// TestSnippetSaveFitsOverlay renders the form at the size app.go gives
// it (64×8), so a field or button that no longer fits is caught here.
func TestSnippetSaveFitsOverlay(t *testing.T) {
	sd, _, _, _ := newSaveFixture(t, snippet.Snippet{Body: "b"})
	text := renderedScreenText(t, sd.Primitive(), 64, 8)
	for _, label := range []string{"Save as Snippet", "Name", "Save", "Cancel"} {
		if !strings.Contains(text, label) {
			t.Errorf("rendered overlay is missing %q", label)
		}
	}
}
