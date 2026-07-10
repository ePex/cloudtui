package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// runApp starts a's event loop against a headless simulation screen so
// goroutine + QueueUpdateDraw code (used by the queues view's backend
// calls) actually executes during tests — without a running event loop,
// QueueUpdateDraw blocks forever waiting for something to drain it.
func runApp(t *testing.T, a *App) {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	a.tv.SetScreen(screen)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := a.tv.Run(); err != nil {
			t.Errorf("a.tv.Run() error = %v", err)
		}
	}()
	t.Cleanup(func() {
		a.tv.Stop()
		<-done
	})
}

// waitFor polls cond (safely, via a.tv's own update queue, to avoid racing
// the running event loop) until it's true or the timeout elapses.
func waitFor(t *testing.T, a *App, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var met bool
		a.tv.QueueUpdate(func() { met = cond() })
		if met {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

// fakeBackend is a queue.Backend used to exercise the queues view without
// a real mq-proxy. Guarded by a mutex since its methods run on a
// goroutine while the test's main goroutine may read the recorded calls.
type fakeBackend struct {
	mu sync.Mutex

	summaries    []queue.Summary
	summariesErr error

	messages    map[string][]queue.Message
	messagesErr error

	sendErr error
	sent    struct {
		queueName, body string
	}

	purgeErr error
	purged   string

	moveErr error
	moved   struct {
		source, target string
		max            *int
	}
}

var _ queue.Backend = (*fakeBackend)(nil)

func (f *fakeBackend) List(ctx context.Context) ([]queue.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.summaries, f.summariesErr
}

func (f *fakeBackend) Browse(ctx context.Context, queueName string, limit int) ([]queue.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.messagesErr != nil {
		return nil, f.messagesErr
	}
	return f.messages[queueName], nil
}

func (f *fakeBackend) Send(ctx context.Context, queueName, body string, properties map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent.queueName, f.sent.body = queueName, body
	return f.sendErr
}

func (f *fakeBackend) Purge(ctx context.Context, queueName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.purged = queueName
	return f.purgeErr
}

func (f *fakeBackend) Move(ctx context.Context, sourceQueueName, targetQueueName string, maxMessages *int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.moved.source, f.moved.target, f.moved.max = sourceQueueName, targetQueueName, maxMessages
	return f.moveErr
}

func newTestAppWithBackend(t *testing.T, backend *fakeBackend) *App {
	t.Helper()
	a := New()
	a.backend = backend
	runApp(t, a)
	return a
}

func TestLoadQueuesPopulatesListWithoutBlocking(t *testing.T) {
	fb := &fakeBackend{summaries: []queue.Summary{
		{Name: "foo", PendingCount: 3, ConsumerCount: 1},
	}}
	a := newTestAppWithBackend(t, fb)

	a.loadQueues()
	waitFor(t, a, func() bool { return a.queuesList.GetItemCount() == 1 })

	main, secondary := a.queuesList.GetItemText(0)
	if main != "foo" {
		t.Errorf("item main text = %q, want %q", main, "foo")
	}
	if secondary != "pending: 3  consumers: 1" {
		t.Errorf("item secondary text = %q, want %q", secondary, "pending: 3  consumers: 1")
	}
}

func TestLoadQueuesShowsErrorInStatusBar(t *testing.T) {
	fb := &fakeBackend{summariesErr: errors.New("broker down")}
	a := newTestAppWithBackend(t, fb)

	a.loadQueues()
	waitFor(t, a, func() bool { return strings.Contains(a.statusBar.GetText(true), "broker down") })
}

func TestShowQueueDetailLoadsMessages(t *testing.T) {
	fb := &fakeBackend{messages: map[string][]queue.Message{
		"foo": {{ID: "1", Body: "hello"}, {ID: "2", Body: "world"}},
	}}
	a := newTestAppWithBackend(t, fb)

	a.showQueueDetail("foo")
	waitFor(t, a, func() bool { return a.messagesList.GetItemCount() == 2 })

	if got, want := a.currentQueueName, "foo"; got != want {
		t.Errorf("currentQueueName = %q, want %q", got, want)
	}
	if name, _ := a.queuesRoot.GetFrontPage(); name != "detail" {
		t.Errorf("queuesRoot front page = %q, want %q", name, "detail")
	}
	main, _ := a.messagesList.GetItemText(0)
	if main != "hello" {
		t.Errorf("item main text = %q, want %q", main, "hello")
	}
}

func TestEscapeReturnsFromDetailToList(t *testing.T) {
	fb := &fakeBackend{messages: map[string][]queue.Message{"foo": {{ID: "1", Body: "hi"}}}}
	a := newTestAppWithBackend(t, fb)

	a.showQueueDetail("foo")
	waitFor(t, a, func() bool { return a.messagesList.GetItemCount() == 1 })

	escape := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	if got := a.onMessagesKey(escape); got != nil {
		t.Errorf("onMessagesKey(Escape) = %v, want nil", got)
	}
	if name, _ := a.queuesRoot.GetFrontPage(); name != "list" {
		t.Errorf("queuesRoot front page = %q, want %q", name, "list")
	}
}

func TestSendMessageReloadsDetail(t *testing.T) {
	fb := &fakeBackend{messages: map[string][]queue.Message{}}
	a := newTestAppWithBackend(t, fb)
	a.currentQueueName = "foo"

	a.sendMessage("foo", "hello there")
	waitFor(t, a, func() bool {
		fb.mu.Lock()
		defer fb.mu.Unlock()
		return fb.sent.body == "hello there"
	})

	fb.mu.Lock()
	gotQueue := fb.sent.queueName
	fb.mu.Unlock()
	if gotQueue != "foo" {
		t.Errorf("sent.queueName = %q, want %q", gotQueue, "foo")
	}
	waitFor(t, a, func() bool { return a.statusBar.GetText(true) == statusReadyText })
}

func TestOpenSendModalShowsFormAndAKeyOpensIt(t *testing.T) {
	fb := &fakeBackend{}
	a := newTestAppWithBackend(t, fb)
	a.currentQueueName = "foo"

	aKey := tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)
	if got := a.onMessagesKey(aKey); got != nil {
		t.Errorf("onMessagesKey('a') = %v, want nil", got)
	}
	if !a.rootPages.HasPage("action") {
		t.Error("rootPages has no \"action\" page after 'a'")
	}
}

func TestPurgeQueueReloadsListAndDetail(t *testing.T) {
	fb := &fakeBackend{
		summaries: []queue.Summary{{Name: "foo", PendingCount: 0, ConsumerCount: 0}},
		messages:  map[string][]queue.Message{"foo": {}},
	}
	a := newTestAppWithBackend(t, fb)
	a.currentQueueName = "foo"

	a.purgeQueue("foo")
	waitFor(t, a, func() bool {
		fb.mu.Lock()
		defer fb.mu.Unlock()
		return fb.purged == "foo"
	})
	waitFor(t, a, func() bool { return a.statusBar.GetText(true) == statusReadyText })
}

func TestOpenPurgeConfirmShowsModalAndDKeyOpensIt(t *testing.T) {
	fb := &fakeBackend{}
	a := newTestAppWithBackend(t, fb)
	a.currentQueueName = "foo"

	dKey := tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone)
	if got := a.onMessagesKey(dKey); got != nil {
		t.Errorf("onMessagesKey('d') = %v, want nil", got)
	}
	if !a.rootPages.HasPage("action") {
		t.Error("rootPages has no \"action\" page after 'd'")
	}
}

