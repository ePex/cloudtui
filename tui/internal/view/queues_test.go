package view

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

type fakeQueueBackend struct {
	summaries []queue.Summary

	// Injectable per-test overrides for the purge/move-all routing tests
	// (see TestQueuesViewPurge*/TestQueuesViewMoveAll* below) — nil means
	// the plain no-op default every other test in this file already
	// relies on.
	purgeQueueFn      func(ctx context.Context, queueName string) error
	deleteMessagesFn  func(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error)
	moveAllMessagesFn func(ctx context.Context, sourceQueue, targetQueue string) (int, error)
	moveMessagesFn    func(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error)
	// listFn overrides List entirely when set — used by the Load()
	// loading-indicator/stale-response tests below to control timing and
	// inject errors, without disturbing every other test's plain
	// f.summaries default.
	listFn func(ctx context.Context) ([]queue.Summary, error)
}

func (f *fakeQueueBackend) List(ctx context.Context) ([]queue.Summary, error) {
	if f.listFn != nil {
		return f.listFn(ctx)
	}
	return f.summaries, nil
}

func (f *fakeQueueBackend) BrowseMessages(_ context.Context, _ string, _ queue.MessageFilter) ([]queue.Message, error) {
	return nil, nil
}

func (f *fakeQueueBackend) PurgeQueue(ctx context.Context, queueName string) error {
	if f.purgeQueueFn == nil {
		return nil
	}
	return f.purgeQueueFn(ctx, queueName)
}

func (f *fakeQueueBackend) RemoveMessage(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeQueueBackend) MoveMessage(_ context.Context, _, _, _ string) error {
	return nil
}

func (f *fakeQueueBackend) MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) {
	if f.moveAllMessagesFn == nil {
		return 0, nil
	}
	return f.moveAllMessagesFn(ctx, sourceQueue, targetQueue)
}

func (f *fakeQueueBackend) SendMessage(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeQueueBackend) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
	if f.deleteMessagesFn == nil {
		return 0, nil
	}
	return f.deleteMessagesFn(ctx, queueName, filter)
}

func (f *fakeQueueBackend) MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
	if f.moveMessagesFn == nil {
		return 0, nil
	}
	return f.moveMessagesFn(ctx, sourceQueue, targetQueue, filter)
}

func newTestQueuesView(t *testing.T) (*fakeViewHost, *QueuesView) {
	t.Helper()
	return newTestQueuesViewWithBackend(t, &fakeQueueBackend{})
}

