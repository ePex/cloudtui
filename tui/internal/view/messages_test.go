package view

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

func newTestMessagesView(t *testing.T) (*fakeViewHost, *dialog.ConfirmDialog, *dialog.MovePicker, *MessagesView) {
	t.Helper()
	host := newFakeViewHost()
	messageFilter := dialog.NewMessageFilter(host)
	sendMessage := dialog.NewSendMessageOverlay(host)
	confirm := dialog.NewConfirmDialog(host)
	movePicker := dialog.NewMovePicker(host)
	return host, confirm, movePicker, NewMessagesView(host, messageFilter, sendMessage, confirm, movePicker, func(string, queue.Message) {})
}

// newTestMessagesViewWithMsgs builds a MessagesView already populated via
// repaint, as it would be after a real load().
func newTestMessagesViewWithMsgs(t *testing.T, msgs []queue.Message) *MessagesView {
	t.Helper()
	_, _, _, mv := newTestMessagesView(t)
	mv.repaint(msgs)
	return mv
}

func TestMessagesViewHeaderLabels(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	want := []string{"", "ID", "TYPE", "CORR.ID", "TIMESTAMP", "PREVIEW"}
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
	_, _, _, mv := newTestMessagesView(t)
	if got, want := mv.table.GetColumnCount(), 6; got != want {
		t.Errorf("GetColumnCount() = %d, want %d", got, want)
	}
}

func TestMessagesViewShortcutRPresent(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	for _, s := range mv.Shortcuts() {
		if s.Key == "r" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"r\"")
}

func TestMessagesViewShortcutsIncludeMultiSelectKeys(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	want := []string{"space", "a", "n", "d", "m"}
	for _, k := range want {
		found := false
		for _, s := range mv.Shortcuts() {
			if s.Key == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Shortcuts() missing key %q", k)
		}
	}
}

func TestMessagesViewWrapShortcutPresent(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	for _, s := range mv.Shortcuts() {
		if s.Key == "W" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"W\"")
}

func TestMessagesViewWrapTogglesAtBottomEdge(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{{ID: "a"}, {ID: "b"}, {ID: "c"}})
	mv.table.Select(3, 0) // last message row

	capture := mv.table.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'W', tcell.ModNone))
	capture(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))

	if row, _ := mv.table.GetSelection(); row != 1 {
		t.Errorf("selection after wrap = %d, want 1 (wrapped to first message row)", row)
	}
}

// TestMessagesViewShortcutsFitTopBar guards against shortcut entries
// silently clipping off the bottom of the context panel: the top bar's
// height is fixed (ui.ShortcutPanelRows), not scrollable, so a
// Shortcuts() list longer than that is invisible rather than an error —
// exactly what happened live when '/' and 'f' were added without bumping
// ShortcutPanelRows to match (found via manual verification, not tests).
func TestMessagesViewShortcutsFitTopBar(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	if got := len(mv.Shortcuts()); got > ui.ShortcutPanelRows {
		t.Errorf("len(Shortcuts()) = %d, want <= %d (ui.ShortcutPanelRows) or entries clip off the context panel", got, ui.ShortcutPanelRows)
	}
}

func TestMessagesViewShortcutsIncludeFilterKeys(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	want := []string{"/", "f"}
	for _, k := range want {
		found := false
		for _, s := range mv.Shortcuts() {
			if s.Key == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Shortcuts() missing key %q", k)
		}
	}
}

func TestMessagesViewQuickSearchFilters(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	mv.applyQuickSearch("order")
	mv.repaint([]queue.Message{
		{ID: "msg-1", JMSType: "order-created", Timestamp: time.Unix(2, 0)},
		{ID: "msg-2", JMSType: "invoice-created", Timestamp: time.Unix(1, 0)},
	})

	if got := mv.table.GetRowCount(); got != 2 { // header + 1 match
		t.Errorf("row count = %d, want 2 (header + 1 match)", got)
	}
	if len(mv.msgs) != 1 || mv.msgs[0].ID != "msg-1" {
		t.Errorf("mv.msgs = %v, want just msg-1", mv.msgs)
	}
}

