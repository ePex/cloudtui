package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// selectConfirmItem moves the confirm list to item index (0 = No,
// 1 = Yes) and presses Enter on it.
func selectConfirmItem(c *ConfirmDialog, index int) {
	c.list.SetCurrentItem(index)
	c.list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
}

// pressConfirmEsc sends Esc through the list's input capture, the way
// tview.Application would before the list's own handler.
func pressConfirmEsc(c *ConfirmDialog) {
	c.list.GetInputCapture()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
}

func TestConfirmShowDefaultsToNo(t *testing.T) {
	host := newTestHost()
	c := NewConfirmDialog(host)
	c.Show("Delete?", func() {})

	if got, _ := c.list.GetItemText(c.list.GetCurrentItem()); got != "No" {
		t.Errorf("focused item = %q, want %q", got, "No")
	}
	if host.focused != c.list || !c.Visible() {
		t.Error("Show did not focus the list and mark the dialog visible")
	}
}

func TestConfirmShowNoAndEscFocusMain(t *testing.T) {
	for _, tt := range []struct {
		name   string
		cancel func(*ConfirmDialog)
	}{
		{"No", func(c *ConfirmDialog) { selectConfirmItem(c, 0) }},
		{"Esc", pressConfirmEsc},
	} {
		t.Run(tt.name, func(t *testing.T) {
			host := newTestHost()
			c := NewConfirmDialog(host)
			confirmed := false
			c.Show("Delete?", func() { confirmed = true })

			tt.cancel(c)

			if confirmed {
				t.Error("onConfirm was called")
			}
			if host.focusMainCalls != 1 {
				t.Errorf("FocusMain calls = %d, want 1", host.focusMainCalls)
			}
			if c.Visible() {
				t.Error("dialog still visible")
			}
		})
	}
}

func TestConfirmShowYesConfirms(t *testing.T) {
	host := newTestHost()
	c := NewConfirmDialog(host)
	confirmed := false
	c.Show("Delete?", func() { confirmed = true })

	selectConfirmItem(c, 1)

	if !confirmed {
		t.Error("onConfirm was not called")
	}
	if c.Visible() {
		t.Error("dialog still visible")
	}
}

func TestConfirmShowWithCancelNoAndEscCallOnCancel(t *testing.T) {
	for _, tt := range []struct {
		name   string
		cancel func(*ConfirmDialog)
	}{
		{"No", func(c *ConfirmDialog) { selectConfirmItem(c, 0) }},
		{"Esc", pressConfirmEsc},
	} {
		t.Run(tt.name, func(t *testing.T) {
			host := newTestHost()
			c := NewConfirmDialog(host)
			confirmed, cancelled := false, 0
			c.ShowWithCancel("Overwrite?", func() { confirmed = true }, func() { cancelled++ })

			tt.cancel(c)

			if confirmed {
				t.Error("onConfirm was called")
			}
			if cancelled != 1 {
				t.Errorf("onCancel calls = %d, want 1", cancelled)
			}
			if host.focusMainCalls != 0 {
				t.Errorf("FocusMain calls = %d, want 0 (onCancel restores focus instead)", host.focusMainCalls)
			}
			if c.Visible() {
				t.Error("dialog still visible")
			}
		})
	}
}

func TestConfirmShowWithCancelYesSkipsOnCancel(t *testing.T) {
	host := newTestHost()
	c := NewConfirmDialog(host)
	confirmed, cancelled := false, false
	c.ShowWithCancel("Overwrite?", func() { confirmed = true }, func() { cancelled = true })

	selectConfirmItem(c, 1)

	if !confirmed || cancelled {
		t.Errorf("confirmed = %v, cancelled = %v; want true, false", confirmed, cancelled)
	}
}

// TestConfirmReuseDropsPreviousOnCancel guards the shared-instance case:
// a later plain Show must not fire an onCancel left over from an earlier
// ShowWithCancel.
func TestConfirmReuseDropsPreviousOnCancel(t *testing.T) {
	host := newTestHost()
	c := NewConfirmDialog(host)
	staleCancel := false
	c.ShowWithCancel("First?", func() {}, func() { staleCancel = true })
	selectConfirmItem(c, 1)

	c.Show("Second?", func() {})
	pressConfirmEsc(c)

	if staleCancel {
		t.Error("onCancel from the earlier ShowWithCancel fired")
	}
	if host.focusMainCalls != 2 {
		t.Errorf("FocusMain calls = %d, want 2 (Yes, then Esc on the plain Show)", host.focusMainCalls)
	}
}

// TestConfirmLongQuestionFitsOverlay renders the dialog at the size
// app.go gives it (52×8) with a long question — a delete of a deeply
// nested snippet folder — and checks nothing is cut off: the end of the
// question (the counts) and both answers stay visible.
func TestConfirmLongQuestionFitsOverlay(t *testing.T) {
	c := NewConfirmDialog(newTestHost())
	question := `Delete folder "archive/2026/q3/eu/payments/refunds/partial" and its 12 snippets, 3 subfolders?`
	c.Show(question, func() {})

	text := renderedScreenText(t, c.Primitive(), 52, 8)
	for _, want := range []string{"archive/2026/q3", "subfolders?", "No", "Yes"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered confirm (52x8) lacks %q:\n%s", want, text)
		}
	}
}
