package dialog

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
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
func (f *fakeQueueBackend) SendMessage(_ context.Context, _ string, _ queue.SendMessageRequest) error {
	return nil
}
func (f *fakeQueueBackend) DeleteMessages(_ context.Context, _ string, _ queue.MessageFilter) (int, error) {
	return 0, nil
}
func (f *fakeQueueBackend) MoveMessages(_ context.Context, _, _ string, _ queue.MessageFilter) (int, error) {
	return 0, nil
}

// ── Live theme switch regression ─────────────────────────────────────────

// paletteColors returns every color value in p (all its string fields),
// as tcell colors.
func paletteColors(p config.Palette) map[tcell.Color]string {
	colors := map[tcell.Color]string{}
	v := reflect.ValueOf(p)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.String && v.Field(i).String() != "" {
			colors[tcell.GetColor(v.Field(i).String())] = v.Type().Field(i).Name
		}
	}
	return colors
}

// staleColors draws prim and returns a description of every cell still
// carrying a color from old that new doesn't also use — i.e. a color that
// can only have survived from before the switch.
func staleColors(t *testing.T, prim tview.Primitive, width, height int, old, new config.Palette) []string {
	t.Helper()
	oldOnly := paletteColors(old)
	for c := range paletteColors(new) {
		delete(oldOnly, c)
	}

	prim.SetRect(0, 0, width, height)
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)
	prim.Draw(screen)
	screen.Show()

	var stale []string
	cells, w, _ := screen.GetContents()
	for i, c := range cells {
		fg, bg, _ := c.Style.Decompose()
		r := ' '
		if len(c.Runes) > 0 {
			r = c.Runes[0]
		}
		for _, col := range []tcell.Color{fg, bg} {
			if name, ok := oldOnly[col]; ok {
				stale = append(stale, fmt.Sprintf("(%d,%d) %q uses old %s", i%w, i/w, r, name))
			}
		}
	}
	return stale
}

