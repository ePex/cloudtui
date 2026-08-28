package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestSectionHeaderGetLabelIsEmpty(t *testing.T) {
	h := newSectionHeader("General")
	if got := h.GetLabel(); got != "" {
		t.Errorf("GetLabel() = %q, want empty (see type doc comment on why)", got)
	}
}

func TestSectionHeaderFieldHeightIsOne(t *testing.T) {
	h := newSectionHeader("General")
	if got := h.GetFieldHeight(); got != 1 {
		t.Errorf("GetFieldHeight() = %d, want 1", got)
	}
}

func TestSectionHeaderInputHandlerIsNil(t *testing.T) {
	h := newSectionHeader("General")
	if h.InputHandler() != nil {
		t.Error("InputHandler() should be nil — a section header never has focus, so it never handles a key event")
	}
}

// TestSectionHeaderFocusReplaysLastKeyInsteadOfTakingFocus confirms Focus
// calls the finished callback (with a negative key, meaning "repeat")
// rather than actually taking focus, once SetFinishedFunc has wired it —
// this is what lets a tview.Form's Tab navigation skip straight over a
// header instead of stopping on it.
func TestSectionHeaderFocusReplaysLastKeyInsteadOfTakingFocus(t *testing.T) {
	h := newSectionHeader("General")
	var gotKey tcell.Key
	called := false
	h.SetFinishedFunc(func(key tcell.Key) {
		called = true
		gotKey = key
	})

	h.Focus(func(p tview.Primitive) {})

	if !called {
		t.Fatal("Focus did not call the finished callback")
	}
	if gotKey >= 0 {
		t.Errorf("finished called with key = %d, want a negative (repeat) key", gotKey)
	}
	if h.HasFocus() {
		t.Error("HasFocus() = true after Focus() replayed the last key — it should never actually take focus")
	}
}

// TestSectionHeaderFocusFallsBackWhenFinishedUnset confirms Focus takes
// normal Box focus when SetFinishedFunc hasn't been called yet (mirrors
// tview.TextView.Focus's own fallback for the same not-yet-wired case).
func TestSectionHeaderFocusFallsBackWhenFinishedUnset(t *testing.T) {
	h := newSectionHeader("General")
	h.Focus(func(p tview.Primitive) {})

	if !h.HasFocus() {
		t.Error("HasFocus() = false after Focus() with no finished callback wired, want true (normal Box focus fallback)")
	}
}
