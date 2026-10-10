package app

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// fakeClock is a manual clock for opRunner's timers: nothing fires until
// advance moves time past it.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
	// lateTimers makes stopping a timer ineffective, as when a real
	// timer's func is already running when Stop is called.
	lateTimers bool
}

type fakeTimer struct {
	at      time.Time
	f       func()
	stopped bool
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) afterFunc(d time.Duration, f func()) func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return func() {
		c.mu.Lock()
		if !c.lateTimers {
			t.stopped = true
		}
		c.mu.Unlock()
	}
}

// advance moves time forward by d, firing due timers in order. A timer's
// func runs without c.mu held, so it can schedule more timers.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	target := c.now.Add(d)
	for {
		var next *fakeTimer
		for _, t := range c.timers {
			if !t.stopped && !t.at.After(target) && (next == nil || t.at.Before(next.at)) {
				next = t
			}
		}
		if next == nil {
			break
		}
		next.stopped = true
		c.now = next.at
		c.mu.Unlock()
		next.f()
		c.mu.Lock()
	}
	c.now = target
	c.mu.Unlock()
}

// newTestRunner returns a runner on a fake clock that counts redraws.
func newTestRunner() (*opRunner, *fakeClock) {
	c := &fakeClock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	r := &opRunner{
		modal:     dialog.NewProgressModal(),
		afterFunc: c.afterFunc,
		now:       c.Now,
		redraw:    func() {},
	}
	return r, c
}

// drawnText draws r's modal on an 80×24 simulation screen and returns
// the visible text, empty when nothing is drawn.
func drawnText(t *testing.T, r *opRunner) string {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	r.draw(screen)
	screen.Show()
	cells, _, _ := screen.GetContents()
	var b strings.Builder
	for _, c := range cells {
		if len(c.Runes) > 0 {
			b.WriteRune(c.Runes[0])
		}
	}
	return strings.TrimSpace(b.String())
}

func TestOperationShowsOnlyAfterTheDelay(t *testing.T) {
	r, c := newTestRunner()
	r.start("Loading queues", 0, ui.OpAbortable, false)

	c.advance(opShowDelay - time.Millisecond)
	if got := drawnText(t, r); got != "" {
		t.Fatalf("modal drawn before %v: %q", opShowDelay, got)
	}
	if !r.active() {
		t.Error("keys aren't blocked before the modal shows")
	}
	c.advance(time.Millisecond)
	if got := drawnText(t, r); !strings.Contains(got, "Loading queues") {
		t.Fatalf("modal not drawn after %v: %q", opShowDelay, got)
	}
}

func TestFastOperationNeverShows(t *testing.T) {
	r, c := newTestRunner()
	op := r.start("Loading queues", 0, ui.OpAbortable, false)
	c.advance(100 * time.Millisecond)
	op.Finish()

	if r.active() {
		t.Error("still active after Finish")
	}
	c.advance(time.Second)
	if got := drawnText(t, r); got != "" {
		t.Errorf("finished operation drawn later: %q", got)
	}
}

func TestShownOperationStaysUpForTheMinimum(t *testing.T) {
	r, c := newTestRunner()
	op := r.start("Purging orders", 0, ui.OpStoppable, false)
	c.advance(opShowDelay + 50*time.Millisecond) // shown 50ms ago
	op.Finish()

	c.advance(opMinVisible - 50*time.Millisecond - time.Millisecond)
	if !strings.Contains(drawnText(t, r), "Purging orders") {
		t.Fatal("modal closed before the minimum time")
	}
	if !r.active() {
		t.Error("keys unblocked while the modal is still up")
	}
	c.advance(time.Millisecond)
	if got := drawnText(t, r); got != "" {
		t.Errorf("modal still drawn after the minimum time: %q", got)
	}
	if r.active() {
		t.Error("still active after the modal closed")
	}
}

func TestLongOperationClosesAtOnce(t *testing.T) {
	r, c := newTestRunner()
	op := r.start("Purging orders", 0, ui.OpStoppable, false)
	c.advance(time.Second)
	op.Finish()
	if r.active() || drawnText(t, r) != "" {
		t.Error("modal not closed at once after running longer than the minimum")
	}
}

