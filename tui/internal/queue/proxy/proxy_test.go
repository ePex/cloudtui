package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

// newTestClient starts an httptest server with handler h and returns a Client
// pointed at it plus a cleanup function.
func newTestClient(t *testing.T, h http.Handler) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	c := NewClient(config.ProxyConfig{
		URL:      srv.URL,
		Username: "user",
		Password: "pass",
	})
	return c, srv.Close
}

// checkBasicAuth fails the test if the request does not carry valid Basic auth.
func checkBasicAuth(t *testing.T, r *http.Request) {
	t.Helper()
	u, p, ok := r.BasicAuth()
	if !ok || u != "user" || p != "pass" {
		t.Errorf("missing or wrong Basic auth: got user=%q pass=%q ok=%v", u, p, ok)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestList(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.URL.Path != "/api/management/command/list-queues" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"name": "orders", "messageCount": 5, "consumerCount": 1, "enqueuedCount": 10, "dequeuedCount": 5, "producerCount": 2},
			},
			"errors": []any{},
		})
	}))
	defer stop()

	summaries, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("List() returned %d summaries, want 1", len(summaries))
	}
	s := summaries[0]
	if s.Name != "orders" {
		t.Errorf("Name = %q, want %q", s.Name, "orders")
	}
	if s.PendingCount != 5 {
		t.Errorf("PendingCount = %d, want 5", s.PendingCount)
	}
	if s.ConsumerCount != 1 {
		t.Errorf("ConsumerCount = %d, want 1", s.ConsumerCount)
	}
	if s.EnqueueCount != 10 {
		t.Errorf("EnqueueCount = %d, want 10", s.EnqueueCount)
	}
	if s.DequeueCount != 5 {
		t.Errorf("DequeueCount = %d, want 5", s.DequeueCount)
	}
	if s.ProducerCount != 2 {
		t.Errorf("ProducerCount = %d, want 2", s.ProducerCount)
	}
}

func TestListError(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "broker down"})
	}))
	defer stop()

	_, err := c.List(context.Background())
	if err == nil {
		t.Fatal("List() error = nil, want non-nil")
	}
}

func TestListEnvelopeError(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []any{},
			"errors": []map[string]string{{"code": "BROKER_ERROR", "message": "connection lost"}},
		})
	}))
	defer stop()

	_, err := c.List(context.Background())
	if err == nil {
		t.Fatal("List() error = nil, want non-nil for a populated errors array")
	}
}

// ── BrowseMessages ────────────────────────────────────────────────────────────

func TestBrowseMessages(t *testing.T) {
	body := "hello world"
	ts := "2024-01-01T00:00:00Z"
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if !strings.HasPrefix(r.URL.Path, "/api/management/command/list-messages") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("sourceQueue") != "orders" {
			t.Errorf("sourceQueue = %q, want %q", r.URL.Query().Get("sourceQueue"), "orders")
		}
		if r.URL.Query().Get("returnBody") != "true" {
			t.Errorf("returnBody = %q, want %q", r.URL.Query().Get("returnBody"), "true")
		}
		for _, key := range []string{"filter.jmsType", "filter.messageId", "filter.fromDate", "filter.toDate", "filter.maxCount"} {
			if r.URL.Query().Has(key) {
				t.Errorf("unexpected query param %q for a zero-value filter", key)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"sourceQueue": "orders", "messageId": "ID:m1", "jmsType": "order-created", "timestamp": ts, "body": body, "headers": map[string]string{"foo": "bar"}},
			},
			"errors": []any{},
		})
	}))
	defer stop()

	msgs, _, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("BrowseMessages() returned %d messages, want 1", len(msgs))
	}
	m := msgs[0]
	if m.ID != "ID:m1" {
		t.Errorf("ID = %q, want %q", m.ID, "ID:m1")
	}
	if m.JMSType != "order-created" {
		t.Errorf("JMSType = %q, want %q", m.JMSType, "order-created")
	}
	if m.JMSTypeInferred {
		t.Error("JMSTypeInferred = true, want false (mq-proxy reported a real jmsType)")
	}
	if m.Preview != "hello world" {
		t.Errorf("Preview = %q, want %q", m.Preview, "hello world")
	}
	wantTS, _ := time.Parse(time.RFC3339, ts)
	if !m.Timestamp.Equal(wantTS) {
		t.Errorf("Timestamp = %v, want %v", m.Timestamp, wantTS)
	}
	if got, _ := m.RawFields["text"].(string); got != body {
		t.Errorf("RawFields[text] = %q, want %q", got, body)
	}
	props, ok := m.RawFields["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("RawFields[properties] type = %T, want map[string]interface{}", m.RawFields["properties"])
	}
	if props["foo"] != "bar" {
		t.Errorf("properties[foo] = %v, want %q", props["foo"], "bar")
	}
}

