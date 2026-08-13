package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		for _, key := range []string{"jmsType", "messageId", "fromDate", "toDate", "maxCount"} {
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

	msgs, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
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
// is sent as its corresponding list-messages query param.
func TestBrowseMessagesFilterQuery(t *testing.T) {
	from := time.Date(2025, 1, 31, 8, 30, 0, 0, time.UTC)
	to := time.Date(2025, 2, 1, 17, 0, 0, 0, time.UTC)
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got, want := q.Get("jmsType"), "order-created"; got != want {
			t.Errorf("jmsType = %q, want %q", got, want)
		}
		if got, want := q.Get("messageId"), "ID:m1"; got != want {
			t.Errorf("messageId = %q, want %q", got, want)
		}
		if got, want := q.Get("fromDate"), "2025-01-31T08:30:00Z"; got != want {
			t.Errorf("fromDate = %q, want %q", got, want)
		}
		if got, want := q.Get("toDate"), "2025-02-01T17:00:00Z"; got != want {
			t.Errorf("toDate = %q, want %q", got, want)
		}
		if got, want := q.Get("maxCount"), "10"; got != want {
			t.Errorf("maxCount = %q, want %q", got, want)
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}, "errors": []any{}})
	}))
	defer stop()

	_, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{
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

	msgs, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
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

	msgs, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if msgs[0].JMSType != "other" {
		t.Errorf("JMSType = %q, want %q (inferred, since mq-proxy reported an empty jmsType)", msgs[0].JMSType, "other")
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

	msgs, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if msgs[0].JMSType != "text" {
		t.Errorf("JMSType = %q, want %q (inferred from body presence)", msgs[0].JMSType, "text")
	}
}

func TestBrowseMessagesPreviewTruncated(t *testing.T) {
	longBody := strings.Repeat("a", 100)
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]any{
				{"sourceQueue": "orders", "messageId": "ID:m4", "jmsType": "text", "timestamp": "2024-01-01T00:00:00Z", "body": longBody, "headers": map[string]string{}},
			},
			"errors": []any{},
		})
	}))
	defer stop()

	msgs, err := c.BrowseMessages(context.Background(), "orders", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len([]rune(msgs[0].Preview)) != 80 {
		t.Errorf("Preview len = %d, want 80", len([]rune(msgs[0].Preview)))
	}
}

// ── PurgeQueue ────────────────────────────────────────────────────────────────

func TestPurgeQueue(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusOK, map[string]any{
			"data":   []map[string]string{{"messageId": "ID:1"}, {"messageId": "ID:2"}, {"messageId": "ID:3"}},
			"errors": []any{},
		})
	}))
	defer stop()

	if err := c.PurgeQueue(context.Background(), "orders"); err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
}

func TestPurgeQueueError(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "broker error"})
	}))
	defer stop()

	if err := c.PurgeQueue(context.Background(), "orders"); err == nil {
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
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		var reqs []moveMessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(reqs) != 1 || reqs[0].Filter.MaxCount != nil {
			t.Errorf("unexpected request %+v, want a single empty-filter entry", reqs)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []map[string]string{
				{"messageId": "ID:1"}, {"messageId": "ID:2"}, {"messageId": "ID:3"},
				{"messageId": "ID:4"}, {"messageId": "ID:5"}, {"messageId": "ID:6"}, {"messageId": "ID:7"},
			},
			"errors": []any{},
		})
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
		if req.TargetQueue != "orders" || req.JMSType != "text" || req.Body != `{"text":"hello"}` {
			t.Errorf("unexpected request %+v", req)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data":  map[string]string{"messageId": "ID:sent-1"},
			"error": nil,
		})
	}))
	defer stop()

	if err := c.SendMessage(context.Background(), "orders", `{"text":"hello"}`); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
}

func TestSendMessageError(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "broker down"})
	}))
	defer stop()

	if err := c.SendMessage(context.Background(), "orders", `{}`); err == nil {
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

	if err := c.SendMessage(context.Background(), "orders", `{}`); err == nil {
		t.Fatal("SendMessage() error = nil, want non-nil for a populated error field")
	}
}
