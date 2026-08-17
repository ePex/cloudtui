package view

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

type fakeQueueBackend struct {
	summaries []queue.Summary
}

func (f *fakeQueueBackend) List(_ context.Context) ([]queue.Summary, error) {
	return f.summaries, nil
}

func (f *fakeQueueBackend) BrowseMessages(_ context.Context, _ string, _ queue.MessageFilter) ([]queue.Message, error) {
	return nil, nil
}

func (f *fakeQueueBackend) PurgeQueue(_ context.Context, _ string) error {
	return nil
}

func (f *fakeQueueBackend) RemoveMessage(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeQueueBackend) MoveMessage(_ context.Context, _, _, _ string) error {
	return nil
}

func (f *fakeQueueBackend) MoveAllMessages(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

func (f *fakeQueueBackend) SendMessage(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeQueueBackend) DeleteMessages(_ context.Context, _ string, _ queue.MessageFilter) (int, error) {
	return 0, nil
}

func (f *fakeQueueBackend) MoveMessages(_ context.Context, _, _ string, _ queue.MessageFilter) (int, error) {
	return 0, nil
}

func newTestQueuesView(t *testing.T) (*fakeViewHost, *QueuesView) {
	t.Helper()
	host := newFakeViewHost()
	confirm := dialog.NewConfirmDialog(host)
	movePicker := dialog.NewMovePicker(host)
	sendMessage := dialog.NewSendMessageOverlay(host)
	return host, NewQueuesView(host, &fakeQueueBackend{}, confirm, movePicker, sendMessage, func(string) {})
}

func TestQueuesViewHeaderLabels(t *testing.T) {
	_, qv := newTestQueuesView(t)
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
	_, qv := newTestQueuesView(t)
	if got, want := qv.table.GetColumnCount(), 5; got != want {
		t.Errorf("GetColumnCount() = %d, want %d", got, want)
	}
}

func TestQueuesViewShortcutRPresent(t *testing.T) {
	_, qv := newTestQueuesView(t)
	for _, s := range qv.Shortcuts() {
		if s.Key == "r" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"r\"")
}

func TestQueuesViewPurgeShortcutPresent(t *testing.T) {
	_, qv := newTestQueuesView(t)
	for _, s := range qv.Shortcuts() {
		if s.Key == "p" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"p\"")
}

func TestQueuesViewShortcutFilterPresent(t *testing.T) {
	_, qv := newTestQueuesView(t)
	for _, s := range qv.Shortcuts() {
		if s.Key == "/" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"/\"")
}

func TestQueuesViewFilterApplied(t *testing.T) {
	_, qv := newTestQueuesView(t)

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
	_, qv := newTestQueuesView(t)

	qv.applyFilter("foo")
	qv.repaint([]queue.Summary{{Name: "foo.queue"}, {Name: "bar.queue"}})
	// Second repaint with new data — filter must still apply.
	qv.repaint([]queue.Summary{{Name: "foo.queue"}, {Name: "bar.queue"}, {Name: "foo.other"}})

	if got := qv.table.GetRowCount(); got != 3 {
		t.Errorf("row count after second repaint = %d, want 3 (header + 2 matches)", got)
	}
}

func TestQueuesViewFilterClear(t *testing.T) {
	_, qv := newTestQueuesView(t)

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
	_, qv := newTestQueuesView(t)

	qv.applyFilter("foo")
	if got, want := qv.table.GetTitle(), " Queues (foo) "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}

	qv.applyFilter("")
	if got, want := qv.table.GetTitle(), " Queues "; got != want {
		t.Errorf("title after clear = %q, want %q", got, want)
	}
}

// TestQueuesViewRepaintScrollsToTopWithManyRows guards against a regression
// where tview.Table's "track end" auto-scroll (meant for tailing logs)
// latches on during the first draw of the still-empty table — before the
// async load completes — and stays latched through the repaint that follows,
// leaving a long list scrolled to the bottom instead of the top.
func TestQueuesViewRepaintScrollsToTopWithManyRows(t *testing.T) {
	_, qv := newTestQueuesView(t)
	qv.table.SetRect(0, 0, 60, 15) // fewer visible rows than summaries below

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)

	// First draw while the table is still empty (header only), mirroring the
	// real sequence: SwitchTo("queues") draws before the async load returns.
	qv.table.Draw(screen)

	summaries := make([]queue.Summary, 50)
	for i := range summaries {
		summaries[i] = queue.Summary{Name: fmt.Sprintf("queue-%02d", i)}
	}
	qv.repaint(summaries)

	// The redraw that follows repaint via QueueUpdateDraw.
	qv.table.Draw(screen)

	if row, _ := qv.table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}

