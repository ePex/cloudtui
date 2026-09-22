package view

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SnippetsView is the snippet library: a folder-by-folder list of
// ~/.cloudtui/snippets/ on the left, a preview of the entry under the
// cursor on the right, and keys to create, edit, rename/move, and delete
// snippets and folders. It re-reads the folder every time it's opened.
type SnippetsView struct {
	host    ui.Host
	store   *snippet.Store
	confirm *dialog.ConfirmDialog
	editor  *dialog.SnippetEditor
	prompt  *dialog.TextPrompt
	browser *dialog.SnippetBrowser
	preview *tview.TextView
	flex    *tview.Flex
}

var (
	_ ui.View          = (*SnippetsView)(nil)
	_ ui.Shortcuttable = (*SnippetsView)(nil)
	_ ui.Themeable     = (*SnippetsView)(nil)
)

// NewSnippetsView builds the library view over store. editor handles new
// and edited snippets, prompt asks for folder names and rename/move
// paths, and confirm asks before deleting.
func NewSnippetsView(host ui.Host, store *snippet.Store, confirm *dialog.ConfirmDialog, editor *dialog.SnippetEditor, prompt *dialog.TextPrompt) *SnippetsView {
	v := &SnippetsView{host: host, store: store, confirm: confirm, editor: editor, prompt: prompt}
	v.browser = dialog.NewSnippetBrowser(store)
	v.browser.SetEmptyHint("Or press n to create one here.")
	v.browser.SetChosenFunc(v.edit)
	v.browser.SetChangedFunc(v.updatePreview)
	v.browser.SetKeys(v.handleKey)

	v.preview = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	v.preview.SetBorder(true).SetTitle(" Preview ")

	v.flex = tview.NewFlex().
		AddItem(v.browser.List(), 0, 2, true).
		AddItem(v.preview, 0, 3, false)
	v.browser.SetDir("")
	return v
}

func (v *SnippetsView) Name() string               { return "snippets" }
func (v *SnippetsView) Title() string              { return "Snippets" }
func (v *SnippetsView) Primitive() tview.Primitive { return v.flex }

// Shortcuts lists the view's keys for the top bar's context panel.
func (v *SnippetsView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "Enter", Description: "open folder / edit"},
		{Key: "n", Description: "new snippet"},
		{Key: "N", Description: "new folder"},
		{Key: "e", Description: "edit"},
		{Key: "R", Description: "rename / move"},
		{Key: "d", Description: "delete"},
		{Key: "r", Description: "refresh"},
		{Key: "Backspace", Description: "up a folder"},
	}
}

// Activate re-reads the library from disk each time the view is opened,
// since files may change outside the app (e.g. a git pull in a shared
// folder).
func (v *SnippetsView) Activate() {
	v.browser.Reload()
}

// handleKey runs the view's keys before the browser's own (j/k,
// Backspace). Keys that act on an entry do nothing on "..", hint, and
// error rows.
func (v *SnippetsView) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	rel, e, ok := v.browser.Selected()
	switch event.Rune() {
	case 'n':
		v.editor.ShowNew(v.browser.Dir(), v.landOn, v.restoreFocus)
	case 'N':
		v.newFolder()
	case 'r':
		v.browser.Reload()
	case 'e':
		if ok && !e.IsDir {
			v.edit(rel)
		}
	case 'R':
		if ok {
			v.rename(rel)
		}
	case 'd':
		if ok {
			v.delete(rel, e)
		}
	default:
		return event
	}
	return nil
}

// edit opens the snippet at rel in the editor. A file that can't be read
// or parsed is reported instead.
func (v *SnippetsView) edit(rel string) {
	sn, err := v.store.Load(rel)
	if err != nil {
		v.showError(err)
		return
	}
	if formatted := snippet.FormatBody(sn.Body); formatted != sn.Body {
		sn.Body = formatted
		if err := v.store.Save(rel, sn, true); err != nil {
			v.showError(err)
			return
		}
	}
	v.editor.ShowEdit(rel, sn, v.landOn, v.restoreFocus)
}

// newFolder asks for a folder name (relative to the current folder) and
// creates it, nested levels included.
func (v *SnippetsView) newFolder() {
	dir := v.browser.Dir()
	v.prompt.Show("New folder", "Name:", "", func(text string) error {
		// Validated as typed before joining, so ".." can't clean away.
		name, err := snippet.ValidateName(text)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, name)
		if err := v.store.MkDir(path); err != nil {
			return err
		}
		v.host.SetStatus(fmt.Sprintf("Folder created: %s", tview.Escape(filepath.ToSlash(path))))
		v.landOn(path)
		return nil
	}, v.restoreFocus)
}

// rename asks for a new path (relative to the snippets root, prefilled
// with the current one) and moves the entry there. Nothing is
// overwritten; a folder can't be moved into itself.
func (v *SnippetsView) rename(rel string) {
	v.prompt.Show("Rename / move", "Path:", filepath.ToSlash(rel), func(text string) error {
		to, err := snippet.ValidateName(text)
		if err != nil {
			return err
		}
		if to == filepath.Clean(rel) {
			return nil // unchanged: nothing to do
		}
		if err := v.store.Move(rel, to); err != nil {
			return err
		}
		v.host.SetStatus(fmt.Sprintf("Moved %s to %s", tview.Escape(filepath.ToSlash(rel)), tview.Escape(filepath.ToSlash(to))))
		v.landOn(to)
		return nil
	}, v.restoreFocus)
}