func TestMoveMessagesReloadsListAndDetail(t *testing.T) {
	fb := &fakeBackend{
		summaries: []queue.Summary{{Name: "source"}},
		messages:  map[string][]queue.Message{"source": {}},
	}
	a := newTestAppWithBackend(t, fb)
	a.currentQueueName = "source"

	a.moveMessages("source", "target")
	waitFor(t, a, func() bool {
		fb.mu.Lock()
		defer fb.mu.Unlock()
		return fb.moved.target == "target"
	})

	fb.mu.Lock()
	gotSource := fb.moved.source
	fb.mu.Unlock()
	if gotSource != "source" {
		t.Errorf("moved.source = %q, want %q", gotSource, "source")
	}
	waitFor(t, a, func() bool { return a.statusBar.GetText(true) == statusReadyText })
}

func TestOpenMoveModalShowsFormAndVKeyOpensIt(t *testing.T) {
	fb := &fakeBackend{}
	a := newTestAppWithBackend(t, fb)
	a.currentQueueName = "foo"

	vKey := tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModNone)
	if got := a.onMessagesKey(vKey); got != nil {
		t.Errorf("onMessagesKey('v') = %v, want nil", got)
	}
	if !a.rootPages.HasPage("action") {
		t.Error("rootPages has no \"action\" page after 'v'")
	}
}

func TestActivateReloadsQueueList(t *testing.T) {
	fb := &fakeBackend{summaries: []queue.Summary{{Name: "foo"}}}
	a := newTestAppWithBackend(t, fb)

	qv := &queuesView{root: a.queuesRoot, app: a}
	qv.activate()

	waitFor(t, a, func() bool { return a.queuesList.GetItemCount() == 1 })
}
