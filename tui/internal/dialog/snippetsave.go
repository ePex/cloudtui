package dialog

import (
	"errors"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SnippetSaveDialog is the "Save as Snippet" overlay: a single Name field
// (with "/" for subfolders) plus Save/Cancel, used to store the message
// being viewed as a snippet.
type SnippetSaveDialog struct {
	host     ui.Host
	store    *snippet.Store
	confirm  *ConfirmDialog
	form     *tview.Form
	nameItem *tview.InputField
	snippet  snippet.Snippet // what Save writes, set by Show
	onClose  func()
	visible  bool
}

// NewSnippetSaveDialog builds the save-as-snippet overlay's widgets.
// confirm asks before an existing snippet is overwritten.
func NewSnippetSaveDialog(host ui.Host, store *snippet.Store, confirm *ConfirmDialog) *SnippetSaveDialog {
	sd := &SnippetSaveDialog{host: host, store: store, confirm: confirm}
	sd.form = tview.NewForm()
	sd.form.SetBorder(true).SetTitle(" Save as Snippet ")
	sd.form.
		AddInputField("Name", "", 40, nil, nil).
		AddButton("Save", sd.save).
		AddButton("Cancel", sd.close)
	sd.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			sd.close()
			return nil
		}
		return event
	})
	sd.nameItem = sd.form.GetFormItem(0).(*tview.InputField)
	// Enter in the Name field saves. Captured (and swallowed) here rather
	// than via SetDoneFunc: tview.Form runs its own "finished" handler
	// right after the done func, moving focus on to the next element —
	// which would steal focus back from the overwrite confirmation, or
	// off the Name field after a validation error.
	sd.nameItem.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			sd.save()
			return nil
		}
		return event
	})
	return sd
}

// Show opens the dialog to save s. onClose is called on the UI goroutine
// when the dialog is dismissed (saved or cancelled).
func (sd *SnippetSaveDialog) Show(s snippet.Snippet, onClose func()) {
	sd.snippet = s
	sd.onClose = onClose
	sd.nameItem.SetText("")
	sd.form.SetFocus(0)
	sd.host.ShowPage("snippet-save")
	sd.visible = true
	sd.focusName()
}

// focusName gives focus and the context hint back to the Name field — on
// open, after a failed save, and after declining an overwrite.
func (sd *SnippetSaveDialog) focusName() {
	sd.form.SetFocus(0)
	sd.host.SetFocus(sd.form)
	ac := sd.host.Config().Colors.Accent
	sd.host.SetContextHint(fmt.Sprintf("[%s]<Enter>[-] save  [%s]<Esc>[-] cancel  (use / for subfolders)", ac, ac))
}

// save validates the name and writes the snippet. A validation or write
// error is reported in the status bar and the dialog stays open for
// correction; an existing snippet of that name asks before overwriting.
func (sd *SnippetSaveDialog) save() {
	name := sd.nameItem.GetText()
	if _, err := snippet.ValidateName(name); err != nil {
		sd.fail(err)
		return
	}
	err := sd.store.Save(name, sd.snippet, false)
	if errors.Is(err, snippet.ErrExists) {
		sd.confirm.ShowWithCancel(fmt.Sprintf("Overwrite snippet %q?", name),
			func() { sd.finish(name, sd.store.Save(name, sd.snippet, true)) },
			sd.focusName)
		return
	}
	sd.finish(name, err)
}

// finish closes the dialog on a successful save, or reports err and keeps
// it open.
func (sd *SnippetSaveDialog) finish(name string, err error) {
	if err != nil {
		sd.fail(err)
		return
	}
	sd.close()
	sd.host.SetStatus(fmt.Sprintf("Snippet saved: %s", tview.Escape(name)))
}

// fail reports err in the status bar and refocuses the Name field.
func (sd *SnippetSaveDialog) fail(err error) {
	sd.host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", tview.Escape(err.Error())))
	sd.focusName()
}

// close hides the dialog and calls onClose to let the caller restore
// focus and the context panel.
func (sd *SnippetSaveDialog) close() {
	sd.host.HidePage("snippet-save")
	sd.visible = false
	if sd.onClose != nil {
		sd.onClose()
	}
}

// ApplyPalette recolors the save-as-snippet overlay for a live theme switch.
func (sd *SnippetSaveDialog) ApplyPalette(p config.Palette) {
	sd.form.SetBackgroundColor(tcell.GetColor(p.Background))
	sd.form.SetBorderColor(tcell.GetColor(p.Border))
	sd.form.SetTitleColor(tcell.GetColor(p.Border))
}

var _ ui.Themeable = (*SnippetSaveDialog)(nil)

// Primitive returns SnippetSaveDialog's root widget, for sizing/embedding.
func (sd *SnippetSaveDialog) Primitive() tview.Primitive { return sd.form }

// Visible reports whether SnippetSaveDialog is currently shown.
func (sd *SnippetSaveDialog) Visible() bool { return sd.visible }
