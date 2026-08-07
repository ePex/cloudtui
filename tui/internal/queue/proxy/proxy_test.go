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
		if r.URL.Path != "/api/queues" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		writeJSON(w, http.StatusOK, []map[string]interface{}{
			{"name": "orders", "pendingCount": 5, "consumerCount": 1, "enqueueCount": 10, "dequeueCount": 5},
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

// ── BrowseMessages ────────────────────────────────────────────────────────────

func TestBrowseMessages(t *testing.T) {
	body := "hello world"
	ts := "2024-01-01T00:00:00Z"
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.URL.Path != "/api/queues/orders/messages" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		writeJSON(w, http.StatusOK, []map[string]interface{}{
			{"id": "ID:m1", "timestamp": ts, "body": body, "properties": map[string]string{"foo": "bar"}},
		})
	}))
	defer stop()

	msgs, err := c.BrowseMessages(context.Background(), "orders")
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
	if m.JMSType != "text" {
		t.Errorf("JMSType = %q, want %q", m.JMSType, "text")
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

func TestBrowseMessagesNilBody(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]interface{}{
			{"id": "ID:m2", "timestamp": "2024-01-01T00:00:00Z", "body": nil, "properties": map[string]string{}},
		})
	}))
	defer stop()

	msgs, err := c.BrowseMessages(context.Background(), "orders")
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if msgs[0].JMSType != "other" {
		t.Errorf("JMSType = %q, want %q", msgs[0].JMSType, "other")
	}
	if msgs[0].Preview != "" {
		t.Errorf("Preview = %q, want empty", msgs[0].Preview)
	}
}

func TestBrowseMessagesPreviewTruncated(t *testing.T) {
	longBody := strings.Repeat("a", 100)
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []map[string]interface{}{
			{"id": "ID:m3", "timestamp": "2024-01-01T00:00:00Z", "body": longBody, "properties": map[string]string{}},
		})
	}))
	defer stop()

	msgs, err := c.BrowseMessages(context.Background(), "orders")
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
		if r.Method != http.MethodDelete || r.URL.Path != "/api/queues/orders/messages" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]int{"purged": 3})
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
		if r.Method != http.MethodDelete || r.URL.Path != "/api/queues/orders/messages/ID:m1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer stop()

	if err := c.RemoveMessage(context.Background(), "orders", "ID:m1"); err != nil {
		t.Fatalf("RemoveMessage() error = %v", err)
	}
}

func TestRemoveMessageNotFound(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}))
	defer stop()

	if err := c.RemoveMessage(context.Background(), "orders", "ID:gone"); err == nil {
		t.Fatal("RemoveMessage() error = nil, want non-nil")
	}
}

// ── MoveMessage ───────────────────────────────────────────────────────────────

func TestMoveMessage(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		wantPath := "/api/queues/orders/messages/ID:m1/move"
		if r.Method != http.MethodPost || r.URL.Path != wantPath {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("to") != "dlq" {
			t.Errorf("query param to = %q, want %q", r.URL.Query().Get("to"), "dlq")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer stop()

	if err := c.MoveMessage(context.Background(), "orders", "ID:m1", "dlq"); err != nil {
		t.Fatalf("MoveMessage() error = %v", err)
	}
}

func TestMoveMessageNotFound(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}))
	defer stop()

	if err := c.MoveMessage(context.Background(), "orders", "ID:gone", "dlq"); err == nil {
		t.Fatal("MoveMessage() error = nil, want non-nil")
	}
}

// ── MoveAllMessages ───────────────────────────────────────────────────────────

func TestMoveAllMessages(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost || r.URL.Path != "/api/queues/orders/move" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("to") != "archive" {
			t.Errorf("query param to = %q, want %q", r.URL.Query().Get("to"), "archive")
		}
		writeJSON(w, http.StatusOK, map[string]int{"moved": 7})
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

// ── SendMessage ───────────────────────────────────────────────────────────────

func TestSendMessage(t *testing.T) {
	c, stop := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkBasicAuth(t, r)
		if r.Method != http.MethodPost || r.URL.Path != "/api/queues/orders/messages" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		w.WriteHeader(http.StatusCreated)
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