func TestNewOperationTakesOverAVisibleModal(t *testing.T) {
	r, c := newTestRunner()
	first := r.start("Loading queues", 0, ui.OpAbortable, false)
	c.advance(opShowDelay)
	second := r.start("Loading queues again", 0, ui.OpAbortable, false)

	if got := drawnText(t, r); !strings.Contains(got, "Loading queues again") {
		t.Fatalf("replacing operation not drawn at once: %q", got)
	}
	first.Finish()
	if !strings.Contains(drawnText(t, r), "Loading queues again") {
		t.Fatal("the replaced operation's Finish closed the new one")
	}
	c.advance(opMinVisible)
	second.Finish()
	if r.active() {
		t.Error("still active after the replacing operation finished")
	}
}

func TestNewOperationBeforeTheModalShowsWaitsItsOwnDelay(t *testing.T) {
	r, c := newTestRunner()
	r.start("first", 0, ui.OpAbortable, false)
	c.advance(200 * time.Millisecond)
	r.start("second", 0, ui.OpAbortable, false)

	c.advance(100 * time.Millisecond) // 300ms after the first started
	if got := drawnText(t, r); got != "" {
		t.Fatalf("modal drawn before the second operation's own delay: %q", got)
	}
	c.advance(opShowDelay)
	if got := drawnText(t, r); !strings.Contains(got, "second") || strings.Contains(got, "first") {
		t.Errorf("drawn %q, want only the second operation", got)
	}
}

func TestKeysWhileAnOperationRuns(t *testing.T) {
	r, _ := newTestRunner()
	q := tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)
	if ev, ok := r.keyEvent(q); ok || ev != q {
		t.Fatalf("no operation: keyEvent = %v, %v; want the key passed on", ev, ok)
	}

	r.start("Loading queues", 0, ui.OpAbortable, false)
	if ev, ok := r.keyEvent(q); !ok || ev != nil {
		t.Errorf("'q' during an operation: keyEvent = %v, %v; want swallowed", ev, ok)
	}
	ctrlC := tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	if ev, ok := r.keyEvent(ctrlC); !ok || ev != ctrlC {
		t.Errorf("Ctrl+C during an operation: keyEvent = %v, %v; want the same event passed on", ev, ok)
	}
}

func TestEscAbortsALoadAtOnce(t *testing.T) {
	r, c := newTestRunner()
	op := r.start("Loading queues", 0, ui.OpAbortable, false)
	c.advance(opShowDelay) // visible, well inside the minimum time

	esc := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	if ev, ok := r.keyEvent(esc); !ok || ev != nil {
		t.Fatalf("Esc: keyEvent = %v, %v; want handled", ev, ok)
	}
	if op.Ctx().Err() == nil || !op.Stopped() {
		t.Error("Esc didn't cancel the load")
	}
	if r.active() || drawnText(t, r) != "" {
		t.Error("aborted load's modal still up")
	}
	op.Finish() // the view's own Finish afterwards is harmless
}

func TestEscStopsAMutationButWaitsForIt(t *testing.T) {
	tests := []struct {
		total      int
		wantFooter string
	}{
		{10, opFooterStoppingMessage},
		{0, opFooterStoppingRequest},
	}
	for _, tt := range tests {
		r, c := newTestRunner()
		op := r.start("Deleting", tt.total, ui.OpStoppable, false)
		c.advance(opShowDelay)
		if !strings.Contains(drawnText(t, r), opFooterCancel) {
			t.Errorf("total %d: footer before Esc isn't %q", tt.total, opFooterCancel)
		}

		r.keyEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
		if op.Ctx().Err() == nil || !op.Stopped() {
			t.Errorf("total %d: Esc didn't stop the mutation", tt.total)
		}
		if !r.active() || !strings.Contains(drawnText(t, r), tt.wantFooter) {
			t.Errorf("total %d: after Esc, want the modal up saying %q", tt.total, tt.wantFooter)
		}
		c.advance(time.Second)
		op.Finish()
		if r.active() {
			t.Errorf("total %d: still active after the caller finished", tt.total)
		}
	}
}

func TestStepAndDetailAreDrawn(t *testing.T) {
	r, c := newTestRunner()
	op := r.start("Deleting 10 message(s)", 10, ui.OpStoppable, false)
	c.advance(opShowDelay)
	op.Step(3, 1)
	op.SetDetail("Waiting for the broker")

	got := drawnText(t, r)
	for _, want := range []string{"3 of 10 (1 failed)", "Waiting for the broker"} {
		if !strings.Contains(got, want) {
			t.Errorf("modal doesn't show %q: %q", want, got)
		}
	}
}

