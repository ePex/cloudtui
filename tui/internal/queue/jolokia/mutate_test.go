package jolokia

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

func TestMoveAllMessages(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value":  float64(42),
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	count, err := c.MoveAllMessages(context.Background(), "srcQueue", "dstQueue")
	if err != nil {
		t.Fatalf("MoveAllMessages() error = %v", err)
	}
	if count != 42 {
		t.Errorf("MoveAllMessages() count = %d, want 42", count)
	}
	if got := capturedBody["operation"]; got != "moveMatchingMessagesTo(java.lang.String,java.lang.String)" {
		t.Errorf("operation = %q, want moveMatchingMessagesTo", got)
	}
	args, _ := capturedBody["arguments"].([]any)
	if len(args) < 2 || args[0] != "TRUE" || args[1] != "dstQueue" {
		t.Errorf("arguments = %v, want [TRUE dstQueue]", args)
	}
}

func TestMoveAllMessagesJolokiaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 500,
			"error":  "operation failed",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.MoveAllMessages(context.Background(), "srcQueue", "dstQueue")
	if err == nil {
		t.Fatal("MoveAllMessages() expected error for Jolokia status 500, got nil")
	}
}

func TestSendMessage(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value":  nil,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	req := queue.SendMessageRequest{
		JMSType:       "order.created",
		Body:          "hello world",
		CorrelationID: "corr-1",
		GroupID:       "group-1",
		Headers:       map[string]string{"custom": "value"},
	}
	if err := c.SendMessage(context.Background(), "myQueue", req); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	wantOp := "sendTextMessage(java.util.Map,java.lang.String,java.lang.String,java.lang.String)"
	if got := capturedBody["operation"]; got != wantOp {
		t.Errorf("operation = %q, want %q", got, wantOp)
	}
	args, _ := capturedBody["arguments"].([]any)
	// args: [headers, body, username, password]
	if len(args) < 4 {
		t.Fatalf("arguments len = %d, want 4", len(args))
	}
	headers, ok := args[0].(map[string]any)
	if !ok {
		t.Fatalf("arguments[0] (headers) type = %T, want map[string]any", args[0])
	}
	if headers["JMSType"] != "order.created" {
		t.Errorf("headers[JMSType] = %v, want %q", headers["JMSType"], "order.created")
	}
	if headers["JMSCorrelationID"] != "corr-1" {
		t.Errorf("headers[JMSCorrelationID] = %v, want %q", headers["JMSCorrelationID"], "corr-1")
	}
	if headers["JMSXGroupID"] != "group-1" {
		t.Errorf("headers[JMSXGroupID] = %v, want %q", headers["JMSXGroupID"], "group-1")
	}
	if headers["custom"] != "value" {
		t.Errorf("headers[custom] = %v, want %q", headers["custom"], "value")
	}
	if args[1] != "hello world" {
		t.Errorf("arguments[1] (body) = %v, want \"hello world\"", args[1])
	}
	if args[2] != "admin" {
		t.Errorf("arguments[2] (username) = %v, want \"admin\"", args[2])
	}
}

// TestSendMessageWithoutOptionalFieldsSetsOnlyJMSType covers the "nothing
// but JMSType/Body set" case: no JMSCorrelationID/JMSXGroupID key should
// appear in the headers map at all.
func TestSendMessageWithoutOptionalFieldsSetsOnlyJMSType(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(map[string]any{"status": 200, "value": nil})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	req := queue.SendMessageRequest{JMSType: "text", Body: "hello world"}
	if err := c.SendMessage(context.Background(), "myQueue", req); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	args, _ := capturedBody["arguments"].([]any)
	headers, _ := args[0].(map[string]any)
	if len(headers) != 1 {
		t.Fatalf("headers = %v, want exactly {JMSType: text}", headers)
	}
	if headers["JMSType"] != "text" {
		t.Errorf("headers[JMSType] = %v, want %q", headers["JMSType"], "text")
	}
}

