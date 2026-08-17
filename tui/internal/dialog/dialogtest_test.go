package dialog

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// renderedScreenText draws prim into a width×height simulation screen
// and returns its visible text. Duplicated from internal/app's
// queues_test.go (unexported, test-only, used by both sides of the
// internal/dialog split — not worth a shared package for one function).
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

// fakeQueueBackend is a zero-behavior queue.Backend, used only as
// testHost's default Backend() — no test moved into this package
// calls host.Backend() today (only movePicker/sendMessageOverlay do,
// neither has a dedicated test), so every method here is unexercised.
// Duplicated from internal/app's queues_test.go for the same reason
// as renderedScreenText above.
type fakeQueueBackend struct{}

func (f *fakeQueueBackend) List(_ context.Context) ([]queue.Summary, error) { return nil, nil }
func (f *fakeQueueBackend) BrowseMessages(_ context.Context, _ string, _ queue.MessageFilter) ([]queue.Message, error) {
	return nil, nil
}
func (f *fakeQueueBackend) PurgeQueue(_ context.Context, _ string) error        { return nil }
func (f *fakeQueueBackend) RemoveMessage(_ context.Context, _, _ string) error  { return nil }
func (f *fakeQueueBackend) MoveMessage(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeQueueBackend) MoveAllMessages(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}
func (f *fakeQueueBackend) SendMessage(_ context.Context, _, _ string) error { return nil }
func (f *fakeQueueBackend) DeleteMessages(_ context.Context, _ string, _ queue.MessageFilter) (int, error) {
	return 0, nil
}
func (f *fakeQueueBackend) MoveMessages(_ context.Context, _, _ string, _ queue.MessageFilter) (int, error) {
	return 0, nil
}
