package view

import (
	"context"
	"errors"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
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

func TestPurgeFlowExecuteShowsAndFinishesIndeterminateProgress(t *testing.T) {
	host := newFakeViewHost()
	release := make(chan struct{})
	done := make(chan struct{})
	purgeFlow{
		host: host,
		backend: func() queue.Backend {
			return &fakeQueueBackend{purgeQueueFn: func(context.Context, string) (int, error) {
				<-release
				return 2, nil
			}}
		},
	}.execute("orders", "", func() {
		if host.progressVisible {
			t.Error("purge completion callback ran before progress was hidden")
		}
		close(done)
	}, func(err error) { t.Errorf("onError(%v) called on success", err) })

	if !host.progressVisible || host.progressLabel != "Purging orders" || host.progressTotal != 0 {
		t.Fatalf("progress at start = visible %v, label %q, total %d", host.progressVisible, host.progressLabel, host.progressTotal)
	}
	close(release)
	<-done
	if host.progressVisible {
		t.Error("purge left progress visible after success")
	}
	if got := host.status; got != `Purged 2 message(s) from "orders"` {
		t.Errorf("status after purge = %q", got)
	}
}

func TestPurgeFlowExecuteHidesProgressOnError(t *testing.T) {
	host := newFakeViewHost()
	wantErr := errors.New("broker unavailable")
	release := make(chan struct{})
	done := make(chan error, 1)
	purgeFlow{
		host: host,
		backend: func() queue.Backend {
			return &fakeQueueBackend{purgeQueueFn: func(context.Context, string) (int, error) {
				<-release
				return 0, wantErr
			}}
		},
	}.execute("orders", "", func() { t.Error("onDone called on failure") }, func(err error) {
		done <- err
	})

	if !host.progressVisible {
		t.Fatal("purge did not show progress before the backend returned")
	}
	close(release)
	if got := <-done; got != wantErr {
		t.Errorf("onError got %v, want %v", got, wantErr)
	}
	if host.progressVisible {
		t.Error("purge left progress visible after error")
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