// TestBrowseMessagesFilterQuery covers that every queue.MessageFilter field
// is sent as its corresponding nested "filter." list-messages query param.
func TestBrowseMessagesFilterQuery(t *testing.T) {
	from := time.Date(2025, 1, 31, 8, 30, 0, 0, time.UTC)
	to := time.Date(2025, 2, 1, 17, 0, 0, 0, time.UTC)
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got, want := q.Get("filter.jmsType"), "order-created"; got != want {
			t.Errorf("filter.jmsType = %q, want %q", got, want)
		}
		if got, want := q.Get("filter.messageId"), "ID:m1"; got != want {
			t.Errorf("filter.messageId = %q, want %q", got, want)
		}
		if got, want := q.Get("filter.fromDate"), "2025-01-31T08:30:00Z"; got != want {
			t.Errorf("filter.fromDate = %q, want %q", got, want)
		}
		if got, want := q.Get("filter.toDate"), "2025-02-01T17:00:00Z"; got != want {
			t.Errorf("filter.toDate = %q, want %q", got, want)
		}
		if got, want := q.Get("filter.maxCount"), "10"; got != want {
			t.Errorf("filter.maxCount = %q, want %q", got, want)
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}, "errors": []any{}})
	}))
	defer stop()

	_, _, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{
		JMSType: "order-created", MessageID: "ID:m1", FromDate: from, ToDate: to, MaxCount: 10,
	})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
}

// TestBrowseMessagesNonStringHeaderValue covers a real JMS-management API
// implementing this contract that reports header/property values with their
// real JMS types (numbers, booleans, ...) instead of mq-proxy's always-a-string
// convention — decoding must not fail just because a header value isn't a
// JSON string.
func TestBrowseMessagesNonStringHeaderValue(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{
					"sourceQueue": "orders", "messageId": "ID:m5", "jmsType": "text",
					"timestamp": "2024-01-01T00:00:00Z", "body": "hi",
					"headers": map[string]any{"retryCount": 3, "urgent": true, "note": "ok"},
				},
			},
			"errors": []any{},
		})
	}))
	defer stop()

	msgs, _, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	props, ok := msgs[0].RawFields["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("RawFields[properties] type = %T, want map[string]interface{}", msgs[0].RawFields["properties"])
	}
	if props["retryCount"] != float64(3) {
		t.Errorf("properties[retryCount] = %v, want 3", props["retryCount"])
	}
	if props["urgent"] != true {
		t.Errorf("properties[urgent] = %v, want true", props["urgent"])
	}
	if props["note"] != "ok" {
		t.Errorf("properties[note] = %v, want %q", props["note"], "ok")
	}
}