func newTestQueuesViewWithBackend(t *testing.T, b *fakeQueueBackend) (*fakeViewHost, *QueuesView) {
	t.Helper()
	host := newFakeViewHost()
	confirm := dialog.NewConfirmDialog(host)
	movePicker := dialog.NewMovePicker(host)
	sendMessage := dialog.NewSendMessageOverlay(host)
	jmsTypePrompt := dialog.NewJMSTypePrompt(host)
	return host, NewQueuesView(host, b, confirm, movePicker, sendMessage, jmsTypePrompt, func(string) {})
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

// TestQueuesViewDoPurgeUnfilteredCallsPurgeQueue confirms an empty
// jmsType keeps using the existing fast, native PurgeQueue path rather
// than DeleteMessages with an empty filter — see spec/09's "Preserving
// the existing unfiltered path".
func TestQueuesViewDoPurgeUnfilteredCallsPurgeQueue(t *testing.T) {
	var gotQueue string
	purgeCalled := false
	deleteCalled := false
	b := &fakeQueueBackend{
		purgeQueueFn: func(_ context.Context, queueName string) error {
			purgeCalled = true
			gotQueue = queueName
			return nil
		},
		deleteMessagesFn: func(context.Context, string, queue.MessageFilter) (int, error) {
			deleteCalled = true
			return 0, nil
		},
	}
	_, qv := newTestQueuesViewWithBackend(t, b)

	err := qv.doPurge(context.Background(), "orders", "")

	if err != nil {
		t.Fatalf("doPurge() error = %v", err)
	}
	if !purgeCalled {
		t.Error("PurgeQueue was not called for an empty JMS type")
	}
	if deleteCalled {
		t.Error("DeleteMessages was called for an empty JMS type, want PurgeQueue only")
	}
	if gotQueue != "orders" {
		t.Errorf("queue passed to PurgeQueue = %q, want %q", gotQueue, "orders")
	}
}

// TestQueuesViewDoPurgeFilteredCallsDeleteMessages confirms a non-empty
// jmsType routes through DeleteMessages with the filter set, not
// PurgeQueue.
func TestQueuesViewDoPurgeFilteredCallsDeleteMessages(t *testing.T) {
	var gotQueue string
	var gotFilter queue.MessageFilter
	purgeCalled := false
	b := &fakeQueueBackend{
		purgeQueueFn: func(context.Context, string) error {
			purgeCalled = true
			return nil
		},
		deleteMessagesFn: func(_ context.Context, queueName string, filter queue.MessageFilter) (int, error) {
			gotQueue, gotFilter = queueName, filter
			return 3, nil
		},
	}
	_, qv := newTestQueuesViewWithBackend(t, b)

	err := qv.doPurge(context.Background(), "orders", "OrderCreated")

	if err != nil {
		t.Fatalf("doPurge() error = %v", err)
	}
	if purgeCalled {
		t.Error("PurgeQueue was called for a non-empty JMS type, want DeleteMessages only")
	}
	if gotQueue != "orders" {
		t.Errorf("queue passed to DeleteMessages = %q, want %q", gotQueue, "orders")
	}
	if want := (queue.MessageFilter{JMSType: "OrderCreated"}); gotFilter != want {
		t.Errorf("filter passed to DeleteMessages = %+v, want %+v", gotFilter, want)
	}
}

func TestQueuesViewDoMoveAllUnfilteredCallsMoveAllMessages(t *testing.T) {
	var gotSrc, gotDst string
	moveAllCalled := false
	moveCalled := false
	b := &fakeQueueBackend{
		moveAllMessagesFn: func(_ context.Context, sourceQueue, targetQueue string) (int, error) {
			moveAllCalled = true
			gotSrc, gotDst = sourceQueue, targetQueue
			return 5, nil
		},
		moveMessagesFn: func(context.Context, string, string, queue.MessageFilter) (int, error) {
			moveCalled = true
			return 0, nil
		},
	}
	_, qv := newTestQueuesViewWithBackend(t, b)

	count, err := qv.doMoveAll(context.Background(), "orders", "orders-archive", "")

	if err != nil {
		t.Fatalf("doMoveAll() error = %v", err)
	}
	if !moveAllCalled {
		t.Error("MoveAllMessages was not called for an empty JMS type")
	}
	if moveCalled {
		t.Error("MoveMessages was called for an empty JMS type, want MoveAllMessages only")
	}
	if gotSrc != "orders" || gotDst != "orders-archive" {
		t.Errorf("MoveAllMessages(%q, %q), want (%q, %q)", gotSrc, gotDst, "orders", "orders-archive")
	}
	if count != 5 {
		t.Errorf("doMoveAll() count = %d, want %d", count, 5)
	}
}

func TestQueuesViewDoMoveAllFilteredCallsMoveMessages(t *testing.T) {
	var gotSrc, gotDst string
	var gotFilter queue.MessageFilter
	moveAllCalled := false
	b := &fakeQueueBackend{
		moveAllMessagesFn: func(context.Context, string, string) (int, error) {
			moveAllCalled = true
			return 0, nil
		},
		moveMessagesFn: func(_ context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
			gotSrc, gotDst, gotFilter = sourceQueue, targetQueue, filter
			return 2, nil
		},
	}
	_, qv := newTestQueuesViewWithBackend(t, b)

	count, err := qv.doMoveAll(context.Background(), "orders", "orders-archive", "OrderCreated")

	if err != nil {
		t.Fatalf("doMoveAll() error = %v", err)
	}
	if moveAllCalled {
		t.Error("MoveAllMessages was called for a non-empty JMS type, want MoveMessages only")
	}
	if gotSrc != "orders" || gotDst != "orders-archive" {
		t.Errorf("MoveMessages(%q, %q, ...), want (%q, %q)", gotSrc, gotDst, "orders", "orders-archive")
	}
	if want := (queue.MessageFilter{JMSType: "OrderCreated"}); gotFilter != want {
		t.Errorf("filter passed to MoveMessages = %+v, want %+v", gotFilter, want)
	}
	if count != 2 {
		t.Errorf("doMoveAll() count = %d, want %d", count, 2)
	}
}

// TestQueuesViewPurgeKeyShowsJMSTypePrompt confirms pressing 'p' opens
// the JMS Type prompt (rather than jumping straight to the confirm
// dialog, as it did before this filter step existed).
func TestQueuesViewPurgeKeyShowsJMSTypePrompt(t *testing.T) {
	_, qv := newTestQueuesView(t)
	qv.repaint([]queue.Summary{{Name: "orders"}})
	qv.table.Select(1, 0)

	qv.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))

	if !qv.jmsTypePrompt.Visible() {
		t.Error("jmsTypePrompt is not visible after pressing 'p'")
	}
	if qv.confirm.Visible() {
		t.Error("confirm dialog is visible before the JMS Type prompt was continued past")
	}
}

// TestQueuesViewMoveAllKeyShowsJMSTypePrompt is the 'M' analogue of
// TestQueuesViewPurgeKeyShowsJMSTypePrompt.
func TestQueuesViewMoveAllKeyShowsJMSTypePrompt(t *testing.T) {
	_, qv := newTestQueuesView(t)
	qv.repaint([]queue.Summary{{Name: "orders"}})
	qv.table.Select(1, 0)

	qv.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'M', tcell.ModNone))

	if !qv.jmsTypePrompt.Visible() {
		t.Error("jmsTypePrompt is not visible after pressing 'M'")
	}
	if qv.movePicker.Visible() {
		t.Error("movePicker is visible before the JMS Type prompt was continued past")
	}
}