// TestDialogsFullyRecolorOnLiveThemeSwitch builds every overlay while
// tview.Styles and the host's palette hold "dark" (as at startup) and
// opens it with some content. It then switches the host to "cyberpunk"
// and calls ApplyPalette, which is what a live theme switch does. Then it
// opens the overlay again, since in the app every overlay except the
// theme picker is closed while you switch (you switch *from* the theme
// picker) and is only seen again after reopening. It fails if any drawn
// cell still carries a dark-only color. The theme picker is checked
// without reopening, since it stays on screen across the switch.
func TestDialogsFullyRecolorOnLiveThemeSwitch(t *testing.T) {
	dark, _ := config.PaletteForTheme("dark")
	cyber, _ := config.PaletteForTheme("cyberpunk")

	type overlay interface {
		ui.Themeable
		Primitive() tview.Primitive
	}
	// build constructs the overlay and returns it plus a func that opens
	// it with content (called once before the switch and, unless
	// staysOpen, once after).
	tests := []struct {
		name      string
		staysOpen bool
		build     func(t *testing.T, host *testHost) (overlay, func())
	}{
		{"ConfirmDialog", false, func(t *testing.T, host *testHost) (overlay, func()) {
			c := NewConfirmDialog(host)
			return c, func() { c.Show("Delete?", func() {}) }
		}},
		{"MovePicker", false, func(t *testing.T, host *testHost) (overlay, func()) {
			mp := NewMovePicker(host)
			// Filled directly: Show loads the queues in a goroutine.
			return mp, func() {
				mp.queues = []string{"orders", "dlq.orders", "billing"}
				mp.fillList("")
				mp.search.SetText("ord")
			}
		}},
		{"SendMessageOverlay", false, func(t *testing.T, host *testHost) (overlay, func()) {
			sm := NewSendMessageOverlay(host, NewSnippetPicker(host, snippet.NewStore(t.TempDir())), NewConfirmDialog(host))
			return sm, func() {
				sm.Show("orders", func() {})
				sm.jmsTypeItem.SetText("OrderCreated")
				sm.bodyItem.SetText("{}", false)
			}
		}},
		{"SnippetPicker", false, func(t *testing.T, host *testHost) (overlay, func()) {
			root := t.TempDir()
			writeSnippetFile(t, root, "orders/created.json", "x")
			writeSnippetFile(t, root, "ping.txt", "x")
			sp := NewSnippetPicker(host, snippet.NewStore(root))
			return sp, func() { sp.Show(func(snippet.Snippet) {}, func() {}) }
		}},
		{"SnippetSaveDialog", false, func(t *testing.T, host *testHost) (overlay, func()) {
			sd := NewSnippetSaveDialog(host, snippet.NewStore(t.TempDir()), NewConfirmDialog(host))
			return sd, func() {
				sd.Show(snippet.Snippet{Body: "x"}, func() {})
				sd.nameItem.SetText("orders/created.json")
			}
		}},
		{"ConnManager", false, func(t *testing.T, host *testHost) (overlay, func()) {
			cm := NewConnManager(host, NewConfirmDialog(host))
			return cm, cm.Show
		}},
		{"ConnEditor", false, func(t *testing.T, host *testHost) (overlay, func()) {
			cm := NewConnManager(host, NewConfirmDialog(host))
			ce := NewConnEditor(host, cm)
			cm.SetEditor(ce)
			return ce, func() { ce.Show(host.cfg.Connections[0], false, host.cfg.Connections[0].Name) }
		}},
		{"MessageFilter", false, func(t *testing.T, host *testHost) (overlay, func()) {
			mf := NewMessageFilter(host)
			return mf, mf.Show
		}},
		{"JMSTypePrompt", false, func(t *testing.T, host *testHost) (overlay, func()) {
			jp := NewJMSTypePrompt(host)
			return jp, func() { jp.Show("Purge", "orders", func(string) {}, func() {}) }
		}},
		{"TimeRangeModal", false, func(t *testing.T, host *testHost) (overlay, func()) {
			tm := NewTimeRangeModal(host)
			return tm, func() { tm.Show(ui.TimeRange{}, func(ui.TimeRange) {}) }
		}},
		{"DatadogEditor", false, func(t *testing.T, host *testHost) (overlay, func()) {
			de := NewDatadogEditor(host)
			return de, de.Show
		}},
		{"AMQManagerSettingsEditor", false, func(t *testing.T, host *testHost) (overlay, func()) {
			e := NewAMQManagerSettingsEditor(host)
			return e, e.Show
		}},
		{"ThemePicker", true, func(t *testing.T, host *testHost) (overlay, func()) {
			tp := NewThemePicker(host)
			return tp, tp.Show
		}},
		{"TextPrompt", false, func(t *testing.T, host *testHost) (overlay, func()) {
			tp := NewTextPrompt(host)
			return tp, func() { tp.Show("New folder", "Name:", "orders", func(string) error { return nil }, func() {}) }
		}},
		{"SnippetEditor", false, func(t *testing.T, host *testHost) (overlay, func()) {
			se := NewSnippetEditor(host, snippet.NewStore(t.TempDir()), NewConfirmDialog(host))
			return se, func() {
				se.ShowEdit("orders/created.json", snippet.Snippet{JMSType: "OrderCreated", Body: "{\n  \"id\": 1\n}"}, func(string) {}, func() {})
			}
		}},
		{"AWSProfilesPicker", false, func(t *testing.T, host *testHost) (overlay, func()) {
			ap := NewAWSProfilesPicker(host)
			return ap, func() {
				ap.Show()
				ap.filterInput.SetText("dev")
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := tview.Styles
			t.Cleanup(func() { tview.Styles = saved })
			ui.ApplyTviewStyles(dark)
			host := newTestHost()
			host.cfg.Colors = dark

			o, open := tt.build(t, host)
			o.ApplyPalette(dark) // as App.New does at startup
			open()

			ui.ApplyTviewStyles(cyber) // the live switch: reapplyTheme
			host.cfg.Colors = cyber
			o.ApplyPalette(cyber)
			if !tt.staysOpen {
				open()
			}

			if stale := staleColors(t, o.Primitive(), 90, 30, dark, cyber); len(stale) > 0 {
				t.Errorf("%d cells keep dark-only colors after the switch, e.g. %s", len(stale), strings.Join(stale[:min(len(stale), 5)], "; "))
			}
		})
	}
}