// TestSendMessageCustomHeaderCannotOverrideReservedKey covers the
// precedence decision in spec-wip/fe-send-message-metadata/plan.md: a
// custom Headers entry named after a reserved key is dropped, never
// applied — the dedicated field always wins.
func TestSendMessageCustomHeaderCannotOverrideReservedKey(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(map[string]any{"status": 200, "value": nil})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	req := queue.SendMessageRequest{
		JMSType: "order.created",
		Body:    "hello world",
		GroupID: "real-group",
		Headers: map[string]string{
			"JMSXGroupID": "spoofed-group",
			"JMSType":     "spoofed-type",
		},
	}
	if err := c.SendMessage(context.Background(), "myQueue", req); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	args, _ := capturedBody["arguments"].([]any)
	headers, _ := args[0].(map[string]any)
	if headers["JMSXGroupID"] != "real-group" {
		t.Errorf("headers[JMSXGroupID] = %v, want the dedicated GroupID field's %q, not the spoofed header", headers["JMSXGroupID"], "real-group")
	}
	if headers["JMSType"] != "order.created" {
		t.Errorf("headers[JMSType] = %v, want the dedicated JMSType field's %q, not the spoofed header", headers["JMSType"], "order.created")
	}
}

func TestSendMessageJolokiaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 500,
			"error":  "operation failed",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.SendMessage(context.Background(), "myQueue", queue.SendMessageRequest{JMSType: "text", Body: "hello"})
	if err == nil {
		t.Fatal("SendMessage() expected error for Jolokia status 500, got nil")
	}
}

