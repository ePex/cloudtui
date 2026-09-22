package dialog

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SnippetPicker is the "Snippets" overlay: a SnippetBrowser of the
// snippet library, used to pick a snippet to load into the send-message
// dialog.
type SnippetPicker struct {
	host     ui.Host
	store    *snippet.Store
	browser  *SnippetBrowser
	list     *tview.List // browser.List(), kept for layout and focus
	onSelect func(snippet.Snippet)
	onClose  func()
	visible  bool
}

// NewSnippetPicker builds the snippet-picker overlay's widgets.
func NewSnippetPicker(host ui.Host, store *snippet.Store) *SnippetPicker {
	sp := &SnippetPicker{host: host, store: store, browser: NewSnippetBrowser(store)}
	sp.list = sp.browser.List()
	sp.browser.SetChosenFunc(sp.load)
	sp.browser.SetKeys(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
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
	sp.browser.SetDir("")

	sp.host.ShowPage("snippet-picker")
	sp.host.SetFocus(sp.list)
	sp.visible = true
	ac := sp.host.Config().Colors.Accent
	sp.host.SetContextHint(fmt.Sprintf("[%s]<Enter>[-] open/load  [%s]<Backspace>[-] up  [%s]<Esc>[-] cancel", ac, ac, ac))
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
