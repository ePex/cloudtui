package app

import (
	"context"
	"testing"

	"github.com/gdamore/tcell/v2"

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
	want := []string{"NAME ▲", "PENDING", "CONSUMERS", "ENQUEUED", "DEQUEUED"}
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
	if got, want := qv.table.GetColumnCount(), 5; got != want {
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

func TestQueuesViewRepaintSortsAlphabetically(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	summaries := []queue.Summary{
		{Name: "zebra"},
		{Name: "apple"},
		{Name: "mango"},
	}
	qv.repaint(summaries)

	want := []string{"apple", "mango", "zebra"}
	for i, name := range want {
		cell := qv.table.GetCell(i+1, 0)
		if cell == nil {
			t.Fatalf("row %d col 0 is nil", i+1)
		}
		if got := cell.Text; got != name {
			t.Errorf("row %d name = %q, want %q", i+1, got, name)
		}
	}
}

func TestQueuesViewPendingAccentWhenNonZero(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	qv.repaint([]queue.Summary{{Name: "q", PendingCount: 7, ConsumerCount: 1}})

	pendingCell := qv.table.GetCell(1, 1)
	if pendingCell == nil {
		t.Fatal("pending cell is nil")
	}
	wantColor := tcell.GetColor(a.cfg.Colors.Accent)
	fg, _, _ := pendingCell.Style.Decompose()
	if fg != wantColor {
		t.Errorf("pending cell color = %v, want accent %v", fg, wantColor)
	}
}

func TestQueuesViewConsumerAccentWhenZero(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	qv.repaint([]queue.Summary{{Name: "q", PendingCount: 0, ConsumerCount: 0}})

	consumerCell := qv.table.GetCell(1, 2)
	if consumerCell == nil {
		t.Fatal("consumer cell is nil")
	}
	wantColor := tcell.GetColor(a.cfg.Colors.Accent)
	fg, _, _ := consumerCell.Style.Decompose()
	if fg != wantColor {
		t.Errorf("consumer cell color = %v, want accent %v", fg, wantColor)
	}
}

func TestQueuesViewPendingTextWhenZero(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	qv.repaint([]queue.Summary{{Name: "q", PendingCount: 0, ConsumerCount: 1}})

	pendingCell := qv.table.GetCell(1, 1)
	if pendingCell == nil {
		t.Fatal("pending cell is nil")
	}
	wantColor := tcell.GetColor(a.cfg.Colors.Text)
	fg, _, _ := pendingCell.Style.Decompose()
	if fg != wantColor {
		t.Errorf("pending cell color = %v, want text %v", fg, wantColor)
	}
}