// purgeServer fakes the Jolokia calls PurgeQueue makes. It answers each
// request by its operation ("read:<attribute>" for reads) from responses,
// with a Jolokia status 500 for anything not listed, and records the
// operations in the order they arrived.
func purgeServer(t *testing.T, responses map[string]any) (*httptest.Server, *[]string) {
	t.Helper()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Type      string `json:"type"`
			Operation string `json:"operation"`
			Attribute string `json:"attribute"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		key := req.Operation
		if req.Type == "read" {
			key = "read:" + req.Attribute
		}
		calls = append(calls, key)
		resp, ok := responses[key]
		if !ok {
			resp = map[string]any{"status": 500, "error": "No operation " + key + " found"}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func ok(value any) map[string]any { return map[string]any{"status": 200, "value": value} }

func TestPurgeQueueDirectOperationReportsQueueSize(t *testing.T) {
	srv, calls := purgeServer(t, map[string]any{
		"read:QueueSize": ok(float64(7)),
		"purgeQueue()":   ok(nil),
	})

	n, err := newTestClient(srv.URL).PurgeQueue(context.Background(), "myQueue")
	if err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
	if n != 7 {
		t.Errorf("PurgeQueue() = %d, want 7 (QueueSize read before purging)", n)
	}
	if want := []string{"read:QueueSize", "purgeQueue()"}; !slices.Equal(*calls, want) {
		t.Errorf("calls = %v, want %v", *calls, want)
	}
}

func TestPurgeQueueRemoveMatchingFallback(t *testing.T) {
	srv, calls := purgeServer(t, map[string]any{
		"read:QueueSize": ok(float64(7)),
		"removeMatchingMessages(java.lang.String)": ok(float64(3)),
	})

	n, err := newTestClient(srv.URL).PurgeQueue(context.Background(), "myQueue")
	if err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
	if n != 3 {
		t.Errorf("PurgeQueue() = %d, want 3 (removeMatchingMessages' own count)", n)
	}
	want := []string{"read:QueueSize", "purgeQueue()", "removeMatchingMessages(java.lang.String)"}
	if !slices.Equal(*calls, want) {
		t.Errorf("calls = %v, want %v", *calls, want)
	}
}

func TestPurgeQueueSkipsDirectOperationWhenQueueSizeUnreadable(t *testing.T) {
	srv, calls := purgeServer(t, map[string]any{
		"purgeQueue()": ok(nil),
		"removeMatchingMessages(java.lang.String)": ok(float64(3)),
	})

	n, err := newTestClient(srv.URL).PurgeQueue(context.Background(), "myQueue")
	if err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
	if n != 3 {
		t.Errorf("PurgeQueue() = %d, want 3", n)
	}
	if slices.Contains(*calls, "purgeQueue()") {
		t.Errorf("calls = %v, want no purgeQueue() without a QueueSize to report", *calls)
	}
}

func TestPurgeQueueBrowseAndRemoveCountsRemovals(t *testing.T) {
	srv, _ := purgeServer(t, map[string]any{
		"browseMessages()": ok([]map[string]any{
			{"messageId": "ID:1", "timestamp": int64(1721000000000), "text": "a"},
			{"messageId": "ID:2", "timestamp": int64(1721000000000), "text": "b"},
		}),
		"removeMessage(java.lang.String)": ok(true),
	})

	n, err := newTestClient(srv.URL).PurgeQueue(context.Background(), "myQueue")
	if err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
	if n != 2 {
		t.Errorf("PurgeQueue() = %d, want 2", n)
	}
}

func TestPurgeQueueBrowseAndRemoveReportsPartialCountOnError(t *testing.T) {
	removes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Operation string `json:"operation"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		switch req.Operation {
		case "browseMessages()":
			json.NewEncoder(w).Encode(ok([]map[string]any{
				{"messageId": "ID:1", "timestamp": int64(1721000000000), "text": "a"},
				{"messageId": "ID:2", "timestamp": int64(1721000000000), "text": "b"},
				{"messageId": "ID:3", "timestamp": int64(1721000000000), "text": "c"},
			}))
		case "removeMessage(java.lang.String)":
			removes++
			if removes == 3 {
				json.NewEncoder(w).Encode(map[string]any{"status": 500, "error": "boom"})
				return
			}
			json.NewEncoder(w).Encode(ok(true))
		default:
			json.NewEncoder(w).Encode(map[string]any{"status": 500, "error": "unsupported"})
		}
	}))
	defer srv.Close()

	n, err := newTestClient(srv.URL).PurgeQueue(context.Background(), "myQueue")
	if err == nil {
		t.Fatal("PurgeQueue() error = nil, want the failed removal")
	}
	if n != 2 {
		t.Errorf("PurgeQueue() = %d, want 2 (removed before the failure)", n)
	}
}

