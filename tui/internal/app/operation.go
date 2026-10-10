package app

import (
	"context"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// Timing of the progress modal (spec-wip fe-progress-modal): it shows only
// once an operation has run for opShowDelay, then stays up at least
// opMinVisible so it doesn't flash; opFrameInterval paces its animation.
const (
	opShowDelay     = 250 * time.Millisecond
	opMinVisible    = 400 * time.Millisecond
	opFrameInterval = 100 * time.Millisecond
)

// Footers of the progress modal.
const (
	opFooterCancel          = "Esc to cancel"
	opFooterStoppingMessage = "Stopping after the current message…"
	opFooterStoppingRequest = "Stopping after the current request…"
)

// opRunner runs the app's blocking operations, one at a time, and draws
// the progress modal for the current one. Its state is guarded by mu so
// operations can report from any goroutine; the modal itself is only
// touched in draw, on the UI goroutine.
type opRunner struct {
	mu    sync.Mutex
	modal *dialog.ProgressModal
	cur   *operation
	// shown is the operation the modal was last Reset for.
	shown *operation

	// afterFunc runs f after d and returns a func that cancels it
	// (time.AfterFunc in the app; fakes in tests).
	afterFunc func(d time.Duration, f func()) (stop func())
	now       func() time.Time
	// redraw asks for a screen redraw without blocking; called with mu
	// released.
	redraw func()
}

// newOpRunner returns a runner using real timers.
func newOpRunner(modal *dialog.ProgressModal, redraw func()) *opRunner {
	return &opRunner{
		modal: modal,
		afterFunc: func(d time.Duration, f func()) func() {
			t := time.AfterFunc(d, f)
			return func() { t.Stop() }
		},
		now:    time.Now,
		redraw: redraw,
	}
}

// operation is one run of the progress modal; it implements ui.Operation.
type operation struct {
	r            *opRunner
	title        string
	total        int
	kind         ui.OpKind
	forceSpinner bool
	ctx          context.Context
	cancel       context.CancelFunc

	// Guarded by r.mu.
	done, failed int
	detail       string
	stopped      bool
	finished     bool
	visible      bool
	shownAt      time.Time
	frame        int
	stopTimers   []func()
}

var _ ui.Operation = (*operation)(nil)

// start begins an operation, replacing the current one. If the current
// one's modal is already showing, the new one takes it over at once
// (no gap, no second delay); otherwise it shows after opShowDelay.
func (r *opRunner) start(title string, total int, kind ui.OpKind, forceSpinner bool) *operation {
	ctx, cancel := context.WithCancel(context.Background())
	op := &operation{r: r, title: title, total: total, kind: kind, forceSpinner: forceSpinner, ctx: ctx, cancel: cancel}

	r.mu.Lock()
	old := r.cur
	r.cur = op
	if old != nil {
		old.stopAllTimers()
		if old.visible {
			op.visible = true
			op.shownAt = old.shownAt
		}
	}
	if op.visible {
		r.scheduleFrameLocked(op)
	} else {
		op.stopTimers = append(op.stopTimers, r.afterFunc(opShowDelay, func() { r.show(op) }))
	}
	r.mu.Unlock()
	if op.visible {
		r.redraw()
	}
	return op
}

// show makes op's modal visible, if op is still running.
func (r *opRunner) show(op *operation) {
	r.mu.Lock()
	if r.cur != op || op.visible {
		r.mu.Unlock()
		return
	}
	op.visible = true
	op.shownAt = r.now()
	r.scheduleFrameLocked(op)
	r.mu.Unlock()
	r.redraw()
}

// scheduleFrameLocked queues op's next animation frame. r.mu must be held.
func (r *opRunner) scheduleFrameLocked(op *operation) {
	op.stopTimers = append(op.stopTimers, r.afterFunc(opFrameInterval, func() {
		r.mu.Lock()
		if r.cur != op || !op.visible {
			r.mu.Unlock()
			return
		}
		op.frame++
		r.scheduleFrameLocked(op)
		r.mu.Unlock()
		r.redraw()
	}))
}

// stopAllTimers cancels op's pending timers. r.mu must be held.
func (op *operation) stopAllTimers() {
	for _, stop := range op.stopTimers {
		stop()
	}
	op.stopTimers = nil
}

// active reports whether an operation is running or its modal is still
// up; keys are blocked while it is.
func (r *opRunner) active() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cur != nil
}

