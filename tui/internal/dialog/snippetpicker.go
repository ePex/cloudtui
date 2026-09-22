package dialog

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SnippetPicker is the "Snippets" overlay: a folder-by-folder browser of
// the snippet library, used to pick a snippet to load into the
// send-message dialog.
type SnippetPicker struct {
	host     ui.Host
	store    *snippet.Store
	list     *tview.List
	dir      string // current folder, relative to the store root ("" = root)
	onSelect func(snippet.Snippet)
	onClose  func()
	visible  bool
}

// NewSnippetPicker builds the snippet-picker overlay's widgets.
func NewSnippetPicker(host ui.Host, store *snippet.Store) *SnippetPicker {
	sp := &SnippetPicker{host: host, store: store}
	sp.list = tview.NewList().ShowSecondaryText(false)
	sp.list.SetBorder(true)
	sp.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			sp.up()
			return nil
		case event.Key() == tcell.KeyEscape:
			sp.close()
			return nil
		}
		return event
	})
	return sp
}

// Show opens the picker at the snippets root. onSelect is called (on the
// UI goroutine, after the picker has closed and onClose has run) with the
// loaded snippet. onClose is called whenever the picker is dismissed —
// the caller uses it to restore focus and the context panel.
func (sp *SnippetPicker) Show(onSelect func(snippet.Snippet), onClose func()) {
	sp.onSelect = onSelect
	sp.onClose = onClose
	sp.dir = ""
	sp.fill("")

	sp.host.ShowPage("snippet-picker")
	sp.host.SetFocus(sp.list)
	sp.visible = true
	ac := sp.host.Config().Colors.Accent
	sp.host.SetContextHint(fmt.Sprintf("[%s]<Enter>[-] open/load  [%s]<Backspace>[-] up  [%s]<Esc>[-] cancel", ac, ac, ac))
}

// fill rebuilds the list for the current folder: ".." (below the root),
// then folders ("name/"), then snippets. Names are tview-escaped, since
// a "[" in a file name would otherwise be read as a color tag.
// selectName, if non-empty, is the entry to put the cursor on (the
// folder just left when going up).
func (sp *SnippetPicker) fill(selectName string) {
	sp.list.Clear()
	title := " Snippets "
	if sp.dir != "" {
		title = fmt.Sprintf(" Snippets — /%s ", tview.Escape(filepath.ToSlash(sp.dir)))
	}
	sp.list.SetTitle(title)

	if sp.dir != "" {
		sp.list.AddItem("..", "", 0, sp.up)
	}

	entries, err := sp.store.List(sp.dir)
	if err != nil {
		sp.list.AddItem(fmt.Sprintf("Error: %s", tview.Escape(err.Error())), "", 0, nil)
		return
	}
	if sp.dir == "" && len(entries) == 0 {
		// Three rows rather than one: tview.List doesn't wrap, and the
		// folder path would be cut off at the overlay's width.
		sp.list.AddItem("No snippets yet.", "", 0, nil)
		sp.list.AddItem("Save one from a message (S), or add files to:", "", 0, nil)
		sp.list.AddItem(tview.Escape(sp.store.Root()), "", 0, nil)
		return
	}

	for _, e := range entries {
		rel := filepath.Join(sp.dir, e.Name)
		if e.IsDir {
			sp.list.AddItem(tview.Escape(e.Name)+"/", "", 0, func() { sp.enter(rel) })
		} else {
			sp.list.AddItem(tview.Escape(e.Name), "", 0, func() { sp.load(rel) })
		}
		if e.Name == selectName {
			sp.list.SetCurrentItem(sp.list.GetItemCount() - 1)
		}
	}
}

// enter opens the folder rel (relative to the store root).
func (sp *SnippetPicker) enter(rel string) {
	sp.dir = rel
	sp.fill("")
}

// up goes to the parent folder, keeping the cursor on the folder just
// left. A no-op at the root — the picker never leaves the snippets folder.
func (sp *SnippetPicker) up() {
	if sp.dir == "" {
		return
	}
	left := filepath.Base(sp.dir)
	sp.dir = filepath.Dir(sp.dir)
	if sp.dir == "." {
		sp.dir = ""
	}
	sp.fill(left)
}

// load reads the snippet rel. A read or parse error is reported in the
// status bar and the picker stays open; on success the picker closes and
// hands the snippet to onSelect.
func (sp *SnippetPicker) load(rel string) {
	sn, err := sp.store.Load(rel)
	if err != nil {
		sp.host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", tview.Escape(err.Error())))
		return
	}
	onSelect := sp.onSelect
	sp.close()
	if onSelect != nil {
		onSelect(sn)
	}
}

// close hides the picker and calls onClose to let the caller restore
// focus and the context panel.
func (sp *SnippetPicker) close() {
	sp.host.HidePage("snippet-picker")
	sp.visible = false
	if sp.onClose != nil {
		sp.onClose()
	}
}

// ApplyPalette recolors the snippet picker for a live theme switch.
func (sp *SnippetPicker) ApplyPalette(p config.Palette) {
	ui.StyleList(sp.list, p)
	sp.list.SetBackgroundColor(tcell.GetColor(p.Background))
	sp.list.SetBorderColor(tcell.GetColor(p.Border))
	sp.list.SetTitleColor(tcell.GetColor(p.Border))
}

var _ ui.Themeable = (*SnippetPicker)(nil)

// Primitive returns SnippetPicker's root widget, for sizing/embedding.
func (sp *SnippetPicker) Primitive() tview.Primitive { return sp.list }

// Visible reports whether SnippetPicker is currently shown.
func (sp *SnippetPicker) Visible() bool { return sp.visible }
