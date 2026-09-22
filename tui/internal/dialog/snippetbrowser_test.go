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

// browserItems returns the browser list's item texts in order.
func browserItems(b *SnippetBrowser) []string {
	items := make([]string, b.List().GetItemCount())
	for i := range items {
		items[i], _ = b.List().GetItemText(i)
	}
	return items
}

// browserSelect puts the cursor on the item with text and presses Enter.
func browserSelect(t *testing.T, b *SnippetBrowser, text string) {
	t.Helper()
	for i, item := range browserItems(b) {
		if item == text {
			b.List().SetCurrentItem(i)
			b.List().InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
			return
		}
	}
	t.Fatalf("no item %q in %q", text, browserItems(b))
}

// newBrowserFixture builds a browser over root/{orders/created.json,
// orders/eu/, ping.txt}, opened at the root.
func newBrowserFixture(t *testing.T) (*SnippetBrowser, string) {
	t.Helper()
	root := t.TempDir()
	writeSnippetFile(t, root, "orders/created.json", "x")
	writeSnippetFile(t, root, "orders/eu/updated.json", "x")
	writeSnippetFile(t, root, "ping.txt", "p")
	b := NewSnippetBrowser(snippet.NewStore(root))
	b.SetDir("")
	return b, root
}

func TestSnippetBrowserSelected(t *testing.T) {
	b, _ := newBrowserFixture(t)

	b.List().SetCurrentItem(0)
	if rel, e, ok := b.Selected(); !ok || rel != "orders" || !e.IsDir {
		t.Errorf("Selected on orders/ = (%q, %#v, %v), want (orders, a folder, true)", rel, e, ok)
	}

	browserSelect(t, b, "orders/")
	b.List().SetCurrentItem(0) // ".."
	if _, _, ok := b.Selected(); ok {
		t.Error(`Selected on ".." reported an entry`)
	}
	if !b.Select("created.json") {
		t.Fatal("Select(created.json) = false")
	}
	if rel, e, ok := b.Selected(); !ok || rel != filepath.Join("orders", "created.json") || e.IsDir {
		t.Errorf("Selected = (%q, %#v, %v), want (orders/created.json, a snippet, true)", rel, e, ok)
	}
	if b.Select("missing.json") {
		t.Error("Select(missing.json) = true")
	}
}

func TestSnippetBrowserSelectedOnHintRows(t *testing.T) {
	b := NewSnippetBrowser(snippet.NewStore(filepath.Join(t.TempDir(), "none")))
	b.SetDir("")
	for i := 0; i < b.List().GetItemCount(); i++ {
		b.List().SetCurrentItem(i)
		if _, _, ok := b.Selected(); ok {
			t.Errorf("hint row %d reported as an entry", i)
		}
	}
}

func TestSnippetBrowserSetDirAndDir(t *testing.T) {
	b, _ := newBrowserFixture(t)
	b.SetDir(filepath.Join("orders", "eu"))
	if got := b.Dir(); got != filepath.Join("orders", "eu") {
		t.Errorf("Dir = %q", got)
	}
	if want := []string{"..", "updated.json"}; !reflect.DeepEqual(browserItems(b), want) {
		t.Errorf("items = %q, want %q", browserItems(b), want)
	}
	if !strings.Contains(b.List().GetTitle(), "/orders/eu") {
		t.Errorf("title = %q", b.List().GetTitle())
	}
}

func TestSnippetBrowserChosenFunc(t *testing.T) {
	b, _ := newBrowserFixture(t)
	var chosen string
	b.SetChosenFunc(func(rel string) { chosen = rel })

	browserSelect(t, b, "orders/")
	browserSelect(t, b, "created.json")
	if chosen != filepath.Join("orders", "created.json") {
		t.Errorf("chosen = %q", chosen)
	}
}

func TestSnippetBrowserChangedFunc(t *testing.T) {
	b, _ := newBrowserFixture(t)
	var seen []string
	b.SetChangedFunc(func() {
		if _, e, ok := b.Selected(); ok {
			seen = append(seen, e.Name)
		} else {
			seen = append(seen, "-")
		}
	})

	b.List().InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(tview.Primitive) {})
	if len(seen) == 0 || seen[len(seen)-1] != "ping.txt" {
		t.Errorf("after moving down, last changed = %q, want ping.txt", seen)
	}
	seen = nil
	b.Reload()
	if len(seen) == 0 || seen[len(seen)-1] != "ping.txt" {
		t.Errorf("after Reload, last changed = %q, want ping.txt (cursor kept)", seen)
	}
}

func TestSnippetBrowserReload(t *testing.T) {
	b, root := newBrowserFixture(t)
	b.Select("ping.txt")
	writeSnippetFile(t, root, "added.txt", "x")

	b.Reload()
	if want := []string{"orders/", "added.txt", "ping.txt"}; !reflect.DeepEqual(browserItems(b), want) {
		t.Errorf("items after Reload = %q, want %q", browserItems(b), want)
	}
	if _, e, _ := b.Selected(); e.Name != "ping.txt" {
		t.Errorf("cursor after Reload on %q, want ping.txt", e.Name)
	}
}

// TestSnippetBrowserReloadFallsBackWhenFolderIsGone covers the folder
// being deleted outside the app while it's open.
func TestSnippetBrowserReloadFallsBackWhenFolderIsGone(t *testing.T) {
	b, root := newBrowserFixture(t)
	b.SetDir(filepath.Join("orders", "eu"))
	if err := os.RemoveAll(filepath.Join(root, "orders")); err != nil {
		t.Fatal(err)
	}

	b.Reload()
	if b.Dir() != "" {
		t.Errorf("Dir after Reload = %q, want the root", b.Dir())
	}
	if want := []string{"ping.txt"}; !reflect.DeepEqual(browserItems(b), want) {
		t.Errorf("items = %q, want %q", browserItems(b), want)
	}
}

func TestSnippetBrowserEmptyHint(t *testing.T) {
	b := NewSnippetBrowser(snippet.NewStore(filepath.Join(t.TempDir(), "none")))
	b.SetEmptyHint("Or press n to create one.")
	b.SetDir("")
	items := browserItems(b)
	if len(items) == 0 || items[len(items)-1] != "Or press n to create one." {
		t.Errorf("items = %q, want the extra hint line last", items)
	}
}

func TestSnippetBrowserOwnerKeysRunFirst(t *testing.T) {
	b, _ := newBrowserFixture(t)
	var owner []rune
	b.SetKeys(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'j' {
			owner = append(owner, 'j')
			return nil // consumed: the browser must not also move the cursor
		}
		return event
	})

	if ev := b.List().GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)); ev != nil {
		t.Errorf("owner-consumed j still returned %v", ev)
	}
	if len(owner) != 1 {
		t.Errorf("owner handler calls = %d, want 1", len(owner))
	}
	if ev := b.List().GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone)); ev == nil || ev.Key() != tcell.KeyUp {
		t.Errorf("k mapped to %v, want KeyUp (browser handling after the owner passes)", ev)
	}
}
