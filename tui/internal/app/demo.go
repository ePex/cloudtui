package app

import (
	"context"
	"time"

	"github.com/ePex/cloudtui/tui/internal/ui"
)

// The progress demo (":progress-demo", spec-wip fe-progress-modal): a
// counted run of demoSteps steps, one every demoStepInterval, every
// demoFailEvery-th one "failing", then an uncounted run of
// demoSpinnerDuration. No backend involved.
const (
	demoSteps           = 20
	demoStepInterval    = 300 * time.Millisecond
	demoSpinnerDuration = 4 * time.Second
	demoFailEvery       = 7
)

// Status lines the demo ends with.
const (
	demoFinishedStatus  = "Progress demo finished"
	demoCancelledStatus = "Progress demo cancelled"
)

// runProgressDemo starts the progress demo in the background and reports
// how it ended in the status line. forceSpinner previews the fallback.
func (a *App) runProgressDemo(forceSpinner bool) {
	go func() {
		status := a.ops.demo(forceSpinner, demoStepInterval, demoSpinnerDuration)
		a.QueueUpdateDraw(func() { a.SetStatus(status) })
	}()
}

// demo runs the two demo operations, blocking until they end, and returns
// the status line to show. Esc in either part ends the whole demo.
func (r *opRunner) demo(forceSpinner bool, step, spin time.Duration) string {
	title := "Progress demo"
	if forceSpinner {
		title += " (spinner)"
	}
	counted := r.start(title, demoSteps, ui.OpStoppable, forceSpinner)
	failed := 0
	for i := 1; i <= demoSteps; i++ {
		if !waitOrCancel(counted.Ctx(), step) {
			counted.Finish()
			return demoCancelledStatus
		}
		if i%demoFailEvery == 0 {
			failed++
		}
		counted.Step(i, failed)
	}
	// Started before the counted one finishes, so it takes over the
	// visible modal instead of closing it and reopening after a delay.
	uncounted := r.start(title+": no count", 0, ui.OpAbortable, forceSpinner)
	counted.Finish()
	if !waitOrCancel(uncounted.Ctx(), spin) {
		uncounted.Finish()
		return demoCancelledStatus
	}
	uncounted.Finish()
	return demoFinishedStatus
}

// waitOrCancel waits d, returning false early if ctx is cancelled.
func waitOrCancel(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
