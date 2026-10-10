package view

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// TestPurgeUnfilteredCallsPurgeQueue confirms an empty jmsType keeps using
// the existing fast, native PurgeQueue path rather than DeleteMessages
// with an empty filter — see spec/09's "Preserving the existing
// unfiltered path".
func TestPurgeUnfilteredCallsPurgeQueue(t *testing.T) {
	var gotQueue string
	deleteCalled := false
	b := &fakeQueueBackend{
		purgeQueueFn: func(_ context.Context, queueName string) (int, error) {
			gotQueue = queueName
			return 7, nil
		},
		deleteMessagesFn: func(context.Context, string, queue.MessageFilter) (int, error) {
			deleteCalled = true
			return 0, nil
		},
	}

	n, err := purge(context.Background(), b, "orders", "")

	if err != nil {
		t.Fatalf("purge() error = %v", err)
	}
	if n != 7 {
		t.Errorf("purge() = %d, want 7 (PurgeQueue's count)", n)
	}
	if gotQueue != "orders" {
		t.Errorf("queue passed to PurgeQueue = %q, want %q", gotQueue, "orders")
	}
	if deleteCalled {
		t.Error("DeleteMessages was called for an empty JMS type, want PurgeQueue only")
	}
}

// TestPurgeFilteredCallsDeleteMessages confirms a non-empty jmsType routes
// through DeleteMessages with the filter set, not PurgeQueue.
func TestPurgeFilteredCallsDeleteMessages(t *testing.T) {
	var gotQueue string
	var gotFilter queue.MessageFilter
	purgeCalled := false
	b := &fakeQueueBackend{
		purgeQueueFn: func(context.Context, string) (int, error) {
			purgeCalled = true
			return 0, nil
		},
		deleteMessagesFn: func(_ context.Context, queueName string, filter queue.MessageFilter) (int, error) {
			gotQueue, gotFilter = queueName, filter
			return 3, nil
		},
	}

	n, err := purge(context.Background(), b, "orders", "OrderCreated")

	if err != nil {
		t.Fatalf("purge() error = %v", err)
	}
	if n != 3 {
		t.Errorf("purge() = %d, want 3 (DeleteMessages' count)", n)
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

func TestPurgeQuestion(t *testing.T) {
	tests := []struct {
		jmsType, want string
	}{
		{"", `Purge "orders"? All messages will be deleted.`},
		{"TypeB", `Purge "orders"? All TypeB messages will be deleted.`},
	}
	for _, tt := range tests {
		if got := purgeQuestion("orders", tt.jmsType); got != tt.want {
			t.Errorf("purgeQuestion(%q) = %q, want %q", tt.jmsType, got, tt.want)
		}
	}
}

func newTestPurgeFlow(host *fakeViewHost) purgeFlow {
	return purgeFlow{
		host:    host,
		backend: func() queue.Backend { return host.backend },
		prompt:  dialog.NewJMSTypePrompt(host),
		confirm: dialog.NewConfirmDialog(host),
	}
}

func TestPurgeFlowFinishSuccessSetsStatusThenCallsOnDone(t *testing.T) {
	tests := []struct {
		jmsType, want string
	}{
		{"", `Purged 1600 message(s) from "orders"`},
		{"TypeB", `Purged 800 TypeB message(s) from "orders"`},
	}
	for _, tt := range tests {
		host := newFakeViewHost()
		n := 1600
		if tt.jmsType != "" {
			n = 800
		}
		var statusAtDone string
		newTestPurgeFlow(host).finish("orders", tt.jmsType, n, "", nil,
			func() { statusAtDone = host.status },
			func(err error) { t.Errorf("onError(%v) called on success", err) })

		if statusAtDone != tt.want {
			t.Errorf("status when onDone ran = %q, want %q", statusAtDone, tt.want)
		}
	}
}

func TestPurgeFlowFinishAppendsTheWarning(t *testing.T) {
	host := newFakeViewHost()
	warning := " — 1600 message(s) remain; matching messages further back may not have been reached"

	newTestPurgeFlow(host).finish("orders", "TypeB", 0, warning, nil, func() {}, func(error) {})

	if want := `Purged 0 TypeB message(s) from "orders"` + warning; host.status != want {
		t.Errorf("status = %q, want %q", host.status, want)
	}
}

func TestFilteredRemainderWarning(t *testing.T) {
	const warn1600 = " — 1600 message(s) remain; matching messages further back may not have been reached"
	tests := []struct {
		name    string
		jmsType string
		pending int64
		listErr error
		want    string
	}{
		{"filtered, above the window", "TypeB", 1600, nil, warn1600},
		{"filtered, just above the window", "TypeB", 201, nil, " — 201 message(s) remain; matching messages further back may not have been reached"},
		{"filtered, at the window", "TypeB", 200, nil, ""},
		{"filtered, below the window", "TypeB", 10, nil, ""},
		{"unfiltered never warns", "", 1600, nil, ""},
		{"List fails", "TypeB", 1600, errors.New("boom"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listCalled := false
			b := &fakeQueueBackend{listFn: func(context.Context) ([]queue.Summary, error) {
				listCalled = true
				if tt.listErr != nil {
					return nil, tt.listErr
				}
				return []queue.Summary{{Name: "other", PendingCount: 9999}, {Name: "orders", PendingCount: tt.pending}}, nil
			}}

			if got := filteredRemainderWarning(context.Background(), b, "orders", tt.jmsType); got != tt.want {
				t.Errorf("filteredRemainderWarning() = %q, want %q", got, tt.want)
			}
			if tt.jmsType == "" && listCalled {
				t.Error("List was called for an unfiltered operation")
			}
		})
	}
}

func TestPurgeFlowFinishErrorCallsOnErrorOnly(t *testing.T) {
	host := newFakeViewHost()
	wantErr := errors.New("boom")
	var gotErr error

	newTestPurgeFlow(host).finish("orders", "", 400, "", wantErr,
		func() { t.Error("onDone called on error") },
		func(err error) { gotErr = err })

	if gotErr != wantErr {
		t.Errorf("onError got %v, want %v", gotErr, wantErr)
	}
	if host.status != "" {
		t.Errorf("status = %q, want it untouched on error", host.status)
	}
}

func TestPurgeTitle(t *testing.T) {
	if got, want := purgeTitle("orders", ""), `Purging "orders"`; got != want {
		t.Errorf("purgeTitle() = %q, want %q", got, want)
	}
	if got, want := purgeTitle("orders", "TypeB"), `Purging TypeB messages from "orders"`; got != want {
		t.Errorf("purgeTitle(TypeB) = %q, want %q", got, want)
	}
}

func TestPurgeFlowRunsAStoppableOperation(t *testing.T) {
	base := newFakeViewHost()
	host := &drawSignalingHost{fakeViewHost: base, drawn: make(chan struct{}, 2)}
	// The JMS Type prompt starts a scan whose result would also draw;
	// keep it from finishing, so the only draw is the purge's.
	base.scanJMSTypesFn = func(context.Context, string, int) (ui.JMSTypeScan, error) { select {} }
	var gotCtx context.Context
	base.backend = &fakeQueueBackend{purgeQueueFn: func(ctx context.Context, _ string) (int, error) {
		gotCtx = ctx
		return 5, nil
	}}
	pf := purgeFlow{
		host:    host,
		backend: func() queue.Backend { return base.backend },
		prompt:  dialog.NewJMSTypePrompt(host),
		confirm: dialog.NewConfirmDialog(host),
	}
	pf.start("orders", func() {}, func() {}, func(err error) { t.Errorf("onError(%v)", err) })
	pf.prompt.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	confirmYes(pf.confirm)
	<-host.drawn

	op := host.lastOperation()
	if op == nil {
		t.Fatal("purge started no operation")
	}
	if op.title != `Purging "orders"` || op.total != 0 || op.kind != ui.OpStoppable {
		t.Errorf("operation = %q, total %d, kind %v; want the purge, uncounted, stoppable", op.title, op.total, op.kind)
	}
	if gotCtx != op.Ctx() {
		t.Error("purge didn't run with the operation's context, so Esc couldn't reach it")
	}
	if !op.isFinished() {
		t.Error("purge operation not finished")
	}
	if want := `Purged 5 message(s) from "orders"`; host.status != want {
		t.Errorf("status = %q, want %q", host.status, want)
	}
}

func TestPurgeFlowFinishCancelledReportsAndReloads(t *testing.T) {
	host := newFakeViewHost()
	done := false
	err := fmt.Errorf("purge queue %q: %w", "orders", context.Canceled)
	newTestPurgeFlow(host).finish("orders", "", 400, "", err,
		func() { done = true },
		func(err error) { t.Errorf("onError(%v) called for a stop", err) })

	if want := `Purged 400 message(s) from "orders" (cancelled)`; host.status != want {
		t.Errorf("status = %q, want %q", host.status, want)
	}
	if !done {
		t.Error("onDone not called after a stop, want the view reloaded")
	}
}

func TestUnlessQueueEmptied(t *testing.T) {
	stopped := fmt.Errorf("purge queue: %w", context.Canceled)
	broken := errors.New("broken")
	tests := []struct {
		name    string
		jmsType string
		pending []queue.Summary
		err     error
		want    error
	}{
		{"no error", "", nil, nil, nil},
		{"other error", "", []queue.Summary{{Name: "orders"}}, broken, broken},
		{"stopped, queue emptied", "", []queue.Summary{{Name: "orders", PendingCount: 0}}, stopped, nil},
		{"stopped, messages left", "", []queue.Summary{{Name: "orders", PendingCount: 5}}, stopped, stopped},
		{"stopped, filtered", "TypeB", []queue.Summary{{Name: "orders", PendingCount: 0}}, stopped, stopped},
		{"stopped, count unreadable", "", nil, stopped, stopped},
	}
	for _, tt := range tests {
		b := &fakeQueueBackend{summaries: tt.pending}
		if got := unlessQueueEmptied(b, "orders", tt.jmsType, tt.err); got != tt.want {
			t.Errorf("%s: unlessQueueEmptied() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestPurgeFlowStoppedAfterEmptyingReportsComplete: a stop that only
// skipped the check for more (the queue is empty) isn't reported as
// cancelled.
func TestPurgeFlowStoppedAfterEmptyingReportsComplete(t *testing.T) {
	base := newFakeViewHost()
	host := &drawSignalingHost{fakeViewHost: base, drawn: make(chan struct{}, 2)}
	base.scanJMSTypesFn = func(context.Context, string, int) (ui.JMSTypeScan, error) { select {} }
	base.backend = &fakeQueueBackend{
		summaries: []queue.Summary{{Name: "orders", PendingCount: 0}},
		purgeQueueFn: func(context.Context, string) (int, error) {
			return 1600, fmt.Errorf("purge queue: %w", context.Canceled)
		},
	}
	pf := purgeFlow{
		host:    host,
		backend: func() queue.Backend { return base.backend },
		prompt:  dialog.NewJMSTypePrompt(host),
		confirm: dialog.NewConfirmDialog(host),
	}
	pf.start("orders", func() {}, func() {}, func(err error) { t.Errorf("onError(%v)", err) })
	pf.prompt.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	confirmYes(pf.confirm)
	<-host.drawn

	if want := `Purged 1600 message(s) from "orders"`; host.status != want {
		t.Errorf("status = %q, want %q (nothing was left to cancel)", host.status, want)
	}
}