func TestBrowseMessagesIncomplete(t *testing.T) {
	tests := []struct {
		name     string
		envelope map[string]any
		want     bool
	}{
		{"hasMore", map[string]any{"data": []any{}, "errors": []any{}, "hasMore": true}, true},
		{"truncated", map[string]any{"data": []any{}, "errors": []any{}, "hasMore": false, "truncated": true}, true},
		{"neither", map[string]any{"data": []any{}, "errors": []any{}, "hasMore": false, "truncated": false}, false},
		{"absent", map[string]any{"data": []any{}, "errors": []any{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, tt.envelope)
			}))
			defer stop()

			_, got, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
			if err != nil {
				t.Fatalf("BrowseMessages() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("BrowseMessages() incomplete = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrowseMessagesEmptyJMSTypeNilBody(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"sourceQueue": "orders", "messageId": "ID:m2", "jmsType": "", "timestamp": "2024-01-01T00:00:00Z", "body": nil, "headers": map[string]string{}},
			},
			"errors": []any{},
		})
	}))
	defer stop()

	msgs, _, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if msgs[0].JMSType != "other" {
		t.Errorf("JMSType = %q, want %q (inferred, since mq-proxy reported an empty jmsType)", msgs[0].JMSType, "other")
	}
	if !msgs[0].JMSTypeInferred {
		t.Error("JMSTypeInferred = false, want true")
	}
	if msgs[0].Preview != "" {
		t.Errorf("Preview = %q, want empty", msgs[0].Preview)
	}
}

func TestBrowseMessagesEmptyJMSTypeWithBodyInfersText(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"sourceQueue": "orders", "messageId": "ID:m3", "jmsType": "", "timestamp": "2024-01-01T00:00:00Z", "body": "hi", "headers": map[string]string{}},
			},
			"errors": []any{},
		})
	}))
	defer stop()

	msgs, _, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if msgs[0].JMSType != "text" {
		t.Errorf("JMSType = %q, want %q (inferred from body presence)", msgs[0].JMSType, "text")
	}
	if !msgs[0].JMSTypeInferred {
		t.Error("JMSTypeInferred = false, want true")
	}
}

func TestBrowseMessagesPreviewTruncated(t *testing.T) {
	longBody := strings.Repeat("a", previewMaxLen+100)
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"sourceQueue": "orders", "messageId": "ID:m4", "jmsType": "text", "timestamp": "2024-01-01T00:00:00Z", "body": longBody, "headers": map[string]string{}},
			},
			"errors": []any{},
		})
	}))
	defer stop()

	msgs, _, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len([]rune(msgs[0].Preview)) != previewMaxLen {
		t.Errorf("Preview len = %d, want %d", len([]rune(msgs[0].Preview)), previewMaxLen)
	}
}

// ── PurgeQueue ────────────────────────────────────────────────────────────────

func TestPurgeQueue(t *testing.T) {
	calls := 0
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost || r.URL.Path != "/api/management/command/delete-messages" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var reqs []deleteMessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(reqs) != 1 || reqs[0].SourceQueue != "orders" || reqs[0].Filter.MaxCount != nil {
			t.Errorf("unexpected request %+v, want a single empty-filter entry for orders", reqs)
		}
		data := []map[string]string{{"messageId": "ID:1"}, {"messageId": "ID:2"}, {"messageId": "ID:3"}}
		if calls > 1 {
			data = nil // the queue is empty now
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "errors": []any{}})
	}))
	defer stop()

	n, err := c.PurgeQueue(context.Background(), "orders")
	if err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
	if n != 3 {
		t.Errorf("PurgeQueue() = %d, want 3 (the deleted messages the server reported)", n)
	}
	if calls != 2 {
		t.Errorf("requests = %d, want 2 (one deleting, one confirming nothing is left)", calls)
	}
}

func TestPurgeQueueError(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "broker error"})
	}))
	defer stop()

	if _, err := c.PurgeQueue(context.Background(), "orders"); err == nil {
		t.Fatal("PurgeQueue() error = nil, want non-nil")
	}
}

// ── RemoveMessage ─────────────────────────────────────────────────────────────

