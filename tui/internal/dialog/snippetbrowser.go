package dialog

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/snippet"
)

// SnippetBrowser is a folder-by-folder list of the snippet library,
// shared by the send dialog's SnippetPicker and the Snippets view:
// folders first ("name/"), then snippets, with ".." at the top of any
// subfolder. Enter opens a folder or chooses a snippet; ".." and
// Backspace go up one level (never above the root), keeping the cursor
// on the folder just left; j/k move the cursor.
type SnippetBrowser struct {
	store     *snippet.Store
	list      *tview.List
	dir       string      // current folder, relative to the store root ("" = root)
	rows      []browseRow // one per list item, in order
	emptyHint []string    // extra lines after the standard empty-library hint
	keys      func(*tcell.EventKey) *tcell.EventKey
	onChosen  func(rel string)
	onChanged func()
}

// browseRow is what one list item stands for.
type browseRow struct {
	kind  rowKind
	rel   string // folder or snippet path relative to the root (entries only)
	entry snippet.Entry
}

type rowKind int

const (
	rowInfo  rowKind = iota // a hint or error line: not selectable as an entry
	rowUp                   // ".."
	rowEntry                // a folder or snippet
)

// NewSnippetBrowser builds the list over store, positioned at the root.
// Call Reload (or SetDir) to fill it.
func NewSnippetBrowser(store *snippet.Store) *SnippetBrowser {
	b := &SnippetBrowser{store: store}
	b.list = tview.NewList().ShowSecondaryText(false)
	b.list.SetBorder(true)
	b.list.SetChangedFunc(func(int, string, string, rune) { b.changed() })
	b.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if b.keys != nil {
			if event = b.keys(event); event == nil {
				return nil
			}
		}
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			b.Up()
			return nil
		}
		return event
	})
	return b
}

// List returns the underlying list, for layout, focus, and styling.
func (b *SnippetBrowser) List() *tview.List { return b.list }

// SetKeys installs the owner's own key handling. It runs before the
// browser's (j/k/Backspace) and can consume an event by returning nil.
func (b *SnippetBrowser) SetKeys(fn func(*tcell.EventKey) *tcell.EventKey) {
	b.keys = fn
}

// SetChosenFunc sets what Enter on a snippet does; rel is its path
// relative to the store root.
func (b *SnippetBrowser) SetChosenFunc(fn func(rel string)) { b.onChosen = fn }

// SetChangedFunc sets a callback run whenever the item under the cursor
// may have changed: the cursor moved, or the list was rebuilt.
func (b *SnippetBrowser) SetChangedFunc(fn func()) { b.onChanged = fn }

// SetEmptyHint adds lines shown after the standard "No snippets yet"
// hint when the library is empty.
func (b *SnippetBrowser) SetEmptyHint(lines ...string) { b.emptyHint = lines }

// Dir returns the current folder, relative to the store root ("" = root).
func (b *SnippetBrowser) Dir() string { return b.dir }

// SetDir opens dir (relative to the store root; "" = root) with the
// cursor on its first row.
func (b *SnippetBrowser) SetDir(dir string) {
	b.dir = dir
	b.fill("")
}

// Reload re-reads the current folder from disk, keeping the cursor on
// the same entry when it still exists. If the folder itself has gone
// (e.g. deleted outside the app), it falls back to the nearest parent
// that still exists.
func (b *SnippetBrowser) Reload() {
	var keep string
	if _, e, ok := b.Selected(); ok {
		keep = e.Name
	}
	for b.dir != "" {
		if _, err := b.store.Stat(b.dir); !errors.Is(err, fs.ErrNotExist) {
			break
		}
		keep = ""
		b.dir = parentDir(b.dir)
	}
	b.fill(keep)
}

// Select puts the cursor on the entry called name in the current folder,
// reporting whether it was found.
func (b *SnippetBrowser) Select(name string) bool {
	for i, r := range b.rows {
		if r.kind == rowEntry && r.entry.Name == name {
			b.list.SetCurrentItem(i)
			b.changed()
			return true
		}
	}
	return false
}

// Selected returns the folder or snippet under the cursor and its path
// relative to the store root. ok is false on "..", hint, and error rows.
func (b *SnippetBrowser) Selected() (rel string, e snippet.Entry, ok bool) {
	i := b.list.GetCurrentItem()
	if i < 0 || i >= len(b.rows) || b.rows[i].kind != rowEntry {
		return "", snippet.Entry{}, false
	}
	return b.rows[i].rel, b.rows[i].entry, true
}

// Up goes to the parent folder, keeping the cursor on the folder just
// left. A no-op at the root — the browser never leaves the snippets
// folder.
func (b *SnippetBrowser) Up() {
	if b.dir == "" {
		return
	}
	left := filepath.Base(b.dir)
	b.dir = parentDir(b.dir)
	b.fill(left)
}

// fill rebuilds the list for the current folder: ".." (below the root),
// then folders ("name/"), then snippets. Names are tview-escaped, since
// a "[" in a file name would otherwise be read as a color tag.
// selectName, if non-empty, is the entry to put the cursor on.
func (b *SnippetBrowser) fill(selectName string) {
	b.list.Clear()
	b.rows = b.rows[:0]
	title := " Snippets "
	if b.dir != "" {
		title = fmt.Sprintf(" Snippets — /%s ", tview.Escape(filepath.ToSlash(b.dir)))
	}
	b.list.SetTitle(title)

	if b.dir != "" {
		b.add(browseRow{kind: rowUp}, "..", b.Up)
	}

	entries, err := b.store.List(b.dir)
	switch {
	case err != nil:
		b.add(browseRow{}, fmt.Sprintf("Error: %s", tview.Escape(err.Error())), nil)
	case b.dir == "" && len(entries) == 0:
		// Separate rows rather than one: tview.List doesn't wrap, and the
		// folder path would be cut off at the widget's width.
		b.add(browseRow{}, "No snippets yet.", nil)
		b.add(browseRow{}, "Save one from a message (S), or add files to:", nil)
		b.add(browseRow{}, tview.Escape(b.store.Root()), nil)
		for _, line := range b.emptyHint {
			b.add(browseRow{}, line, nil)
		}
	default:
		for _, e := range entries {
			row := browseRow{kind: rowEntry, rel: filepath.Join(b.dir, e.Name), entry: e}
			if e.IsDir {
				b.add(row, tview.Escape(e.Name)+"/", func() { b.SetDir(row.rel) })
			} else {
				b.add(row, tview.Escape(e.Name), func() {
					if b.onChosen != nil {
						b.onChosen(row.rel)
					}
				})
			}
			if e.Name == selectName {
				b.list.SetCurrentItem(b.list.GetItemCount() - 1)
			}
		}
	}
	b.changed()
}

// add appends one list item and its row.
func (b *SnippetBrowser) add(row browseRow, text string, selected func()) {
	b.rows = append(b.rows, row)
	b.list.AddItem(text, "", 0, selected)
}

// changed runs the owner's changed callback, if any.
func (b *SnippetBrowser) changed() {
	if b.onChanged != nil {
		b.onChanged()
	}
}

// parentDir returns dir's parent, relative to the root ("" = root).
func parentDir(dir string) string {
	if parent := filepath.Dir(dir); parent != "." {
		return parent
	}
	return ""
}
