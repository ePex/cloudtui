package dialog

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SnippetEditor is the "New / Edit Snippet" overlay: Name, JMS Type, and
// a multi-line Body, plus Save/Cancel. New snippets are created in a
// given folder; editing keeps the snippet's front-matter keys the app
// doesn't know (snippet.Snippet.Extra) and renames it when its Name
// changes.
type SnippetEditor struct {
	host        ui.Host
	store       *snippet.Store
	confirm     *ConfirmDialog
	form        *tview.Form
	nameItem    *tview.InputField
	jmsTypeItem *tview.InputField
	bodyItem    *tview.TextArea

	dir      string          // folder the Name is relative to ("" = root)
	origPath string          // the edited snippet's path; "" for a new one
	orig     snippet.Snippet // the edited snippet as loaded (for Extra)
	initial  [3]string       // Name, JMS Type, Body as opened, for the unsaved-changes check
	onSaved  func(path string)
	onClose  func()
	visible  bool
}

// NewSnippetEditor builds the editor's widgets. confirm asks before an
// existing snippet is overwritten and before unsaved changes are
// discarded.
func NewSnippetEditor(host ui.Host, store *snippet.Store, confirm *ConfirmDialog) *SnippetEditor {
	se := &SnippetEditor{host: host, store: store, confirm: confirm}
	se.form = tview.NewForm()
	se.form.SetBorder(true)
	se.form.
		AddInputField("Name", "", 60, nil, nil).
		AddInputField("JMS Type", "", 60, nil, nil).
		AddTextArea("Body", "", 0, 14, 0, nil).
		AddButton("Save", se.save).
		AddButton("Cancel", se.cancel)
	se.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			se.cancel()
			return nil
		}
		return event
	})
	se.nameItem = se.form.GetFormItem(0).(*tview.InputField)
	se.jmsTypeItem = se.form.GetFormItem(1).(*tview.InputField)
	se.bodyItem = se.form.GetFormItem(2).(*tview.TextArea)

	// Enter in Name or JMS Type saves; in Body it inserts a newline, as a
	// TextArea does. Captured (and swallowed) rather than via
	// SetDoneFunc: tview.Form moves focus on to the next element right
	// after a done func, which would steal focus from the overwrite
	// confirmation or from the field after an error (see tui/CLAUDE.md's
	// tview gotchas).
	for _, field := range []*tview.InputField{se.nameItem, se.jmsTypeItem} {
		field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEnter {
				se.save()
				return nil
			}
			return event
		})
	}
	return se
}

// ShowNew opens the editor for a new snippet in dir (relative to the
// store root; "" = root), all fields empty. onSaved gets the saved
// snippet's path after the editor has closed; onClose is called whenever
// the editor is dismissed (saved or cancelled).
func (se *SnippetEditor) ShowNew(dir string, onSaved func(path string), onClose func()) {
	se.open(dir, "", snippet.Snippet{}, "", onSaved, onClose)
	se.form.SetTitle(fmt.Sprintf(" New Snippet — /%s ", tview.Escape(filepath.ToSlash(dir))))
}

// ShowEdit opens the editor on the snippet sn stored at path (relative to
// the store root). Its Name is shown relative to its own folder.
func (se *SnippetEditor) ShowEdit(path string, sn snippet.Snippet, onSaved func(path string), onClose func()) {
	se.open(parentDir(path), path, sn, filepath.Base(path), onSaved, onClose)
	se.form.SetTitle(fmt.Sprintf(" Edit Snippet — %s ", tview.Escape(filepath.ToSlash(path))))
}

func (se *SnippetEditor) open(dir, origPath string, sn snippet.Snippet, name string, onSaved func(string), onClose func()) {
	se.dir, se.origPath, se.orig = dir, origPath, sn
	se.onSaved, se.onClose = onSaved, onClose
	ui.SetInputFieldText(se.nameItem, name)
	ui.SetInputFieldText(se.jmsTypeItem, sn.JMSType)
	se.bodyItem.SetText(sn.Body, false)
	se.initial = se.values()
	se.host.ShowPage("snippet-editor")
	se.visible = true
	se.form.SetFocus(0)
	se.refocus()
}

