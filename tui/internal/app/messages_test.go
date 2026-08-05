package app

import (
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func newTestMessagesView(t *testing.T) *messagesView {
	t.Helper()
	a := New(config.Default())
	return newMessagesView(a)
}

func TestMessagesViewHeaderLabels(t *testing.T) {
	mv := newTestMessagesView(t)
	want := []string{"ID", "TYPE", "CORR.ID", "TIMESTAMP", "PREVIEW"}
	for col, label := range want {
		cell := mv.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

func TestMessagesViewColumnCount(t *testing.T) {
	mv := newTestMessagesView(t)
	if got, want := mv.table.GetColumnCount(), 5; got != want {
		t.Errorf("GetColumnCount() = %d, want %d", got, want)
	}
}

func TestMessagesViewShortcutRPresent(t *testing.T) {
	mv := newTestMessagesView(t)
	for _, s := range mv.Shortcuts() {
		if s.Key == "r" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"r\"")
}