func TestRemoveMessage(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		var reqs []deleteMessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(reqs) != 1 || reqs[0].SourceQueue != "orders" || reqs[0].Filter.MessageID != "ID:m1" || reqs[0].Filter.MaxCount == nil || *reqs[0].Filter.MaxCount != 1 {
			t.Errorf("unexpected request %+v", reqs)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []map[string]string{{"messageId": "ID:m1"}},
			"errors": []any{},
		})
	}))
	defer stop()

	if err := c.RemoveMessage(context.Background(), "orders", "ID:m1"); err != nil {
		t.Fatalf("RemoveMessage() error = %v", err)
	}
}

func TestRemoveMessageNotFound(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []any{},
			"errors": []any{},
		})
	}))
	defer stop()

	if err := c.RemoveMessage(context.Background(), "orders", "ID:gone"); err == nil {
		t.Fatal("RemoveMessage() error = nil, want non-nil when the filter matched nothing")
	}
}

// ── MoveMessage ───────────────────────────────────────────────────────────────

func TestMoveMessage(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost || r.URL.Path != "/api/management/command/move-messages" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var reqs []moveMessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(reqs) != 1 || reqs[0].SourceQueue != "orders" || reqs[0].TargetQueue != "dlq" || reqs[0].Filter.MessageID != "ID:m1" {
			t.Errorf("unexpected request %+v", reqs)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []map[string]string{{"messageId": "ID:m1"}},
			"errors": []any{},
		})
	}))
	defer stop()

	if err := c.MoveMessage(context.Background(), "orders", "ID:m1", "dlq"); err != nil {
		t.Fatalf("MoveMessage() error = %v", err)
	}
}

func TestMoveMessageNotFound(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []any{},
			"errors": []any{},
		})
	}))
	defer stop()

	if err := c.MoveMessage(context.Background(), "orders", "ID:gone", "dlq"); err == nil {
		t.Fatal("MoveMessage() error = nil, want non-nil when the filter matched nothing")
	}
}

// ── MoveAllMessages ───────────────────────────────────────────────────────────

func TestMoveAllMessages(t *testing.T) {
	calls := 0
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		checkBasicAuth(t, r)
		var reqs []moveMessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(reqs) != 1 || reqs[0].Filter.MaxCount != nil {
			t.Errorf("unexpected request %+v, want a single empty-filter entry", reqs)
		}
		data := []map[string]string{
			{"messageId": "ID:1"}, {"messageId": "ID:2"}, {"messageId": "ID:3"},
			{"messageId": "ID:4"}, {"messageId": "ID:5"}, {"messageId": "ID:6"}, {"messageId": "ID:7"},
		}
		if calls > 1 {
			data = nil // the source queue is empty now
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "errors": []any{}})
	}))
	defer stop()

	n, err := c.MoveAllMessages(context.Background(), "orders", "archive")
	if err != nil {
		t.Fatalf("MoveAllMessages() error = %v", err)
	}
	if n != 7 {
		t.Errorf("MoveAllMessages() = %d, want 7", n)
	}
}

func TestMoveAllMessagesError(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "broker error"})
	}))
	defer stop()

	_, err := c.MoveAllMessages(context.Background(), "orders", "archive")
	if err == nil {
		t.Fatal("MoveAllMessages() error = nil, want non-nil")
	}
}

// ── DeleteMessages / MoveMessages (filtered bulk) ────────────────────────────

func TestDeleteMessagesSendsFilter(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqs []deleteMessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(reqs) != 1 {
			t.Fatalf("len(reqs) = %d, want 1", len(reqs))
		}
		f := reqs[0].Filter
		if f.JMSType != "order-created" || f.MaxCount == nil || *f.MaxCount != 10 {
			t.Errorf("filter = %+v, want jmsType=order-created maxCount=10", f)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []map[string]string{{"messageId": "ID:1"}, {"messageId": "ID:2"}},
			"errors": []any{},
		})
	}))
	defer stop()

	n, err := c.DeleteMessages(context.Background(), "orders", queue.MessageFilter{JMSType: "order-created", MaxCount: 10})
	if err != nil {
		t.Fatalf("DeleteMessages() error = %v", err)
	}
	if n != 2 {
		t.Errorf("DeleteMessages() = %d, want 2", n)
	}
}