// values returns the fields' current Name, JMS Type, and Body.
func (se *SnippetEditor) values() [3]string {
	return [3]string{se.nameItem.GetText(), se.jmsTypeItem.GetText(), se.bodyItem.GetText()}
}

// refocus gives focus and the context hint back to the editor. The form
// remembers which field or button had focus, so after a declined
// confirmation you're back where you were.
func (se *SnippetEditor) refocus() {
	se.host.SetFocus(se.form)
	ac := se.host.Config().Colors.Accent
	se.host.SetContextHint(fmt.Sprintf("[%s]<Tab>[-] next field  [%s]<Enter>[-] save (in Name/JMS Type)  [%s]<Esc>[-] cancel", ac, ac, ac))
}

// save validates the Name and writes the snippet. New: an existing name
// asks before overwriting. Edit: a changed Name first moves the snippet
// (never overwriting anything), then writes it. Errors go to the status
// bar and the editor stays open.
func (se *SnippetEditor) save() {
	// The Name is validated on its own first (so "..", absolute paths,
	// and an empty Name are rejected as typed), then placed in the folder.
	name, err := snippet.ValidateName(se.nameItem.GetText())
	if err != nil {
		se.fail(err)
		return
	}
	path := filepath.Join(se.dir, name)
	sn := snippet.Snippet{
		JMSType: strings.TrimSpace(se.jmsTypeItem.GetText()),
		Body:    se.bodyItem.GetText(),
		Extra:   se.orig.Extra,
	}

	if se.origPath == "" {
		err := se.store.Save(path, sn, false)
		if errors.Is(err, snippet.ErrExists) {
			se.confirm.ShowWithCancel(fmt.Sprintf("Overwrite snippet %q?", filepath.ToSlash(path)),
				func() { se.finish(path, se.store.Save(path, sn, true)) },
				se.refocus)
			return
		}
		se.finish(path, err)
		return
	}

	if path != filepath.Clean(se.origPath) {
		if err := se.store.Move(se.origPath, path); err != nil {
			se.fail(err)
			return
		}
		// From here on the snippet lives at its new path, even if the
		// write below fails.
		se.origPath = path
	}
	se.finish(path, se.store.Save(path, sn, true))
}

// finish closes the editor on a successful save and hands the path to
// onSaved, or reports err and keeps the editor open.
func (se *SnippetEditor) finish(path string, err error) {
	if err != nil {
		se.fail(err)
		return
	}
	onSaved := se.onSaved
	se.close()
	se.host.SetStatus(fmt.Sprintf("Snippet saved: %s", tview.Escape(filepath.ToSlash(path))))
	if onSaved != nil {
		onSaved(path)
	}
}

// fail reports err in the status bar and refocuses the Name field.
func (se *SnippetEditor) fail(err error) {
	se.host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", tview.Escape(err.Error())))
	se.form.SetFocus(0)
	se.refocus()
}

// cancel closes the editor, first asking "Discard changes?" when any
// field differs from what it was opened with.
func (se *SnippetEditor) cancel() {
	if se.values() == se.initial {
		se.close()
		return
	}
	se.confirm.ShowWithCancel("Discard changes?", se.close, se.refocus)
}

// close hides the editor and calls onClose to let the caller restore
// focus and the context panel.
func (se *SnippetEditor) close() {
	se.host.HidePage("snippet-editor")
	se.visible = false
	if se.onClose != nil {
		se.onClose()
	}
}

// ApplyPalette recolors the editor for a live theme switch.
func (se *SnippetEditor) ApplyPalette(p config.Palette) {
	se.form.SetBackgroundColor(tcell.GetColor(p.Background))
	se.form.SetBorderColor(tcell.GetColor(p.Border))
	se.form.SetTitleColor(tcell.GetColor(p.Border))
	ui.StyleForm(se.form, p)
}

var _ ui.Themeable = (*SnippetEditor)(nil)

// Primitive returns SnippetEditor's root widget, for sizing/embedding.
func (se *SnippetEditor) Primitive() tview.Primitive { return se.form }

// Visible reports whether SnippetEditor is currently shown.
func (se *SnippetEditor) Visible() bool { return se.visible }