// escape handles Esc for the current operation: an abortable one is
// cancelled and closed at once; a stoppable one is cancelled and says it's
// stopping, and closes when its caller finishes it.
func (r *opRunner) escape() {
	r.mu.Lock()
	op := r.cur
	if op == nil || op.stopped || op.finished {
		r.mu.Unlock()
		return
	}
	op.stopped = true
	op.cancel()
	if op.kind == ui.OpAbortable {
		op.finished = true
		r.closeLocked(op)
	}
	r.mu.Unlock()
	r.redraw()
}

// closeLocked removes op as the current operation and stops its timers.
// r.mu must be held.
func (r *opRunner) closeLocked(op *operation) {
	op.stopAllTimers()
	if r.cur == op {
		r.cur = nil
	}
}

// keyEvent is the app's key handling while an operation runs: Esc goes to
// escape, Ctrl+C is passed on (so tview still quits), everything else is
// swallowed. Returns the event unchanged, with ok false, when no
// operation runs.
func (r *opRunner) keyEvent(event *tcell.EventKey) (_ *tcell.EventKey, ok bool) {
	if !r.active() {
		return event, false
	}
	switch event.Key() {
	case tcell.KeyCtrlC:
		return event, true
	case tcell.KeyEscape:
		r.escape()
	}
	return nil, true
}

// draw paints the current operation's modal, centered, on top of
// everything else. Runs on the UI goroutine, after the main draw.
func (r *opRunner) draw(screen tcell.Screen) {
	r.mu.Lock()
	op := r.cur
	if op == nil || !op.visible {
		r.shown = nil
		r.mu.Unlock()
		return
	}
	m := r.modal
	if r.shown != op {
		m.Reset(op.title, op.total)
		r.shown = op
	}
	m.SetProgress(op.done, op.failed)
	m.SetDetail(op.detail)
	m.SetFooter(op.footer())
	m.SetForceSpinner(op.forceSpinner)
	m.SetFrame(op.frame)
	r.mu.Unlock()

	w, h := screen.Size()
	width, height := min(dialog.ProgressModalWidth, w), min(dialog.ProgressModalHeight, h)
	m.SetRect((w-width)/2, (h-height)/2, width, height)
	m.Draw(screen)
}

// footer is op's last modal line. r.mu must be held.
func (op *operation) footer() string {
	switch {
	case !op.stopped:
		return opFooterCancel
	case op.total > 0:
		return opFooterStoppingMessage
	default:
		return opFooterStoppingRequest
	}
}

func (op *operation) Ctx() context.Context { return op.ctx }

func (op *operation) Stopped() bool {
	op.r.mu.Lock()
	defer op.r.mu.Unlock()
	return op.stopped
}

func (op *operation) Step(done, failed int) {
	op.update(func() { op.done, op.failed = done, failed })
}

func (op *operation) SetDetail(text string) {
	op.update(func() { op.detail = text })
}

// update applies f to op and redraws if op's modal is showing.
func (op *operation) update(f func()) {
	r := op.r
	r.mu.Lock()
	f()
	visible := r.cur == op && op.visible
	r.mu.Unlock()
	if visible {
		r.redraw()
	}
}

// Finish ends op. The modal closes at once if it never showed or op was
// replaced; otherwise no sooner than opMinVisible after it appeared.
func (op *operation) Finish() {
	r := op.r
	r.mu.Lock()
	if op.finished {
		r.mu.Unlock()
		return
	}
	op.finished = true
	op.cancel()
	if r.cur != op {
		r.mu.Unlock()
		return
	}
	op.stopAllTimers()
	remaining := opMinVisible - r.now().Sub(op.shownAt)
	if !op.visible || remaining <= 0 {
		r.closeLocked(op)
		r.mu.Unlock()
		r.redraw()
		return
	}
	// Keep animating until it closes.
	r.scheduleFrameLocked(op)
	op.stopTimers = append(op.stopTimers, r.afterFunc(remaining, func() {
		r.mu.Lock()
		r.closeLocked(op)
		r.mu.Unlock()
		r.redraw()
	}))
	r.mu.Unlock()
}
