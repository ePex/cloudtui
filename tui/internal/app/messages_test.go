package app

import (
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

func newTestMessagesView(t *testing.T) *messagesView {
	t.Helper()
	a := New(config.Default())
	return newMessagesView(a)
}

// newTestMessagesViewWithMsgs builds a messagesView already populated via
// repaint, as it would be after a real load().
func newTestMessagesViewWithMsgs(t *testing.T, msgs []queue.Message) *messagesView {
	t.Helper()
	mv := newTestMessagesView(t)
	mv.repaint(msgs)
	return mv
}

func TestMessagesViewHeaderLabels(t *testing.T) {
	mv := newTestMessagesView(t)
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
	mv := newTestMessagesView(t)
	if got, want := mv.table.GetColumnCount(), 6; got != want {
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

func TestMessagesViewShortcutsIncludeMultiSelectKeys(t *testing.T) {
	mv := newTestMessagesView(t)
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

func TestMessagesViewDeleteMarkedNoopWithoutMarks(t *testing.T) {
	a := New(config.Default())
	backend := &fakeQueueBackend{}
	a.backend = backend
	mv := newMessagesView(a)
	mv.queueName = "orders"
	mv.repaint([]queue.Message{{ID: "msg-1", Timestamp: time.Unix(1, 0)}})

	mv.deleteMarked()

	if a.confirmVisible {
		t.Error("deleteMarked() with no marks should not open the confirm dialog")
	}
}

func TestMessagesViewMoveMarkedNoopWithoutMarks(t *testing.T) {
	a := New(config.Default())
	backend := &fakeQueueBackend{}
	a.backend = backend
	mv := newMessagesView(a)
	mv.queueName = "orders"
	mv.repaint([]queue.Message{{ID: "msg-1", Timestamp: time.Unix(1, 0)}})

	mv.moveMarked()

	if a.movePickerVisible {
		t.Error("moveMarked() with no marks should not open the move picker")
	}
}
