package app

import (
	"context"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

type fakeQueueBackend struct {
	summaries []queue.Summary
}

func (f *fakeQueueBackend) List(_ context.Context) ([]queue.Summary, error) {
	return f.summaries, nil
}

func newTestQueuesView(t *testing.T) *queuesView {
	t.Helper()
	a := New(config.Default())
	return newQueuesView(a, &fakeQueueBackend{})
}

func TestQueuesViewHeaderLabels(t *testing.T) {
	qv := newTestQueuesView(t)
	want := []string{"Name", "Pending", "Consumers"}
	for col, label := range want {
		cell := qv.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

func TestQueuesViewColumnCount(t *testing.T) {
	qv := newTestQueuesView(t)
	if got, want := qv.table.GetColumnCount(), 3; got != want {
		t.Errorf("GetColumnCount() = %d, want %d", got, want)
	}
}

func TestQueuesViewShortcutRPresent(t *testing.T) {
	qv := newTestQueuesView(t)
	for _, s := range qv.Shortcuts() {
		if s.Key == "r" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"r\"")
}