// drawSignalingHost wraps fakeViewHost to additionally signal after every
// QueueUpdateDraw call — used only by TestQueuesViewLoadDiscardsStaleResponse
// below to deterministically order assertions around two overlapping Load()
// goroutines without an actual data race (channel operations establish the
// happens-before edge -race needs).
type drawSignalingHost struct {
	*fakeViewHost
	drawn chan struct{}
}

func (h *drawSignalingHost) QueueUpdateDraw(fn func()) {
	fn()
	h.drawn <- struct{}{}
}

// newTestQueuesViewWithDrawSignal is newTestQueuesViewWithBackend's
// draw-signaling counterpart: drawn receives once per QueueUpdateDraw call,
// letting a test deterministically wait for Load()'s background goroutine
// to finish applying its result before asserting on qv.table — without it,
// a fast (non-blocking) fakeQueueBackend races the test's own goroutine.
func newTestQueuesViewWithDrawSignal(t *testing.T, b *fakeQueueBackend, bufSize int) (*drawSignalingHost, *QueuesView) {
	t.Helper()
	base := newFakeViewHost()
	base.backend = b
	host := &drawSignalingHost{fakeViewHost: base, drawn: make(chan struct{}, bufSize)}
	confirm := dialog.NewConfirmDialog(host)
	movePicker := dialog.NewMovePicker(host)
	sendMessage := dialog.NewSendMessageOverlay(host)
	jmsTypePrompt := dialog.NewJMSTypePrompt(host)
	return host, NewQueuesView(host, b, confirm, movePicker, sendMessage, jmsTypePrompt, func(string) {})
}

func TestQueuesViewLoadShowsLoadingStatusImmediately(t *testing.T) {
	unblock := make(chan struct{})
	backend := &fakeQueueBackend{
		listFn: func(context.Context) ([]queue.Summary, error) {
			<-unblock
			return nil, nil
		},
	}
	_, qv := newTestQueuesViewWithBackend(t, backend)

	qv.Load()

	cell := qv.table.GetCell(1, 0)
	if cell == nil || cell.Text != "Loading queues…" {
		t.Errorf("row(1,0) after Load() = %+v, want text %q", cell, "Loading queues…")
	}
	close(unblock) // let the goroutine finish so it doesn't leak past the test
}

func TestQueuesViewLoadRepaintsOnSuccess(t *testing.T) {
	backend := &fakeQueueBackend{summaries: []queue.Summary{{Name: "orders"}}}
	host, qv := newTestQueuesViewWithDrawSignal(t, backend, 1)

	qv.Load()
	<-host.drawn

	name := qv.table.GetCell(1, 0).Text
	if name != "orders" {
		t.Errorf("row(1,0) after Load() success = %q, want %q", name, "orders")
	}
}

func TestQueuesViewLoadShowsErrorOnFailure(t *testing.T) {
	wantErr := errors.New("connection refused")
	backend := &fakeQueueBackend{
		listFn: func(context.Context) ([]queue.Summary, error) {
			return nil, wantErr
		},
	}
	host, qv := newTestQueuesViewWithDrawSignal(t, backend, 1)

	qv.Load()
	<-host.drawn

	cell := qv.table.GetCell(1, 0)
	if cell == nil || !strings.Contains(cell.Text, wantErr.Error()) {
		t.Errorf("row(1,0) after Load() failure = %+v, want it to contain %q", cell, wantErr.Error())
	}
}

// TestQueuesViewLoadDiscardsStaleResponse is the key regression test for
// loadSeq: if the user triggers a second Load() (another connection
// switch, or 'r') before the first resolves, the first's eventual — slower
// — response must not clobber the second's already-rendered result.
func TestQueuesViewLoadDiscardsStaleResponse(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	firstCalled := make(chan struct{})
	releaseFirst := make(chan struct{})

	backend := &fakeQueueBackend{}
	backend.listFn = func(context.Context) ([]queue.Summary, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			close(firstCalled)
			<-releaseFirst
			return []queue.Summary{{Name: "stale"}}, nil
		}
		return []queue.Summary{{Name: "fresh"}}, nil
	}

	host, qv := newTestQueuesViewWithDrawSignal(t, backend, 2)

	qv.Load()     // call 1 — will become "stale"; blocks inside listFn
	<-firstCalled // call 1's listFn has started (and is now blocked on releaseFirst)

	qv.Load()    // call 2 — "fresh"; proceeds and draws immediately
	<-host.drawn // call 2's draw has landed (guaranteed first: call 1 can't proceed yet)

	if got := qv.table.GetCell(1, 0).Text; got != "fresh" {
		t.Fatalf("row(1,0) after call 2's draw = %q, want %q", got, "fresh")
	}

	close(releaseFirst) // let call 1 (stale) proceed to its now-discarded draw attempt
	<-host.drawn        // call 1's draw attempt has landed (and should have no-opped)

	if got := qv.table.GetCell(1, 0).Text; got != "fresh" {
		t.Errorf("row(1,0) after stale call 1's draw = %q, want unchanged %q", got, "fresh")
	}
}
