package dialog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/snippet"
)

// writeSnippetFile creates root/rel (with parent folders) holding content.
func writeSnippetFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pickerItems returns the picker list's item texts in order.
func pickerItems(sp *SnippetPicker) []string {
	items := make([]string, sp.list.GetItemCount())
	for i := range items {
		items[i], _ = sp.list.GetItemText(i)
	}
	return items
}

// pickerSelect puts the cursor on the item with text and presses Enter.
func pickerSelect(t *testing.T, sp *SnippetPicker, text string) {
	t.Helper()
	for i, item := range pickerItems(sp) {
		if item == text {
			sp.list.SetCurrentItem(i)
			sp.list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
			return
		}
	}
	t.Fatalf("no picker item %q in %q", text, pickerItems(sp))
}

// pickerKey sends ev through the list's input capture, the way
// tview.Application would before the list's own handler.
func pickerKey(sp *SnippetPicker, ev *tcell.EventKey) {
	sp.list.GetInputCapture()(ev)
}

// currentPickerItem returns the text of the item under the cursor.
func currentPickerItem(sp *SnippetPicker) string {
	text, _ := sp.list.GetItemText(sp.list.GetCurrentItem())
	return text
}

// newPickerFixture builds a picker over a temp library:
//
//	root/zeta.txt, root/Alpha.json, root/.hidden,
//	root/orders/created.json, root/orders/eu/updated.json, root/archive/
func newPickerFixture(t *testing.T) (*SnippetPicker, *testHost, string) {
	t.Helper()
	root := t.TempDir()
	writeSnippetFile(t, root, "zeta.txt", "z")
	writeSnippetFile(t, root, "Alpha.json", "---\njmsType: A\n---\n{}")
	writeSnippetFile(t, root, ".hidden", "")
	writeSnippetFile(t, root, "orders/created.json", "---\njmsType: OrderCreated\n---\n{\"id\":1}")
	writeSnippetFile(t, root, "orders/eu/updated.json", "u")
	if err := os.Mkdir(filepath.Join(root, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	host := newTestHost()
	return NewSnippetPicker(host, snippet.NewStore(root)), host, root
}

func TestSnippetPickerShowListsRoot(t *testing.T) {
	sp, host, _ := newPickerFixture(t)
	sp.Show(func(snippet.Snippet) {}, func() {})

	want := []string{"archive/", "orders/", "Alpha.json", "zeta.txt"}
	if got := pickerItems(sp); !reflect.DeepEqual(got, want) {
		t.Errorf("items = %q, want %q", got, want)
	}
	if !sp.Visible() || host.focused != sp.list {
		t.Error("Show did not make the picker visible and focused")
	}
	if len(host.shownPages) != 1 || host.shownPages[0] != "snippet-picker" {
		t.Errorf("shownPages = %q, want [snippet-picker]", host.shownPages)
	}
	if !strings.Contains(host.contextHint, "Backspace") {
		t.Errorf("context hint %q doesn't mention Backspace", host.contextHint)
	}
}

func TestSnippetPickerFolderNavigation(t *testing.T) {
	sp, _, _ := newPickerFixture(t)
	sp.Show(func(snippet.Snippet) {}, func() {})

	pickerSelect(t, sp, "orders/")
	if want := []string{"..", "eu/", "created.json"}; !reflect.DeepEqual(pickerItems(sp), want) {
		t.Errorf("orders items = %q, want %q", pickerItems(sp), want)
	}
	if got := sp.list.GetTitle(); !strings.Contains(got, "/orders") {
		t.Errorf("title = %q, want it to show /orders", got)
	}

	pickerSelect(t, sp, "eu/")
	if want := []string{"..", "updated.json"}; !reflect.DeepEqual(pickerItems(sp), want) {
		t.Errorf("orders/eu items = %q, want %q", pickerItems(sp), want)
	}
	if got := sp.list.GetTitle(); !strings.Contains(got, "/orders/eu") {
		t.Errorf("title = %q, want it to show /orders/eu", got)
	}

	// ".." goes up and keeps the cursor on the folder just left.
	pickerSelect(t, sp, "..")
	if currentPickerItem(sp) != "eu/" {
		t.Errorf("cursor after .. = %q, want %q", currentPickerItem(sp), "eu/")
	}

	// Backspace also goes up.
	pickerKey(sp, tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if want := []string{"archive/", "orders/", "Alpha.json", "zeta.txt"}; !reflect.DeepEqual(pickerItems(sp), want) {
		t.Errorf("root items after Backspace = %q, want %q", pickerItems(sp), want)
	}
	if currentPickerItem(sp) != "orders/" {
		t.Errorf("cursor after Backspace = %q, want %q", currentPickerItem(sp), "orders/")
	}

	// Backspace at the root is a no-op: never above the snippets folder.
	pickerKey(sp, tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	if want := []string{"archive/", "orders/", "Alpha.json", "zeta.txt"}; !reflect.DeepEqual(pickerItems(sp), want) {
		t.Errorf("root items after Backspace at root = %q, want %q", pickerItems(sp), want)
	}
	if !sp.Visible() {
		t.Error("Backspace at the root closed the picker")
	}
}

func TestSnippetPickerEmptySubfolderShowsOnlyUp(t *testing.T) {
	sp, _, _ := newPickerFixture(t)
	sp.Show(func(snippet.Snippet) {}, func() {})
	pickerSelect(t, sp, "archive/")
	if want := []string{".."}; !reflect.DeepEqual(pickerItems(sp), want) {
		t.Errorf("items = %q, want %q", pickerItems(sp), want)
	}
}

func TestSnippetPickerShowResetsToRoot(t *testing.T) {
	sp, _, _ := newPickerFixture(t)
	sp.Show(func(snippet.Snippet) {}, func() {})
	pickerSelect(t, sp, "orders/")
	pickerKey(sp, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	sp.Show(func(snippet.Snippet) {}, func() {})
	if got := pickerItems(sp); len(got) == 0 || got[0] != "archive/" {
		t.Errorf("reopened items = %q, want the root listing", got)
	}
}

func TestSnippetPickerEmptyRootHint(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snippets") // doesn't exist yet
	host := newTestHost()
	sp := NewSnippetPicker(host, snippet.NewStore(root))
	sp.Show(func(snippet.Snippet) {}, func() {})

	items := strings.Join(pickerItems(sp), "\n")
	if !strings.Contains(items, "No snippets yet") || !strings.Contains(items, root) {
		t.Errorf("items = %q, want the empty hint naming %q", items, root)
	}
	// Enter on a hint row does nothing.
	pickerSelect(t, sp, "No snippets yet.")
	if !sp.Visible() {
		t.Error("Enter on the hint closed the picker")
	}
}

func TestSnippetPickerUnavailableStore(t *testing.T) {
	sp := NewSnippetPicker(newTestHost(), snippet.NewStore(""))
	sp.Show(func(snippet.Snippet) {}, func() {})
	if items := pickerItems(sp); len(items) != 1 || !strings.Contains(items[0], "unavailable") {
		t.Errorf("items = %q, want a single unavailable error row", items)
	}
}

func TestSnippetPickerLoadSelects(t *testing.T) {
	sp, host, _ := newPickerFixture(t)
	var events []string
	var got snippet.Snippet
	sp.Show(
		func(sn snippet.Snippet) { events = append(events, "select"); got = sn },
		func() { events = append(events, "close") },
	)

	pickerSelect(t, sp, "orders/")
	pickerSelect(t, sp, "created.json")

	if want := (snippet.Snippet{JMSType: "OrderCreated", Body: `{"id":1}`}); got != want {
		t.Errorf("selected = %#v, want %#v", got, want)
	}
	if want := []string{"close", "select"}; !reflect.DeepEqual(events, want) {
		t.Errorf("callback order = %q, want %q", events, want)
	}
	if sp.Visible() {
		t.Error("picker still visible after loading")
	}
	if len(host.hiddenPages) != 1 || host.hiddenPages[0] != "snippet-picker" {
		t.Errorf("hiddenPages = %q, want [snippet-picker]", host.hiddenPages)
	}
}

func TestSnippetPickerParseErrorStaysOpen(t *testing.T) {
	sp, host, root := newPickerFixture(t)
	writeSnippetFile(t, root, "broken.txt", "---\njmsType: T\nno closing line")
	selected, closed := false, false
	sp.Show(func(snippet.Snippet) { selected = true }, func() { closed = true })

	pickerSelect(t, sp, "broken.txt")

	if selected || closed || !sp.Visible() {
		t.Errorf("selected = %v, closed = %v, visible = %v; want false, false, true", selected, closed, sp.Visible())
	}
	if !strings.Contains(host.status, "broken.txt") || !strings.Contains(host.status, "[red]") {
		t.Errorf("status = %q, want a red error naming broken.txt", host.status)
	}
}

func TestSnippetPickerEscCloses(t *testing.T) {
	sp, _, _ := newPickerFixture(t)
	selected, closed := false, false
	sp.Show(func(snippet.Snippet) { selected = true }, func() { closed = true })

	pickerKey(sp, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if !closed || selected || sp.Visible() {
		t.Errorf("closed = %v, selected = %v, visible = %v; want true, false, false", closed, selected, sp.Visible())
	}
}

func TestSnippetPickerJKMoveSelection(t *testing.T) {
	sp, _, _ := newPickerFixture(t)
	sp.Show(func(snippet.Snippet) {}, func() {})

	if ev := sp.list.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)); ev.Key() != tcell.KeyDown {
		t.Errorf("j mapped to %v, want KeyDown", ev.Key())
	}
	if ev := sp.list.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone)); ev.Key() != tcell.KeyUp {
		t.Errorf("k mapped to %v, want KeyUp", ev.Key())
	}
}

func TestSnippetPickerEscapesTagLikeNames(t *testing.T) {
	root := t.TempDir()
	writeSnippetFile(t, root, "[red]order.json", "x")
	sp := NewSnippetPicker(newTestHost(), snippet.NewStore(root))
	sp.Show(func(snippet.Snippet) {}, func() {})

	text := renderedScreenText(t, sp.Primitive(), 40, 5)
	if !strings.Contains(text, "[red]order.json") {
		t.Errorf("rendered picker %q doesn't show the literal name [red]order.json", text)
	}
}
