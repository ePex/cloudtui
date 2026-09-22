package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/snippet"
)

// editorFixture is an editor over a temp library, with what its
// callbacks received.
type editorFixture struct {
	se     *SnippetEditor
	host   *testHost
	root   string
	store  *snippet.Store
	saved  []string
	closed int
}

func newEditorFixture(t *testing.T) *editorFixture {
	t.Helper()
	root := t.TempDir()
	host := newTestHost()
	store := snippet.NewStore(root)
	return &editorFixture{se: NewSnippetEditor(host, store, NewConfirmDialog(host)), host: host, root: root, store: store}
}

func (f *editorFixture) onSaved(path string) { f.saved = append(f.saved, filepath.ToSlash(path)) }
func (f *editorFixture) onClose()            { f.closed++ }

func (f *editorFixture) showNew(dir string) {
	f.se.ShowNew(filepath.FromSlash(dir), f.onSaved, f.onClose)
}

// showEdit loads rel from disk and opens it for editing.
func (f *editorFixture) showEdit(t *testing.T, rel string) {
	t.Helper()
	sn, err := f.store.Load(filepath.FromSlash(rel))
	if err != nil {
		t.Fatal(err)
	}
	f.se.ShowEdit(filepath.FromSlash(rel), sn, f.onSaved, f.onClose)
}

func (f *editorFixture) set(name, jmsType, body string) {
	f.se.nameItem.SetText(name)
	f.se.jmsTypeItem.SetText(jmsType)
	f.se.bodyItem.SetText(body, false)
}

// pressEnter sends Enter through the form as tview.Application does,
// with a setFocus that records and applies focus, so tview.Form moving
// focus on by itself shows up in host.focused.
func (f *editorFixture) pressEnter() {
	var setFocus func(tview.Primitive)
	setFocus = func(p tview.Primitive) {
		f.host.focused = p
		p.Focus(setFocus)
	}
	f.se.form.Focus(setFocus)
	f.se.form.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)
}

func (f *editorFixture) pressEsc() {
	f.se.form.GetInputCapture()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
}

func (f *editorFixture) file(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(data)
}

func (f *editorFixture) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(rel)))
	return err == nil
}

func TestSnippetEditorShowNew(t *testing.T) {
	f := newEditorFixture(t)
	f.showNew("orders")

	if f.se.values() != [3]string{"", "", ""} {
		t.Errorf("fields = %q, want empty", f.se.values())
	}
	if !f.se.Visible() || f.host.focused != f.se.form || f.host.shownPages[len(f.host.shownPages)-1] != "snippet-editor" {
		t.Error("ShowNew did not open and focus the editor")
	}
	if title := f.se.form.GetTitle(); !strings.Contains(title, "New Snippet") || !strings.Contains(title, "/orders") {
		t.Errorf("title = %q", title)
	}
}

func TestSnippetEditorFitsOverlay(t *testing.T) {
	f := newEditorFixture(t)
	f.showNew("")
	text := renderedScreenText(t, f.se.Primitive(), 90, 26)
	for _, want := range []string{"New Snippet", "Name", "JMS Type", "Body", "Save", "Cancel"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered editor (90x26) lacks %q", want)
		}
	}
}

func TestSnippetEditorSaveNew(t *testing.T) {
	for _, tt := range []struct {
		name, dir, typed, want string
	}{
		{"in the current folder", "orders", "created.json", "orders/created.json"},
		{"into a new subfolder", "orders", "eu/created.json", "orders/eu/created.json"},
		{"at the root", "", "ping.txt", "ping.txt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newEditorFixture(t)
			f.showNew(tt.dir)
			f.set(tt.typed, "  OrderCreated  ", "{\n  \"id\": 1\n}")

			f.pressEnter()

			want := string(snippet.Format(snippet.Snippet{JMSType: "OrderCreated", Body: "{\n  \"id\": 1\n}"}))
			if got := f.file(t, tt.want); got != want {
				t.Errorf("file = %q, want %q (JMS Type trimmed)", got, want)
			}
			if f.se.Visible() || f.closed != 1 || len(f.saved) != 1 || f.saved[0] != tt.want {
				t.Errorf("visible = %v, closed = %d, saved = %q; want false, 1, [%s]", f.se.Visible(), f.closed, f.saved, tt.want)
			}
			if !strings.Contains(f.host.status, "Snippet saved") {
				t.Errorf("status = %q", f.host.status)
			}
		})
	}
}

func TestSnippetEditorNewOverwritePrompt(t *testing.T) {
	setup := func(t *testing.T) *editorFixture {
		f := newEditorFixture(t)
		writeSnippetFile(t, f.root, "ping.txt", "old")
		f.showNew("")
		f.set("ping.txt", "", "new")
		f.pressEnter()
		if !f.se.confirm.Visible() {
			t.Fatal("no overwrite confirmation")
		}
		if f.host.focused != f.se.confirm.list {
			t.Fatalf("focus = %T, want the confirmation (not stolen back by the form)", f.host.focused)
		}
		return f
	}
	t.Run("No keeps the file and the editor", func(t *testing.T) {
		f := setup(t)
		selectConfirmItem(f.se.confirm, 0)
		if f.file(t, "ping.txt") != "old" || !f.se.Visible() || f.host.focused != f.se.form || f.host.focusMainCalls != 0 {
			t.Errorf("file = %q, visible = %v, focus = %T, FocusMain = %d", f.file(t, "ping.txt"), f.se.Visible(), f.host.focused, f.host.focusMainCalls)
		}
	})
	t.Run("Yes overwrites", func(t *testing.T) {
		f := setup(t)
		selectConfirmItem(f.se.confirm, 1)
		if f.file(t, "ping.txt") != "new" || f.se.Visible() || len(f.saved) != 1 {
			t.Errorf("file = %q, visible = %v, saved = %q", f.file(t, "ping.txt"), f.se.Visible(), f.saved)
		}
	})
}

func TestSnippetEditorRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"", "   ", "../x.txt", "/abs.txt", ".hidden", "a/"} {
		t.Run(name, func(t *testing.T) {
			f := newEditorFixture(t)
			f.showNew("orders")
			f.set(name, "", "x")
			f.pressEnter()

			if !f.se.Visible() || f.closed != 0 || !strings.Contains(f.host.status, "[red]") {
				t.Errorf("visible = %v, closed = %d, status = %q; want the editor open with an error", f.se.Visible(), f.closed, f.host.status)
			}
			if entries, _ := os.ReadDir(f.root); len(entries) != 0 {
				t.Errorf("something was written: %v", entries)
			}
		})
	}
}

// TestSnippetEditorEditKeepsExtra covers editing a hand-written shared
// snippet: its unknown key and comments survive; only JMS Type and Body
// change.
func TestSnippetEditorEditKeepsExtra(t *testing.T) {
	f := newEditorFixture(t)
	writeSnippetFile(t, f.root, "orders/created.json",
		"---\n# Shared by the payments team.\nauthor: someone # ask them\njmsType: OrderCreated\n---\n{}")
	f.showEdit(t, "orders/created.json")

	if got := f.se.values(); got != [3]string{"created.json", "OrderCreated", "{}"} {
		t.Fatalf("fields = %q", got)
	}
	f.se.jmsTypeItem.SetText("OrderUpdated")
	f.se.bodyItem.SetText(`{"v":2}`, false)
	f.pressEnter()

	want := "---\n# Shared by the payments team.\nauthor: someone # ask them\njmsType: OrderUpdated\n---\n{\n  \"v\": 2\n}"
	if got := f.file(t, "orders/created.json"); got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
	if len(f.saved) != 1 || f.saved[0] != "orders/created.json" {
		t.Errorf("saved = %q", f.saved)
	}
}

func TestSnippetEditorEditRename(t *testing.T) {
	f := newEditorFixture(t)
	writeSnippetFile(t, f.root, "orders/created.json", "---\njmsType: T\n---\nold")
	f.showEdit(t, "orders/created.json")
	f.se.nameItem.SetText("eu/renamed.json")
	f.se.bodyItem.SetText("new", false)
	f.pressEnter()

	if f.exists("orders/created.json") {
		t.Error("old file still exists")
	}
	if got := f.file(t, "orders/eu/renamed.json"); got != "---\njmsType: T\n---\nnew" {
		t.Errorf("renamed file = %q", got)
	}
	if len(f.saved) != 1 || f.saved[0] != "orders/eu/renamed.json" {
		t.Errorf("saved = %q", f.saved)
	}
}

func TestSnippetEditorEditRenameOntoExistingIsRefused(t *testing.T) {
	f := newEditorFixture(t)
	writeSnippetFile(t, f.root, "a.txt", "A")
	writeSnippetFile(t, f.root, "b.txt", "B")
	f.showEdit(t, "a.txt")
	f.se.nameItem.SetText("b.txt")
	f.se.bodyItem.SetText("changed", false)
	f.pressEnter()

	if f.file(t, "a.txt") != "A" || f.file(t, "b.txt") != "B" {
		t.Errorf("files changed: a = %q, b = %q", f.file(t, "a.txt"), f.file(t, "b.txt"))
	}
	if !f.se.Visible() || f.se.confirm.Visible() || !strings.Contains(f.host.status, "already exists") {
		t.Errorf("visible = %v, confirm = %v, status = %q; want the editor open with an error, no overwrite prompt",
			f.se.Visible(), f.se.confirm.Visible(), f.host.status)
	}
}

func TestSnippetEditorDiscardChanges(t *testing.T) {
	t.Run("unchanged closes without asking", func(t *testing.T) {
		f := newEditorFixture(t)
		writeSnippetFile(t, f.root, "a.txt", "A")
		f.showEdit(t, "a.txt")
		f.pressEsc()
		if f.se.confirm.Visible() || f.se.Visible() || f.closed != 1 {
			t.Errorf("confirm = %v, visible = %v, closed = %d; want false, false, 1", f.se.confirm.Visible(), f.se.Visible(), f.closed)
		}
	})
	t.Run("changed asks; No keeps editing", func(t *testing.T) {
		f := newEditorFixture(t)
		f.showNew("")
		f.set("draft.txt", "", "typed")
		f.pressEsc()
		if !f.se.confirm.Visible() {
			t.Fatal("no Discard changes? confirmation")
		}
		selectConfirmItem(f.se.confirm, 0)
		if !f.se.Visible() || f.se.values() != [3]string{"draft.txt", "", "typed"} || f.host.focused != f.se.form {
			t.Errorf("visible = %v, values = %q, focus = %T", f.se.Visible(), f.se.values(), f.host.focused)
		}
	})
	t.Run("changed asks; Yes discards", func(t *testing.T) {
		f := newEditorFixture(t)
		f.showNew("")
		f.set("draft.txt", "", "typed")
		f.pressEsc()
		selectConfirmItem(f.se.confirm, 1)
		if f.se.Visible() || f.closed != 1 || f.exists("draft.txt") || len(f.saved) != 0 {
			t.Errorf("visible = %v, closed = %d, written = %v, saved = %q", f.se.Visible(), f.closed, f.exists("draft.txt"), f.saved)
		}
	})
}