func TestMoveMessagesSendsFilter(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqs []moveMessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(reqs) != 1 || reqs[0].TargetQueue != "archive" {
			t.Fatalf("unexpected request %+v", reqs)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []map[string]string{{"messageId": "ID:1"}},
			"errors": []any{},
		})
	}))
	defer stop()

	n, err := c.MoveMessages(context.Background(), "orders", "archive", queue.MessageFilter{MessageID: "ID:1", MaxCount: 1})
	if err != nil {
		t.Fatalf("MoveMessages() error = %v", err)
	}
	if n != 1 {
		t.Errorf("MoveMessages() = %d, want 1", n)
	}
}

// ── Bulk rounds ───────────────────────────────────────────────────────────────

// cappedServer fakes delete-messages/move-messages on a queue holding
// remaining messages, acting on at most perRound of them per request (the
// way a server that browses before consuming stops at the broker's browse
// limit). It records each request's filter; failOn makes that request
// (1-based) fail with HTTP 502, and stuck makes every request report
// perRound messages without ever running out. Every message reported gets
// its own ID, like a real server; noID adds that many entries without a
// messageId to every response.
type cappedServer struct {
	remaining int
	perRound  int
	failOn    int
	stuck     bool
	noID      int
	nextID    int
	filters   []queueMessageFilter
}