// heldPurgeServer is purgeServer with one operation held: the request for
// hold signals arrived, then waits for release before it's answered.
func heldPurgeServer(t *testing.T, responses map[string]any, hold string) (srv *httptest.Server, calls func() []string, arrived, release chan struct{}) {
	t.Helper()
	arrived, release = make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	var got []string
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Type      string `json:"type"`
			Operation string `json:"operation"`
			Attribute string `json:"attribute"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		key := req.Operation
		if req.Type == "read" {
			key = "read:" + req.Attribute
		}
		mu.Lock()
		got = append(got, key)
		mu.Unlock()
		if key == hold {
			close(arrived)
			<-release
		}
		resp, ok := responses[key]
		if !ok {
			resp = map[string]any{"status": 500, "error": "No operation " + key + " found"}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	calls = func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
	return srv, calls, arrived, release
}

// purgeResult is what PurgeQueue returned, sent from a goroutine.
type purgeResult struct {
	n   int
	err error
}

// TestPurgeQueueStopDuringTheSizeReadPurgesNothing: a stop while the
// QueueSize read is in flight lets the read finish, then purges nothing
// (spec-wip fe-progress-modal).
func TestPurgeQueueStopDuringTheSizeReadPurgesNothing(t *testing.T) {
	srv, calls, arrived, release := heldPurgeServer(t, map[string]any{
		"read:QueueSize": ok(float64(7)),
		"purgeQueue()":   ok(nil),
		"removeMatchingMessages(java.lang.String)": ok(float64(7)),
	}, "read:QueueSize")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan purgeResult)
	go func() {
		n, err := newTestClient(srv.URL).PurgeQueue(ctx, "myQueue")
		done <- purgeResult{n, err}
	}()
	<-arrived
	cancel()
	close(release)
	got := <-done

	if !errors.Is(got.err, context.Canceled) || got.n != 0 {
		t.Fatalf("PurgeQueue() = %d, %v; want 0, context.Canceled", got.n, got.err)
	}
	for _, op := range calls() {
		if op != "read:QueueSize" {
			t.Errorf("calls = %v, want nothing after the read", calls())
			break
		}
	}
}

// TestPurgeQueueDirectOperationFinishesDespiteAStop: a purgeQueue() that's
// been sent finishes and reports its count.
func TestPurgeQueueDirectOperationFinishesDespiteAStop(t *testing.T) {
	srv, _, arrived, release := heldPurgeServer(t, map[string]any{
		"read:QueueSize": ok(float64(7)),
		"purgeQueue()":   ok(nil),
	}, "purgeQueue()")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan purgeResult)
	go func() {
		n, err := newTestClient(srv.URL).PurgeQueue(ctx, "myQueue")
		done <- purgeResult{n, err}
	}()
	<-arrived
	cancel()
	close(release)
	if got := <-done; got.err != nil || got.n != 7 {
		t.Errorf("PurgeQueue() = %d, %v; want 7, nil (the sent purge finishes)", got.n, got.err)
	}
}

// TestPurgeQueueBrowseAndRemoveStopsBetweenRemovals: the last-resort
// browse-and-remove stops before the next message once stopped, and
// reports how many it removed.
func TestPurgeQueueBrowseAndRemoveStopsBetweenRemovals(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	removes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Operation string `json:"operation"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		switch req.Operation {
		case "browseMessages()":
			json.NewEncoder(w).Encode(ok([]map[string]any{
				{"messageId": "ID:1", "timestamp": int64(1721000000000), "text": "a"},
				{"messageId": "ID:2", "timestamp": int64(1721000000000), "text": "b"},
				{"messageId": "ID:3", "timestamp": int64(1721000000000), "text": "c"},
			}))
		case "removeMessage(java.lang.String)":
			removes++
			cancel() // the stop arrives while the first removal is in flight
			json.NewEncoder(w).Encode(ok(true))
		default:
			json.NewEncoder(w).Encode(map[string]any{"status": 500, "error": "no"})
		}
	}))
	defer srv.Close()

	n, err := newTestClient(srv.URL).PurgeQueue(ctx, "myQueue")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PurgeQueue() error = %v, want context.Canceled", err)
	}
	if n != 1 || removes != 1 {
		t.Errorf("PurgeQueue() = %d after %d removal(s), want 1 after 1", n, removes)
	}
}

// TestPurgeQueueStopDuringAFailingTierTriesNoOther: a stop while
// purgeQueue() is in flight, which then fails, doesn't go on to the next
// purge route.
func TestPurgeQueueStopDuringAFailingTierTriesNoOther(t *testing.T) {
	srv, calls, arrived, release := heldPurgeServer(t, map[string]any{
		"read:QueueSize": ok(float64(7)),
		"removeMatchingMessages(java.lang.String)": ok(float64(7)),
	}, "purgeQueue()") // not in responses: it fails
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan purgeResult)
	go func() {
		n, err := newTestClient(srv.URL).PurgeQueue(ctx, "myQueue")
		done <- purgeResult{n, err}
	}()
	<-arrived
	cancel()
	close(release)
	got := <-done

	if !errors.Is(got.err, context.Canceled) || got.n != 0 {
		t.Fatalf("PurgeQueue() = %d, %v; want 0, context.Canceled", got.n, got.err)
	}
	if slices.Contains(calls(), "removeMatchingMessages(java.lang.String)") {
		t.Errorf("calls = %v, want no fallback after the stop", calls())
	}
}
