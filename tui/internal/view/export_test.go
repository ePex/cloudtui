package view

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/export"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

// newTestExportFlow builds an exportFlow on a host whose QueueUpdateDraw
// signals, so a test can wait for the background export to finish.
func newTestExportFlow(t *testing.T) (*drawSignalingHost, exportFlow) {
	t.Helper()
	host := &drawSignalingHost{fakeViewHost: newFakeViewHost(), drawn: make(chan struct{}, 4)}
	ef := newExportFlow(host, dialog.NewTextPrompt(host), dialog.NewConfirmDialog(host))
	ef.now = func() time.Time { return time.Date(2026, 10, 10, 15, 30, 0, 0, time.UTC) }
	return host, ef
}

func collected(msgs ...queue.Message) func(context.Context) (exportSet, error) {
	return func(context.Context) (exportSet, error) { return exportSet{msgs: msgs}, nil }
}

func messages(n int) []queue.Message {
	out := make([]queue.Message, n)
	for i := range out {
		out[i] = queue.Message{ID: "ID:" + string(rune('a'+i)), RawFields: map[string]any{"text": "body"}}
	}
	return out
}

// submitPrompt presses Enter in the open path prompt.
func submitPrompt(ef exportFlow) {
	prim := ef.prompt.Primitive()
	focusTree(prim)
	send(prim, key(tcell.KeyEnter))
}

// answerConfirm picks No or Yes in the flow's confirm dialog.
func answerConfirm(ef exportFlow, yes bool) {
	prim := ef.confirm.Primitive()
	focusTree(prim)
	if yes {
		send(prim, key(tcell.KeyDown))
	}
	send(prim, key(tcell.KeyEnter))
}

func waitDrawn(t *testing.T, host *drawSignalingHost) {
	t.Helper()
	select {
	case <-host.drawn:
	case <-time.After(5 * time.Second):
		t.Fatal("export didn't finish")
	}
}

func lineCount(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(raw), "\n")
}

func TestExportFlowWritesTheFileAndReports(t *testing.T) {
	host, ef := newTestExportFlow(t)
	path := filepath.Join(t.TempDir(), "orders.ndjson")
	closed := 0

	ef.askPath("orders", path, collected(messages(2)...), func() { closed++ })
	submitPrompt(ef)
	waitDrawn(t, host)

	if n := lineCount(t, path); n != 2 {
		t.Errorf("file has %d lines, want 2", n)
	}
	if want := "Exported 2 message(s) to " + path; host.status != want {
		t.Errorf("status = %q, want %q", host.status, want)
	}
	if closed != 1 {
		t.Errorf("onClose called %d times, want 1 (focus back to the view)", closed)
	}
}

func TestExportFlowRejectsABadPathAndStaysOpen(t *testing.T) {
	_, ef := newTestExportFlow(t)
	missing := filepath.Join(t.TempDir(), "no-such-folder", "x.ndjson")

	ef.askPath("orders", missing, collected(), func() {})
	submitPrompt(ef)

	if !ef.prompt.Visible() {
		t.Error("prompt closed on a path in a missing folder")
	}
}

func TestExportFlowOverwriteNoReturnsToThePrompt(t *testing.T) {
	_, ef := newTestExportFlow(t)
	path := filepath.Join(t.TempDir(), "orders.ndjson")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ef.askPath("orders", path, collected(messages(2)...), func() {})
	submitPrompt(ef)
	if !ef.confirm.Visible() {
		t.Fatal("no overwrite confirm for an existing file")
	}
	if got := renderedScreenText(t, ef.confirm.Primitive(), 52, 8); !strings.Contains(got, "Overwrite the existing file orders.ndjson?") {
		t.Errorf("confirm shows %q, want the question with the file name visible", got)
	}
	answerConfirm(ef, false)

	if !ef.prompt.Visible() {
		t.Error("No didn't reopen the path prompt")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "old\n" {
		t.Errorf("file changed to %q after No", raw)
	}
}

func TestExportFlowOverwriteYesReplacesTheFile(t *testing.T) {
	host, ef := newTestExportFlow(t)
	path := filepath.Join(t.TempDir(), "orders.ndjson")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ef.askPath("orders", path, collected(messages(3)...), func() {})
	submitPrompt(ef)
	answerConfirm(ef, true)
	waitDrawn(t, host)

	if n := lineCount(t, path); n != 3 {
		t.Errorf("file has %d lines after Yes, want the 3 exported", n)
	}
}

func TestExportFlowReportsAFailedCollect(t *testing.T) {
	host, ef := newTestExportFlow(t)
	path := filepath.Join(t.TempDir(), "orders.ndjson")

	ef.askPath("orders", path, func(context.Context) (exportSet, error) {
		return exportSet{}, errors.New("broker down")
	}, func() {})
	submitPrompt(ef)
	waitDrawn(t, host)

	if !strings.Contains(host.status, "Export failed: broker down") {
		t.Errorf("status = %q, want the failure", host.status)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a failed export left a file behind")
	}
}

func TestExportFlowDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.Mkdir(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, ef := newTestExportFlow(t)

	want := filepath.Join(home, "Downloads", "orders-20261010T153000Z.ndjson")
	if got := ef.defaultPath("orders"); got != want {
		t.Errorf("defaultPath() = %q, want %q", got, want)
	}
}

func TestExportedStatus(t *testing.T) {
	tests := []struct {
		name string
		set  exportSet
		want string
	}{
		{"complete", exportSet{msgs: messages(10)}, "Exported 10 message(s) to /x.ndjson"},
		{"pending known", exportSet{msgs: messages(4), total: 1600, why: "the broker's browse limit"},
			"Exported 4 of 1600 message(s) (the broker's browse limit) to /x.ndjson"},
		{"hasMore only", exportSet{msgs: messages(4), hasMore: true, why: "the broker's browse limit"},
			"Exported 4 message(s), more on the queue (the broker's browse limit), to /x.ndjson"},
	}
	for _, tt := range tests {
		if got := exportedStatus("/x.ndjson", tt.set); got != tt.want {
			t.Errorf("%s: exportedStatus() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestCollectQueueBrowsesUpToTheCapAndReadsThePendingCount(t *testing.T) {
	var gotFilter queue.MessageFilter
	b := &fakeQueueBackend{
		summaries: []queue.Summary{{Name: "orders", PendingCount: 1600}},
	}
	b.browseFn = func(_ context.Context, _ string, f queue.MessageFilter) ([]queue.Message, bool, error) {
		gotFilter = f
		return messages(4), true, nil
	}

	set, err := collectQueue(b, "orders")(context.Background())

	if err != nil {
		t.Fatalf("collectQueue() error = %v", err)
	}
	if gotFilter.MaxCount != export.MaxMessages || gotFilter.JMSType != "" {
		t.Errorf("browse filter = %+v, want no filter and MaxCount %d", gotFilter, export.MaxMessages)
	}
	if len(set.msgs) != 4 || set.total != 1600 || !set.hasMore {
		t.Errorf("set = %d msgs, total %d, hasMore %v; want 4, 1600, true", len(set.msgs), set.total, set.hasMore)
	}
}

func TestQueuesViewExportKeyOpensThePrompt(t *testing.T) {
	_, qv := newTestQueuesView(t)
	qv.repaint([]queue.Summary{{Name: "orders"}})
	qv.table.Select(1, 0)

	qv.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))

	if !qv.export.prompt.Visible() {
		t.Error("'e' on a queue row didn't open the export prompt")
	}
}