func TestMessagesViewQuickSearchMatchesPreview(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	mv.applyQuickSearch("hello")
	mv.repaint([]queue.Message{
		{ID: "msg-1", Preview: "hello world", Timestamp: time.Unix(1, 0)},
		{ID: "msg-2", Preview: "goodbye", Timestamp: time.Unix(2, 0)},
	})

	if len(mv.msgs) != 1 || mv.msgs[0].ID != "msg-1" {
		t.Errorf("mv.msgs = %v, want just msg-1 (matches preview)", mv.msgs)
	}
}

func TestMessagesViewQuickSearchPersistsAfterRepaint(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	mv.applyQuickSearch("order")
	mv.repaint([]queue.Message{{ID: "msg-1", JMSType: "order-created", Timestamp: time.Unix(1, 0)}})
	// Second repaint with new data — quick search must still apply.
	mv.repaint([]queue.Message{
		{ID: "msg-1", JMSType: "order-created", Timestamp: time.Unix(1, 0)},
		{ID: "msg-2", JMSType: "invoice-created", Timestamp: time.Unix(2, 0)},
	})

	if got := mv.table.GetRowCount(); got != 2 { // header + 1 match
		t.Errorf("row count after second repaint = %d, want 2 (header + 1 match)", got)
	}
}

func TestMessagesViewQuickSearchClear(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	mv.applyQuickSearch("order")
	mv.repaint([]queue.Message{
		{ID: "msg-1", JMSType: "order-created", Timestamp: time.Unix(1, 0)},
		{ID: "msg-2", JMSType: "invoice-created", Timestamp: time.Unix(2, 0)},
	})
	mv.applyQuickSearch("")

	if got := mv.table.GetRowCount(); got != 3 { // header + 2 rows
		t.Errorf("row count after clear = %d, want 3", got)
	}
}

func TestMessagesViewMarkAllOnlyMarksSearchFilteredRows(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	mv.applyQuickSearch("order")
	mv.repaint([]queue.Message{
		{ID: "msg-1", JMSType: "order-created", Timestamp: time.Unix(2, 0)},
		{ID: "msg-2", JMSType: "invoice-created", Timestamp: time.Unix(1, 0)},
	})

	mv.markAll()

	if len(mv.marked) != 1 || !mv.marked["msg-1"] {
		t.Errorf("marked = %v, want just msg-1 (search-filtered out msg-2)", mv.marked)
	}
}

func TestMessagesViewTitleUpdatesWithFilterAndSearch(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	mv.queueName = "orders"
	mv.updateTitle()
	if got, want := mv.table.GetTitle(), " Messages — orders (filter: max=500) "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}

	mv.filter = queue.MessageFilter{JMSType: "order-created"}
	mv.updateTitle()
	if got, want := mv.table.GetTitle(), " Messages — orders (filter: type=order-created max=500) "; got != want {
		t.Errorf("title with filter = %q, want %q", got, want)
	}

	mv.applyQuickSearch("foo")
	if got, want := mv.table.GetTitle(), " Messages — orders (filter: type=order-created max=500) [search: foo] "; got != want {
		t.Errorf("title with filter and search = %q, want %q", got, want)
	}

	mv.filter = queue.MessageFilter{JMSType: "order-created", MaxCount: 25}
	mv.updateTitle()
	if got, want := mv.table.GetTitle(), " Messages — orders (filter: type=order-created max=25) [search: foo] "; got != want {
		t.Errorf("title with explicit max count = %q, want %q", got, want)
	}
}

func TestWithDefaultMaxCount(t *testing.T) {
	tests := []struct {
		name string
		in   queue.MessageFilter
		want int
	}{
		{"unset defaults", queue.MessageFilter{}, defaultBrowseMaxCount},
		{"negative defaults", queue.MessageFilter{MaxCount: -1}, defaultBrowseMaxCount},
		{"positive unchanged", queue.MessageFilter{MaxCount: 25}, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withDefaultMaxCount(tt.in)
			if got.MaxCount != tt.want {
				t.Errorf("withDefaultMaxCount(%+v).MaxCount = %d, want %d", tt.in, got.MaxCount, tt.want)
			}
		})
	}

	original := queue.MessageFilter{JMSType: "order-created"}
	if got := withDefaultMaxCount(original); got.JMSType != "order-created" {
		t.Errorf("withDefaultMaxCount changed JMSType: got %q", got.JMSType)
	}
	if original.MaxCount != 0 {
		t.Errorf("withDefaultMaxCount mutated its input: MaxCount = %d, want 0", original.MaxCount)
	}
}

