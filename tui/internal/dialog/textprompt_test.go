package dialog

import (
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// pressEnterInPrompt sends Enter through the prompt's form the way
// tview.Application does, with a setFocus that records and applies focus
// — so tview.Form moving focus on by itself would show up in
// host.focused.
func pressEnterInPrompt(tp *TextPrompt, host *testHost) {
	var setFocus func(tview.Primitive)
	setFocus = func(p tview.Primitive) {
		host.focused = p
		p.Focus(setFocus)
	}
	tp.form.Focus(setFocus)
	tp.form.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)
}

func TestTextPromptShow(t *testing.T) {
	host := newTestHost()
	tp := NewTextPrompt(host)
	tp.Show("New folder", "Name:", "orders/", func(string) error { return nil }, func() {})

	if !tp.Visible() || host.focused != tp.form {
		t.Error("Show did not make the prompt visible and focused")
	}
	if host.shownPages[len(host.shownPages)-1] != "text-prompt" {
		t.Errorf("shownPages = %q", host.shownPages)
	}
	if got := tp.field.GetText(); got != "orders/" {
		t.Errorf("field = %q, want the prefilled text", got)
	}
	text := renderedScreenText(t, tp.Primitive(), 64, 8)
	for _, want := range []string{"New folder", "Name:", "orders/", "OK", "Cancel"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered prompt (64x8) lacks %q", want)
		}
	}
}

func TestTextPromptSubmitSuccessCloses(t *testing.T) {
	host := newTestHost()
	tp := NewTextPrompt(host)
	var got string
	closed := 0
	tp.Show("Rename", "Path:", "a.txt", func(text string) error { got = text; return nil }, func() { closed++ })
	tp.field.SetText("b.txt")

	pressEnterInPrompt(tp, host)

	if got != "b.txt" {
		t.Errorf("onSubmit got %q, want %q", got, "b.txt")
	}
	if closed != 1 || tp.Visible() {
		t.Errorf("closed = %d, visible = %v; want 1, false", closed, tp.Visible())
	}
}

func TestTextPromptSubmitErrorStaysOpen(t *testing.T) {
	host := newTestHost()
	tp := NewTextPrompt(host)
	closed := 0
	tp.Show("Rename", "Path:", "a.txt", func(string) error { return errors.New("already exists") }, func() { closed++ })

	pressEnterInPrompt(tp, host)

	if closed != 0 || !tp.Visible() {
		t.Errorf("closed = %d, visible = %v; want 0, true", closed, tp.Visible())
	}
	if !strings.Contains(host.status, "[red]") || !strings.Contains(host.status, "already exists") {
		t.Errorf("status = %q, want the red error", host.status)
	}
	if item, button := tp.form.GetFocusedItemIndex(); host.focused != tp.form || item != 0 || button != -1 {
		t.Errorf("focus = %T (item %d, button %d), want the form on its field — not moved on by tview.Form", host.focused, item, button)
	}
}

func TestTextPromptOKButtonSubmits(t *testing.T) {
	host := newTestHost()
	tp := NewTextPrompt(host)
	submitted := false
	tp.Show("New folder", "Name:", "x", func(string) error { submitted = true; return nil }, func() {})
	tp.form.GetButton(tp.form.GetButtonIndex("OK")).InputHandler()(
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if !submitted || tp.Visible() {
		t.Errorf("submitted = %v, visible = %v; want true, false", submitted, tp.Visible())
	}
}

func TestTextPromptEscCancels(t *testing.T) {
	host := newTestHost()
	tp := NewTextPrompt(host)
	submitted, closed := false, 0
	tp.Show("New folder", "Name:", "x", func(string) error { submitted = true; return nil }, func() { closed++ })

	tp.form.GetInputCapture()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if submitted || closed != 1 || tp.Visible() {
		t.Errorf("submitted = %v, closed = %d, visible = %v; want false, 1, false", submitted, closed, tp.Visible())
	}
}

// TestTextPromptLongInitialKeepsCursorVisible checks a long prefilled
// path (e.g. a deep rename) shows its end, where the cursor is — the
// ui.SetInputFieldText behavior plain SetText lacks.
func TestTextPromptLongInitialKeepsCursorVisible(t *testing.T) {
	host := newTestHost()
	tp := NewTextPrompt(host)
	long := "archive/2026/q3/eu/orders/created-with-a-long-name.json"
	tp.Show("Rename", "Path:", long, func(string) error { return nil }, func() {})

	if text := renderedScreenText(t, tp.Primitive(), 64, 8); !strings.Contains(text, "long-name.json") {
		t.Errorf("rendered prompt doesn't show the end of the long path: %q", text)
	}
}