func TestModalAnimatesOnlyWhileVisible(t *testing.T) {
	r, c := newTestRunner()
	op := r.start("Loading queues", 0, ui.OpAbortable, false)
	c.advance(opShowDelay)
	first := drawnText(t, r)
	c.advance(opFrameInterval)
	if second := drawnText(t, r); second == first {
		t.Fatalf("modal unchanged after a frame interval: %q", first)
	}

	c.advance(time.Second)
	op.Finish()
	c.mu.Lock()
	pending := 0
	for _, tm := range c.timers {
		if !tm.stopped {
			pending++
		}
	}
	c.mu.Unlock()
	if pending != 0 {
		t.Errorf("%d timer(s) still pending after the modal closed, want the animation stopped", pending)
	}
}

func TestLateFrameTimerDoesntRestartTheAnimation(t *testing.T) {
	r, c := newTestRunner()
	redraws := 0
	r.redraw = func() { redraws++ }
	c.lateTimers = true
	op := r.start("Loading queues", 0, ui.OpAbortable, false)
	c.advance(time.Second)
	op.Finish()

	before := redraws
	c.advance(time.Second)
	if redraws > before {
		t.Errorf("%d redraw(s) after the modal closed, want a late frame timer to do nothing", redraws-before)
	}
}

func TestFinishIsIdempotent(t *testing.T) {
	r, c := newTestRunner()
	op := r.start("Purging", 0, ui.OpStoppable, false)
	c.advance(time.Second)
	op.Finish()
	next := r.start("Loading queues", 0, ui.OpAbortable, false)
	op.Finish()
	if !r.active() || next.Ctx().Err() != nil {
		t.Error("finishing an operation twice affected the next one")
	}
}

func TestProgressDemoRunsToTheEnd(t *testing.T) {
	r, _ := newTestRunner()
	if got := r.demo(false, time.Millisecond, time.Millisecond); got != demoFinishedStatus {
		t.Errorf("demo() = %q, want %q", got, demoFinishedStatus)
	}
	if r.active() {
		t.Error("demo left an operation running")
	}
}

func TestProgressDemoEndsOnEsc(t *testing.T) {
	r, _ := newTestRunner()
	result := make(chan string)
	go func() { result <- r.demo(true, time.Hour, time.Hour) }()
	for !r.active() {
		time.Sleep(time.Millisecond)
	}
	r.escape()
	select {
	case got := <-result:
		if got != demoCancelledStatus {
			t.Errorf("demo() = %q, want %q", got, demoCancelledStatus)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("demo didn't end after Esc")
	}
}

func TestAppBlocksKeysWhileAnOperationRuns(t *testing.T) {
	a := New(config.Default())
	a.SwitchTo("home")
	op := a.StartOperation("Loading queues", 0, ui.OpAbortable)

	if ev := a.onGlobalKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)); ev != nil {
		t.Errorf("'s' during an operation passed on: %v", ev)
	}
	if name, _ := a.pages.GetFrontPage(); name != "home" {
		t.Errorf("front page = %q, want home: the 's' hotkey ran", name)
	}
	a.onGlobalKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if !op.Stopped() {
		t.Error("Esc didn't reach the operation")
	}
	if ev := a.onGlobalKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)); ev != nil {
		t.Errorf("'s' after the load was aborted: onGlobalKey = %v, want handled as the hotkey", ev)
	}
	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page = %q, want settings once keys work again", name)
	}
}

func TestPromptProgressDemoStartsTheDemo(t *testing.T) {
	for _, cmd := range []string{"progress-demo", "progress-demo spinner"} {
		a := New(config.Default())
		a.prompt.SetText(cmd)
		a.onPromptDone(tcell.KeyEnter)
		deadline := time.Now().Add(5 * time.Second)
		for !a.ops.active() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !a.ops.active() {
			t.Fatalf(":%s didn't start an operation", cmd)
		}
		a.ops.mu.Lock()
		forced := a.ops.cur.forceSpinner
		a.ops.mu.Unlock()
		if want := strings.HasSuffix(cmd, "spinner"); forced != want {
			t.Errorf(":%s: forceSpinner = %v, want %v", cmd, forced, want)
		}
		a.ops.escape()
	}
}