func TestMessagesViewToggleMarkMarksAndUnmarks(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{
		{ID: "msg-1", Timestamp: time.Unix(3, 0)},
		{ID: "msg-2", Timestamp: time.Unix(2, 0)},
	})
	mv.table.Select(1, 0) // row 1 = msg-1 (newest first)

	mv.toggleMark()
	if !mv.marked["msg-1"] {
		t.Fatal("toggleMark() did not mark msg-1")
	}
	if got := mv.table.GetCell(1, 0).Text; got != "✓" {
		t.Errorf("marker cell = %q, want \"✓\"", got)
	}

	// toggleMark advances the cursor to row 2; toggle again unmarks... no,
	// it marks row 2. Move back to row 1 explicitly to test unmarking it.
	mv.table.Select(1, 0)
	mv.toggleMark()
	if mv.marked["msg-1"] {
		t.Fatal("second toggleMark() on msg-1 should have unmarked it")
	}
	if got := mv.table.GetCell(1, 0).Text; got != " " {
		t.Errorf("marker cell after unmark = %q, want \" \"", got)
	}
}

func TestMessagesViewToggleMarkAdvancesCursor(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{
		{ID: "msg-1", Timestamp: time.Unix(2, 0)},
		{ID: "msg-2", Timestamp: time.Unix(1, 0)},
	})
	mv.table.Select(1, 0)
	mv.toggleMark()
	if row, _ := mv.table.GetSelection(); row != 2 {
		t.Errorf("cursor row after toggleMark() = %d, want 2", row)
	}
}

func TestMessagesViewToggleMarkSkipsMessagesWithoutID(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{
		{ID: "", Timestamp: time.Unix(1, 0)},
	})
	mv.table.Select(1, 0)
	mv.toggleMark()
	if len(mv.marked) != 0 {
		t.Errorf("toggleMark() on ID-less message marked something: %v", mv.marked)
	}
}

func TestMessagesViewMarkAllSkipsMessagesWithoutID(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{
		{ID: "msg-1", Timestamp: time.Unix(3, 0)},
		{ID: "", Timestamp: time.Unix(2, 0)},
		{ID: "msg-3", Timestamp: time.Unix(1, 0)},
	})
	mv.markAll()
	if got, want := len(mv.marked), 2; got != want {
		t.Fatalf("marked count = %d, want %d", got, want)
	}
	if !mv.marked["msg-1"] || !mv.marked["msg-3"] {
		t.Errorf("markAll() did not mark expected IDs: %v", mv.marked)
	}
}

func TestMessagesViewClearMarksResetsAll(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{
		{ID: "msg-1", Timestamp: time.Unix(1, 0)},
	})
	mv.markAll()
	mv.clearMarks()
	if len(mv.marked) != 0 {
		t.Errorf("marked after clearMarks() = %v, want empty", mv.marked)
	}
	if got := mv.table.GetCell(1, 0).Text; got != " " {
		t.Errorf("marker cell after clearMarks() = %q, want \" \"", got)
	}
}