// delete asks before deleting the entry at rel: a snippet, a symlinked
// folder (only the link goes), an empty folder, or a folder with its
// nested contents counted in the question.
func (v *SnippetsView) delete(rel string, e snippet.Entry) {
	name := filepath.ToSlash(rel)
	var question string
	var remove func() error
	switch {
	case !e.IsDir:
		question = fmt.Sprintf("Delete snippet %q?", name)
		remove = func() error { return v.store.Delete(rel) }
	case e.IsLink:
		question = fmt.Sprintf("Remove link %q? The folder it points to is kept.", name)
		remove = func() error { return v.store.DeleteFolder(rel) }
	default:
		snippets, folders, err := v.store.Count(rel)
		if err != nil {
			v.showError(err)
			return
		}
		if snippets == 0 && folders == 0 {
			question = fmt.Sprintf("Delete empty folder %q?", name)
		} else {
			question = fmt.Sprintf("Delete folder %q and its %s?", name, countText(snippets, folders))
		}
		remove = func() error { return v.store.DeleteFolder(rel) }
	}

	v.confirm.ShowWithCancel(question, func() {
		if err := remove(); err != nil {
			v.showError(err)
		} else {
			v.host.SetStatus(fmt.Sprintf("Deleted %s", tview.Escape(name)))
		}
		v.browser.Reload()
		v.restoreFocus()
	}, v.restoreFocus)
}

// landOn shows the folder path is in, with the cursor on it — after a
// snippet or folder was created, saved, or moved there.
func (v *SnippetsView) landOn(path string) {
	dir := filepath.Dir(path)
	if dir == "." {
		dir = ""
	}
	v.browser.SetDir(dir)
	v.browser.Select(filepath.Base(path))
}

// updatePreview shows the entry under the cursor: a snippet's JMS Type
// and raw body (or why it can't be read), a folder's nested counts, or
// that a folder is a link.
func (v *SnippetsView) updatePreview() {
	rel, e, ok := v.browser.Selected()
	if !ok {
		v.preview.SetText("")
		return
	}
	p := v.host.Config().Colors
	switch {
	case e.IsDir && e.IsLink:
		v.preview.SetText(fmt.Sprintf("[%s]Linked folder[-]\n\nIts contents aren't counted here, and deleting it removes only the link.", p.Label))
	case e.IsDir:
		snippets, folders, err := v.store.Count(rel)
		if err != nil {
			v.preview.SetText(fmt.Sprintf("[red]%s[-]", tview.Escape(err.Error())))
			return
		}
		text := "Empty folder"
		if snippets > 0 || folders > 0 {
			text = countText(snippets, folders)
		}
		v.preview.SetText(fmt.Sprintf("[%s]Folder[-]\n\n%s", p.Label, text))
	default:
		sn, err := v.store.Load(rel)
		if err != nil {
			v.preview.SetText(fmt.Sprintf("[red]%s[-]", tview.Escape(err.Error())))
			return
		}
		jmsType := sn.JMSType
		if jmsType == "" {
			jmsType = "(none)"
		}
		body := snippet.FormatBody(sn.Body)
		v.preview.SetText(fmt.Sprintf("[%s]JMS Type:[-] %s\n\n%s", p.Label, tview.Escape(jmsType), tview.Escape(body)))
	}
	v.preview.ScrollToBeginning()
}

// countText describes nested contents, e.g. "3 snippets, 1 subfolder",
// leaving out a zero count.
func countText(snippets, folders int) string {
	var parts []string
	if snippets > 0 {
		parts = append(parts, plural(snippets, "snippet"))
	}
	if folders > 0 {
		parts = append(parts, plural(folders, "subfolder"))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// showError reports err in the status bar.
func (v *SnippetsView) showError(err error) {
	v.host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", tview.Escape(err.Error())))
}

// restoreFocus gives focus and the shortcut hint back to the list after
// an overlay raised from the view (editor, prompt, confirmation) closes.
func (v *SnippetsView) restoreFocus() {
	v.host.SetFocus(v.browser.List())
	lines := make([]string, 0, len(v.Shortcuts()))
	for _, sc := range v.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", v.host.Config().Colors.Accent, sc.Key, sc.Description))
	}
	v.host.SetContextHint(strings.Join(lines, "\n"))
}

// ApplyPalette recolors the view for a live theme switch (and once at
// startup).
func (v *SnippetsView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	viewColor := tcell.GetColor(p.ViewColor("snippets"))
	list := v.browser.List()
	ui.StyleList(list, p)
	list.SetBackgroundColor(bg)
	list.SetBorderColor(viewColor)
	list.SetTitleColor(viewColor)
	v.preview.SetBackgroundColor(bg)
	v.preview.SetBorderColor(viewColor)
	v.preview.SetTitleColor(viewColor)
	// Untagged preview text (the body) is drawn in the text view's base
	// color, copied at construction.
	v.preview.SetTextColor(tcell.GetColor(p.Text))
	v.updatePreview()
}