func (s *cappedServer) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqs []struct {
			Filter queueMessageFilter `json:"filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil || len(reqs) != 1 {
			t.Errorf("decode request: %v (%d entries)", err, len(reqs))
		}
		s.filters = append(s.filters, reqs[0].Filter)
		if len(s.filters) == s.failOn {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "broker error"})
			return
		}
		n := min(s.remaining, s.perRound)
		if s.stuck {
			n = s.perRound
		}
		s.remaining -= n
		data := make([]map[string]string, 0, n+s.noID)
		for range n {
			data = append(data, map[string]string{"messageId": fmt.Sprintf("ID:%d", s.nextID)})
			s.nextID++
		}
		for range s.noID {
			data = append(data, map[string]string{"messageId": ""})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "errors": []any{}})
	})
}

// TestBulkCountsOnlyEntriesWithAnID: an entry without a messageId isn't
// evidence that a message was removed (a server that browses before
// consuming can report one whose consume came back empty).
func TestBulkCountsOnlyEntriesWithAnID(t *testing.T) {
	s := &cappedServer{remaining: 500, perRound: 200, noID: 2}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.PurgeQueue(context.Background(), "orders")

	if n != 500 {
		t.Errorf("PurgeQueue() = %d, want 500 (the ID-less entries don't count)", n)
	}
	// The ID-less entries keep coming once the real messages are gone — a
	// round with nothing new — so it ends as incomplete, not after 1,000.
	if !errors.Is(err, errBulkNoProgress) || len(s.filters) != 4 {
		t.Errorf("PurgeQueue() error = %v after %d requests, want errBulkNoProgress after 4", err, len(s.filters))
	}
}

func TestSingleMessageCallsTreatAnIDLessEntryAsNotFound(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func(*Client) error
	}{
		{"RemoveMessage", func(c *Client) error { return c.RemoveMessage(context.Background(), "orders", "ID:1") }},
		{"MoveMessage", func(c *Client) error { return c.MoveMessage(context.Background(), "orders", "ID:1", "archive") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, map[string]any{"data": []map[string]string{{"messageId": ""}}, "errors": []any{}})
			}))
			defer stop()

			err := tt.call(c)
			if err == nil || !strings.Contains(err.Error(), "not found") {
				t.Errorf("error = %v, want \"not found\" — the server didn't say which message it acted on", err)
			}
		})
	}
}

func TestPurgeQueueRepeatsUntilNothingIsLeft(t *testing.T) {
	s := &cappedServer{remaining: 1000, perRound: 400}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.PurgeQueue(context.Background(), "orders")
	if err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
	if n != 1000 {
		t.Errorf("PurgeQueue() = %d, want 1000", n)
	}
	if len(s.filters) != 4 {
		t.Errorf("requests = %d, want 4 (400 + 400 + 200 + an empty one)", len(s.filters))
	}
}

func TestPurgeQueueOnEmptyQueueMakesOneRequest(t *testing.T) {
	s := &cappedServer{remaining: 0, perRound: 400}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.PurgeQueue(context.Background(), "orders")
	if err != nil || n != 0 {
		t.Fatalf("PurgeQueue() = %d, %v; want 0, nil", n, err)
	}
	if len(s.filters) != 1 {
		t.Errorf("requests = %d, want 1", len(s.filters))
	}
}

func TestMoveAllMessagesRepeatsUntilNothingIsLeft(t *testing.T) {
	s := &cappedServer{remaining: 900, perRound: 400}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.MoveAllMessages(context.Background(), "orders", "archive")
	if err != nil {
		t.Fatalf("MoveAllMessages() error = %v", err)
	}
	if n != 900 {
		t.Errorf("MoveAllMessages() = %d, want 900", n)
	}
}

func TestDeleteMessagesWithoutMaxCountRepeatsWithTheSameFilter(t *testing.T) {
	s := &cappedServer{remaining: 500, perRound: 200}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.DeleteMessages(context.Background(), "orders", queue.MessageFilter{JMSType: "TypeB"})
	if err != nil {
		t.Fatalf("DeleteMessages() error = %v", err)
	}
	if n != 500 {
		t.Errorf("DeleteMessages() = %d, want 500", n)
	}
	for i, f := range s.filters {
		if f.JMSType != "TypeB" || f.MaxCount != nil {
			t.Errorf("request %d filter = %+v, want jmsType=TypeB and no maxCount", i+1, f)
		}
	}
}

func TestMoveMessagesWithoutMaxCountRepeats(t *testing.T) {
	s := &cappedServer{remaining: 500, perRound: 200}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.MoveMessages(context.Background(), "orders", "archive", queue.MessageFilter{JMSType: "TypeB"})
	if err != nil {
		t.Fatalf("MoveMessages() error = %v", err)
	}
	if n != 500 {
		t.Errorf("MoveMessages() = %d, want 500", n)
	}
}

func TestBulkCallsWithMaxCountMakeOneRequest(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func(*Client) error
	}{
		{"RemoveMessage", func(c *Client) error { return c.RemoveMessage(context.Background(), "orders", "ID:0") }},
		{"MoveMessage", func(c *Client) error { return c.MoveMessage(context.Background(), "orders", "ID:0", "archive") }},
		{"DeleteMessages", func(c *Client) error {
			_, err := c.DeleteMessages(context.Background(), "orders", queue.MessageFilter{MaxCount: 10})
			return err
		}},
		{"MoveMessages", func(c *Client) error {
			_, err := c.MoveMessages(context.Background(), "orders", "archive", queue.MessageFilter{MaxCount: 10})
			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &cappedServer{remaining: 1000, perRound: 1}
			c, stop := newTestClient(t, s.handler(t))
			defer stop()

			if err := tt.call(c); err != nil {
				t.Fatalf("error = %v", err)
			}
			if len(s.filters) != 1 {
				t.Errorf("requests = %d, want 1", len(s.filters))
			}
		})
	}
}

func TestPurgeQueueStopsAtTheRoundLimit(t *testing.T) {
	s := &cappedServer{perRound: 400, stuck: true}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.PurgeQueue(context.Background(), "orders")
	if !errors.Is(err, errBulkIncomplete) {
		t.Fatalf("PurgeQueue() error = %v, want errBulkIncomplete", err)
	}
	if len(s.filters) != maxBulkRounds {
		t.Errorf("requests = %d, want %d", len(s.filters), maxBulkRounds)
	}
	if want := maxBulkRounds * 400; n != want {
		t.Errorf("PurgeQueue() = %d, want %d (the total so far)", n, want)
	}
}

func TestPurgeQueueErrorMidwayReportsTheTotalSoFar(t *testing.T) {
	s := &cappedServer{remaining: 1000, perRound: 400, failOn: 2}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.PurgeQueue(context.Background(), "orders")
	if err == nil {
		t.Fatal("PurgeQueue() error = nil, want the failed second request")
	}
	if n != 400 {
		t.Errorf("PurgeQueue() = %d, want 400 (deleted before the failure)", n)
	}
	if len(s.filters) != 2 {
		t.Errorf("requests = %d, want 2 (POSTs don't retry)", len(s.filters))
	}
}

func TestInRoundsStopsWhenTheContextIsCancelledBetweenRounds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rounds := 0

	n, err := inRounds(ctx, func() ([]string, int, error) {
		rounds++
		cancel() // cancelled after this round completes
		return []string{"ID:1", "ID:2"}, 2, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("inRounds() error = %v, want context.Canceled", err)
	}
	if n != 2 || rounds != 1 {
		t.Errorf("inRounds() = %d after %d rounds, want 2 after 1", n, rounds)
	}
}

// TestPurgeQueueStopsWhenTheServerKeepsReportingTheSameMessage is the
// regression test for spec-wip bugfix-bulk-progress-guard: a server that
// reports a browsed message it couldn't consume in every round used to keep
// the client repeating until maxBulkRounds.
func TestPurgeQueueStopsWhenTheServerKeepsReportingTheSameMessage(t *testing.T) {
	calls := 0
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		data := []map[string]string{{"messageId": "ID:stuck"}}
		if calls <= 3 { // 1,000 real messages in rounds of 400, 400, 200
			for i := range min(400, 1000-(calls-1)*400) {
				data = append(data, map[string]string{"messageId": fmt.Sprintf("ID:%d-%d", calls, i)})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "errors": []any{}})
	}))
	defer stop()

	n, err := c.PurgeQueue(context.Background(), "orders")

	if !errors.Is(err, errBulkNoProgress) {
		t.Fatalf("PurgeQueue() error = %v, want errBulkNoProgress", err)
	}
	if n != 1000 {
		t.Errorf("PurgeQueue() = %d, want 1000 (the stuck one, still reported at the end, wasn't removed)", n)
	}
	if calls != 4 {
		t.Errorf("requests = %d, want 4 (3 with progress, 1 reporting only the stuck message)", calls)
	}
	if msg := err.Error(); !strings.Contains(msg, "kept reporting 1 message(s)") || !strings.Contains(msg, "1000 removed") {
		t.Errorf("error = %q, want it to name the 1 message kept reported and the 1000 removed", msg)
	}
}

func TestPurgeQueueStopsWhenOnlyIDLessEntriesAreReported(t *testing.T) {
	s := &cappedServer{noID: 3}
	c, stop := newTestClient(t, s.handler(t))
	defer stop()

	n, err := c.PurgeQueue(context.Background(), "orders")

	if !errors.Is(err, errBulkNoProgress) {
		t.Fatalf("PurgeQueue() error = %v, want errBulkNoProgress", err)
	}
	if n != 0 || len(s.filters) != 1 {
		t.Errorf("PurgeQueue() = %d after %d requests, want 0 after 1", n, len(s.filters))
	}
}

func TestMoveAllMessagesStopsWhenTheServerMakesNoProgress(t *testing.T) {
	calls := 0
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, http.StatusOK, map[string]any{"data": []map[string]string{{"messageId": "ID:stuck"}}, "errors": []any{}})
	}))
	defer stop()

	n, err := c.MoveAllMessages(context.Background(), "orders", "archive")

	if !errors.Is(err, errBulkNoProgress) || n != 0 || calls != 2 {
		t.Errorf("MoveAllMessages() = %d, %v after %d requests; want 0, errBulkNoProgress after 2", n, err, calls)
	}
}

// ── SendMessage ───────────────────────────────────────────────────────────────

func TestSendMessage(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost || r.URL.Path != "/api/management/command/send-message" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var req sendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.TargetQueue != "orders" || req.JMSType != "order.created" || req.Body != `{"text":"hello"}` ||
			req.CorrelationID != "corr-1" || req.GroupID != "group-1" || req.Headers["custom"] != "value" {
			t.Errorf("unexpected request %+v", req)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data":  map[string]string{"messageId": "ID:sent-1"},
			"error": nil,
		})
	}))
	defer stop()

	sendReq := queue.SendMessageRequest{
		JMSType:       "order.created",
		Body:          `{"text":"hello"}`,
		CorrelationID: "corr-1",
		GroupID:       "group-1",
		Headers:       map[string]string{"custom": "value"},
	}
	if err := c.SendMessage(context.Background(), "orders", sendReq); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
}

func TestSendMessageError(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "broker down"})
	}))
	defer stop()

	if err := c.SendMessage(context.Background(), "orders", queue.SendMessageRequest{JMSType: "text", Body: "{}"}); err == nil {
		t.Fatal("SendMessage() error = nil, want non-nil")
	}
}

func TestSendMessageEnvelopeError(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data":  nil,
			"error": map[string]string{"code": "INVALID_QUEUE", "message": "no such queue"},
		})
	}))
	defer stop()

	if err := c.SendMessage(context.Background(), "orders", queue.SendMessageRequest{JMSType: "text", Body: "{}"}); err == nil {
		t.Fatal("SendMessage() error = nil, want non-nil for a populated error field")
	}
}

// ── Cold-start retry (Bugfix 58) ────────────────────────────────────────────
//
// hijackAndClose simulates the transport-level failure a cold mq-proxy
// instance produces (the client's Do call fails with no response ever
// received) without waiting out a real timeout: it takes over the raw TCP
// connection and closes it immediately, before any HTTP response is
// written.

func hijackAndClose(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("ResponseWriter does not support Hijacker")
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		t.Fatalf("Hijack() error = %v", err)
	}
	conn.Close()
}

func TestGetRetriesOnceOnTransportError(t *testing.T) {
	var calls atomic.Int32
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			hijackAndClose(t, w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"name": "orders", "messageCount": 1, "consumerCount": 0, "enqueuedCount": 0, "dequeuedCount": 0, "producerCount": 0},
			},
			"errors": []any{},
		})
	}))
	defer stop()

	summaries, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v, want the second (successful) attempt's result", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "orders" {
		t.Errorf("List() = %+v, want the second attempt's data", summaries)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("handler called %d times, want exactly 2 (initial attempt + one retry)", got)
	}
}

func TestGetFailsAfterTwoTransportErrors(t *testing.T) {
	var calls atomic.Int32
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		hijackAndClose(t, w)
	}))
	defer stop()

	if _, err := c.List(context.Background()); err == nil {
		t.Fatal("List() error = nil, want an error when both attempts fail")
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("handler called %d times, want exactly 2 (no more than one retry)", got)
	}
}

func TestPostDoesNotRetryOnTransportError(t *testing.T) {
	var calls atomic.Int32
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		// Always hijack-and-close: a second (POST) attempt would also
		// fail here, but the test only cares that no second attempt is
		// made at all.
		hijackAndClose(t, w)
	}))
	defer stop()

	if err := c.SendMessage(context.Background(), "orders", queue.SendMessageRequest{JMSType: "text", Body: "{}"}); err == nil {
		t.Fatal("SendMessage() error = nil, want an error from the single failed attempt")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("handler called %d times, want exactly 1 (a POST must never be retried)", got)
	}
}