func TestMessagesViewMarkedIDsMatchesDisplayOrder(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{
		{ID: "old", Timestamp: time.Unix(1, 0)},
		{ID: "new", Timestamp: time.Unix(2, 0)},
	})
	mv.markAll() // display order is newest-first: "new", then "old"

	ids := mv.markedIDs()
	want := []string{"new", "old"}
	if len(ids) != len(want) {
		t.Fatalf("markedIDs() = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("markedIDs()[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestMessagesViewRepaintClearsMarks(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{
		{ID: "msg-1", Timestamp: time.Unix(1, 0)},
	})
	mv.markAll()
	mv.repaint([]queue.Message{{ID: "msg-1", Timestamp: time.Unix(1, 0)}})
	if len(mv.marked) != 0 {
		t.Errorf("marked after repaint() = %v, want empty (marks reset on reload)", mv.marked)
	}
}

func TestMessagesViewDeleteMarkedNoopOnEmptyList(t *testing.T) {
	_, confirm, _, mv := newTestMessagesView(t)
	mv.queueName = "orders"
	mv.repaint(nil)

	mv.deleteMarked()

	if confirm.Visible() {
		t.Error("deleteMarked() on an empty list should not open the confirm dialog")
	}
}

func TestMessagesViewMoveMarkedNoopOnEmptyList(t *testing.T) {
	_, _, movePicker, mv := newTestMessagesView(t)
	mv.queueName = "orders"
	mv.repaint(nil)

	mv.moveMarked()

	if movePicker.Visible() {
		t.Error("moveMarked() on an empty list should not open the move picker")
	}
}

func TestMessagesViewDeleteFallsBackToCursorWhenNothingMarked(t *testing.T) {
	_, confirm, _, mv := newTestMessagesView(t)
	mv.queueName = "orders"
	mv.repaint([]queue.Message{{ID: "msg-1", Timestamp: time.Unix(1, 0)}})
	mv.table.Select(1, 0) // cursor on the only row; nothing marked

	mv.deleteMarked()

	if !confirm.Visible() {
		t.Fatal("deleteMarked() with a cursor row but no marks should open the confirm dialog")
	}
	want := `Delete message from "orders"?`
	if got := renderedScreenText(t, confirm.Primitive(), 60, 8); !strings.Contains(got, want) {
		t.Errorf("rendered confirm dialog = %q, want it to contain %q", got, want)
	}
}

func TestMessagesViewMoveFallsBackToCursorWhenNothingMarked(t *testing.T) {
	_, _, movePicker, mv := newTestMessagesView(t)
	mv.queueName = "orders"
	mv.repaint([]queue.Message{{ID: "msg-1", Timestamp: time.Unix(1, 0)}})
	mv.table.Select(1, 0) // cursor on the only row; nothing marked

	mv.moveMarked()

	if !movePicker.Visible() {
		t.Error("moveMarked() with a cursor row but no marks should open the move picker")
	}
}

func TestMessagesViewTargetIDsPrefersMarkedOverCursor(t *testing.T) {
	mv := newTestMessagesViewWithMsgs(t, []queue.Message{
		{ID: "new", Timestamp: time.Unix(2, 0)},
		{ID: "old", Timestamp: time.Unix(1, 0)},
	})
	mv.table.Select(1, 0) // cursor on "new"
	mv.marked = map[string]bool{"old": true}

	ids := mv.targetIDs()
	if len(ids) != 1 || ids[0] != "old" {
		t.Errorf("targetIDs() = %v, want [\"old\"] (the marked set, not the cursor row)", ids)
	}
}

func TestMessagesViewDeleteFallsBackNoopWhenCursorRowHasNoID(t *testing.T) {
	_, confirm, _, mv := newTestMessagesView(t)
	mv.queueName = "orders"
	mv.repaint([]queue.Message{{ID: "", Timestamp: time.Unix(1, 0)}})
	mv.table.Select(1, 0)

	mv.deleteMarked()

	if confirm.Visible() {
		t.Error("deleteMarked() should not fall back to a cursor row with no ID")
	}
}

// TestMessagesViewRepaintScrollsToTopWithManyRows guards against the same
// bug fixed for queuesView (spec/11-bugfix-queues-scroll-to-top):
// tview.Table's "track end" auto-scroll latches on during the table's
// first, still-empty draw and stays latched through repaint, scrolling a
// long list to the bottom instead of the top.
func TestMessagesViewRepaintScrollsToTopWithManyRows(t *testing.T) {
	_, _, _, mv := newTestMessagesView(t)
	mv.table.SetRect(0, 0, 60, 15) // fewer visible rows than messages below

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)

	// First draw while the table is still empty (header only), mirroring
	// the real sequence: OpenMessages draws before the async load returns.
	mv.table.Draw(screen)

	msgs := make([]queue.Message, 50)
	for i := range msgs {
		msgs[i] = queue.Message{ID: fmt.Sprintf("id-%02d", i), Timestamp: time.Unix(int64(i), 0)}
	}
	mv.repaint(msgs)

	// The redraw that follows repaint via QueueUpdateDraw.
	mv.table.Draw(screen)

	if row, _ := mv.table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}
