package app

import (
	"context"
	"strings"
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

func (f *fakeQueueBackend) BrowseMessages(_ context.Context, _ string) ([]queue.Message, error) {
	return nil, nil
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

func TestQueuesViewShortcutFilterPresent(t *testing.T) {
	qv := newTestQueuesView(t)
	for _, s := range qv.Shortcuts() {
		if s.Key == "/" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"/\"")
}

func TestQueuesViewFilterApplied(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	summaries := []queue.Summary{
		{Name: "foo.queue"},
		{Name: "bar.queue"},
		{Name: "foo.other"},
	}
	qv.applyFilter("foo")
	qv.repaint(summaries)

	// Rows 1 and 2 should be the two "foo" queues; row 3 should be empty.
	if got := qv.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Errorf("row count = %d, want 3 (header + 2 matches)", got)
	}
}

func TestQueuesViewFilterPersistsAfterRepaint(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	qv.applyFilter("foo")
	qv.repaint([]queue.Summary{{Name: "foo.queue"}, {Name: "bar.queue"}})
	// Second repaint with new data — filter must still apply.
	qv.repaint([]queue.Summary{{Name: "foo.queue"}, {Name: "bar.queue"}, {Name: "foo.other"}})

	if got := qv.table.GetRowCount(); got != 3 {
		t.Errorf("row count after second repaint = %d, want 3 (header + 2 matches)", got)
	}
}

func TestQueuesViewFilterClear(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	summaries := []queue.Summary{
		{Name: "foo.queue"},
		{Name: "bar.queue"},
	}
	qv.applyFilter("foo")
	qv.repaint(summaries)
	qv.applyFilter("")

	if got := qv.table.GetRowCount(); got != 3 { // header + 2 rows
		t.Errorf("row count after clear = %d, want 3", got)
	}
}

func TestQueuesViewTitleUpdatesWithFilter(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	qv.applyFilter("foo")
	if got, want := qv.table.GetTitle(), " Queues [foo] "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}

	qv.applyFilter("")
	if got, want := qv.table.GetTitle(), " Queues "; got != want {
		t.Errorf("title after clear = %q, want %q", got, want)
	}
}

func TestQueuesViewSortShortcutsPresent(t *testing.T) {
	qv := newTestQueuesView(t)
	for _, s := range qv.Shortcuts() {
		if s.Key == "o/O" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"o/O\"")
}

func TestQueuesViewSortByPendingDescending(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})
	qv.sortCol = 1
	qv.sortAsc = false

	qv.repaint([]queue.Summary{
		{Name: "a", PendingCount: 1},
		{Name: "b", PendingCount: 5},
		{Name: "c", PendingCount: 3},
	})

	wantOrder := []string{"b", "c", "a"}
	for i, name := range wantOrder {
		cell := qv.table.GetCell(i+1, 0)
		if cell == nil {
			t.Fatalf("row %d is nil", i+1)
		}
		if got := cell.Text; got != name {
			t.Errorf("row %d = %q, want %q", i+1, got, name)
		}
	}
}

func TestQueuesViewSortDirectionToggle(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})

	summaries := []queue.Summary{{Name: "b"}, {Name: "a"}}
	qv.repaint(summaries) // asc by name: a, b
	firstAsc := qv.table.GetCell(1, 0).Text

	qv.sortAsc = false
	qv.repaint(summaries) // desc: b, a
	firstDesc := qv.table.GetCell(1, 0).Text

	if firstAsc == firstDesc {
		t.Errorf("direction toggle had no effect: both rows[1] = %q", firstAsc)
	}
}

func TestQueuesViewSortHeaderMarker(t *testing.T) {
	a := New(config.Default())
	qv := newQueuesView(a, &fakeQueueBackend{})
	qv.sortCol = 2
	qv.sortAsc = true
	qv.setHeader()

	for col := 0; col < len(queueColumns); col++ {
		cell := qv.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header col %d is nil", col)
		}
		hasMarker := strings.Contains(cell.Text, "▲") || strings.Contains(cell.Text, "▼")
		if col == 2 && !hasMarker {
			t.Errorf("active sort col 2 header %q missing marker", cell.Text)
		}
		if col != 2 && hasMarker {
			t.Errorf("non-sort col %d header %q has unexpected marker", col, cell.Text)
		}
	}
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