// renderedScreenText draws prim to a same-size SimulationScreen and
// concatenates every cell's rune into one string, so a test can check what
// actually gets drawn — as opposed to GetTitle()/GetText(), which only
// return the stored value and would not have caught the bug this guards
// against (tview.Box titles run their text through the same tag-parsing
// Print() that Table cells do, so "[text]" is silently swallowed as an
// invalid color tag; GetTitle() still faithfully returns the literal
// "[text]" string, making the bug invisible to a test that doesn't
// actually render).
func renderedScreenText(t *testing.T, prim tview.Primitive, width, height int) string {
	t.Helper()
	prim.SetRect(0, 0, width, height)
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(width, height)
	prim.Draw(screen)
	screen.Show() // flushes the back buffer into front; GetContents reads front

	cells, w, h := screen.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := cells[y*w+x]
			if len(cell.Runes) > 0 {
				b.WriteRune(cell.Runes[0])
			}
		}
	}
	return b.String()
}

// TestQueuesViewFilteredTitleActuallyRenders is the render-based companion
// to the title-format fix: GetTitle() alone wouldn't have caught the bug
// (see renderedScreenText's doc comment).
func TestQueuesViewFilteredTitleActuallyRenders(t *testing.T) {
	_, qv := newTestQueuesView(t)
	qv.applyFilter("foo")

	rendered := renderedScreenText(t, qv.table, 60, 10)
	if !strings.Contains(rendered, "foo") {
		t.Errorf("rendered screen = %q, want it to contain the filter text %q", rendered, "foo")
	}
}

func TestQueuesViewSortShortcutsPresent(t *testing.T) {
	_, qv := newTestQueuesView(t)
	for _, s := range qv.Shortcuts() {
		if s.Key == "o/O" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"o/O\"")
}

func TestQueuesViewSortByPendingDescending(t *testing.T) {
	_, qv := newTestQueuesView(t)
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
	_, qv := newTestQueuesView(t)

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
	_, qv := newTestQueuesView(t)
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
	_, qv := newTestQueuesView(t)

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
	host, qv := newTestQueuesView(t)

	qv.repaint([]queue.Summary{{Name: "q", PendingCount: 7, ConsumerCount: 1}})

	pendingCell := qv.table.GetCell(1, 1)
	if pendingCell == nil {
		t.Fatal("pending cell is nil")
	}
	wantColor := tcell.GetColor(host.cfg.Colors.Accent)
	fg, _, _ := pendingCell.Style.Decompose()
	if fg != wantColor {
		t.Errorf("pending cell color = %v, want accent %v", fg, wantColor)
	}
}

func TestQueuesViewConsumerAccentWhenZero(t *testing.T) {
	host, qv := newTestQueuesView(t)

	qv.repaint([]queue.Summary{{Name: "q", PendingCount: 0, ConsumerCount: 0}})

	consumerCell := qv.table.GetCell(1, 2)
	if consumerCell == nil {
		t.Fatal("consumer cell is nil")
	}
	wantColor := tcell.GetColor(host.cfg.Colors.Accent)
	fg, _, _ := consumerCell.Style.Decompose()
	if fg != wantColor {
		t.Errorf("consumer cell color = %v, want accent %v", fg, wantColor)
	}
}

func TestQueuesViewPendingTextWhenZero(t *testing.T) {
	host, qv := newTestQueuesView(t)

	qv.repaint([]queue.Summary{{Name: "q", PendingCount: 0, ConsumerCount: 1}})

	pendingCell := qv.table.GetCell(1, 1)
	if pendingCell == nil {
		t.Fatal("pending cell is nil")
	}
	wantColor := tcell.GetColor(host.cfg.Colors.Text)
	fg, _, _ := pendingCell.Style.Decompose()
	if fg != wantColor {
		t.Errorf("pending cell color = %v, want text %v", fg, wantColor)
	}
}
