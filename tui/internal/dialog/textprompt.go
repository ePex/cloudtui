package dialog

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// TextPrompt is a small one-field overlay (title, label, prefilled text,
// OK/Cancel) for asking the user for a single value — e.g. a folder name
// or a new path. It knows nothing about what the value is for: the
// caller's onSubmit validates and applies it.
type TextPrompt struct {
	host     ui.Host
	form     *tview.Form
	field    *tview.InputField
	onSubmit func(text string) error
	onClose  func()
	visible  bool
}

// NewTextPrompt builds the prompt's widgets.
func NewTextPrompt(host ui.Host) *TextPrompt {
	tp := &TextPrompt{host: host}
	tp.form = tview.NewForm()
	tp.form.SetBorder(true)
	tp.form.
		AddInputField("", "", 44, nil, nil).
		AddButton("OK", tp.submit).
		AddButton("Cancel", tp.close)
	tp.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			tp.close()
			return nil
		}
		return event
	})
	tp.field = tp.form.GetFormItem(0).(*tview.InputField)
	// Enter in the field submits. Captured (and swallowed) here rather
	// than via SetDoneFunc: tview.Form moves focus on to the next element
	// right after a done func, which would take focus off the field after
	// a rejected value (see tui/CLAUDE.md's tview gotchas).
	tp.field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			tp.submit()
			return nil
		}
		return event
	})
	return tp
}

// Show opens the prompt titled title, its field labelled label and
// prefilled with initial (cursor at the end). onSubmit gets the entered
// text on OK or Enter: a non-nil error is reported in the status bar and
// the prompt stays open for correction; nil closes it. onClose is called
// whenever the prompt is dismissed (submitted or cancelled).
func (tp *TextPrompt) Show(title, label, initial string, onSubmit func(text string) error, onClose func()) {
	tp.onSubmit = onSubmit
	tp.onClose = onClose
	tp.form.SetTitle(" " + title + " ")
	tp.field.SetLabel(label + " ")
	ui.SetInputFieldText(tp.field, initial)
	tp.host.ShowPage("text-prompt")
	tp.visible = true
	tp.focusField()
}

// focusField gives focus and the context hint back to the field.
func (tp *TextPrompt) focusField() {
	tp.form.SetFocus(0)
	tp.host.SetFocus(tp.form)
	ac := tp.host.Config().Colors.Accent
	tp.host.SetContextHint(fmt.Sprintf("[%s]<Enter>[-] ok  [%s]<Esc>[-] cancel", ac, ac))
}

// submit hands the field's text to onSubmit, closing the prompt on
// success and keeping it open (with the error in the status bar) on
// failure.
func (tp *TextPrompt) submit() {
	if tp.onSubmit != nil {
		if err := tp.onSubmit(tp.field.GetText()); err != nil {
			tp.host.SetStatus(fmt.Sprintf("[red]Error: %s[-]", tview.Escape(err.Error())))
			tp.focusField()
			return
		}
	}
	tp.close()
}

// close hides the prompt and calls onClose to let the caller restore
// focus and the context panel.
func (tp *TextPrompt) close() {
	tp.host.HidePage("text-prompt")
	tp.visible = false
	if tp.onClose != nil {
		tp.onClose()
	}
}

// ApplyPalette recolors the prompt for a live theme switch.
func (tp *TextPrompt) ApplyPalette(p config.Palette) {
	tp.form.SetBackgroundColor(tcell.GetColor(p.Background))
	tp.form.SetBorderColor(tcell.GetColor(p.Border))
	tp.form.SetTitleColor(tcell.GetColor(p.Border))
	ui.StyleForm(tp.form, p)
}

var _ ui.Themeable = (*TextPrompt)(nil)

// Primitive returns TextPrompt's root widget, for sizing/embedding.
func (tp *TextPrompt) Primitive() tview.Primitive { return tp.form }

// Visible reports whether TextPrompt is currently shown.
func (tp *TextPrompt) Visible() bool { return tp.visible }
