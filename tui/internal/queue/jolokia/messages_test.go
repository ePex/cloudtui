package jolokia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

func TestBrowseMessagesHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value": []map[string]any{
				{
					"messageId": "ID:msg-1",
					"timestamp": int64(1721000000000),
					"text":      "Hello World",
				},
				{
					"messageId": "ID:msg-2",
					"timestamp": int64(1721000001000),
					"text":      "",
				},
			},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	msgs, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	if msgs[0].ID != "ID:msg-1" {
		t.Errorf("msgs[0].ID = %q, want %q", msgs[0].ID, "ID:msg-1")
	}
	if msgs[0].Preview != "Hello World" {
		t.Errorf("msgs[0].Preview = %q, want %q", msgs[0].Preview, "Hello World")
	}
	if msgs[1].Preview != "(binary)" {
		t.Errorf("msgs[1].Preview = %q, want %q", msgs[1].Preview, "(binary)")
	}
	if msgs[0].Timestamp.IsZero() {
		t.Error("msgs[0].Timestamp is zero")
	}
}

// TestBrowseMessagesFiltersFullPath verifies BrowseMessages applies filter
// (via filterMessages) to the browseMessagesFull() result before returning.
func TestBrowseMessagesFiltersFullPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value": []map[string]any{
				{"messageId": "ID:msg-1", "timestamp": int64(1721000000000), "text": "Hello World"},
				{"messageId": "ID:msg-2", "timestamp": int64(1721000001000), "text": "Second"},
			},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	msgs, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{MessageID: "ID:msg-2"})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if msgs[0].ID != "ID:msg-2" {
		t.Errorf("msgs[0].ID = %q, want %q", msgs[0].ID, "ID:msg-2")
	}
}

func TestBrowseMessagesCompositeDataMessageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value": []map[string]any{
				{
					"messageId": map[string]any{
						"producerId": map[string]any{
							"connectionId": map[string]any{
								"value": "ID:myhost-1234-1700000000000-1",
							},
							"sessionId": float64(2),
							"value":     float64(3),
						},
						"producerSequenceId": float64(1),
						"brokerSequenceId":   float64(224369),
					},
					"timestamp": int64(1721000000000),
					"text":      "hello",
				},
			},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	msgs, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	want := "ID:myhost-1234-1700000000000-1:2:3:1"
	if msgs[0].ID != want {
		t.Errorf("msgs[0].ID = %q, want %q", msgs[0].ID, want)
	}
}

func TestBrowseMessagesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{})
	if err == nil {
		t.Fatal("BrowseMessages() expected error for HTTP 500, got nil")
	}
}

func TestBrowseMessagesJolokiaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 500,
			"error":  "operation failed",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{})
	if err == nil {
		t.Fatal("BrowseMessages() expected error for Jolokia status 500, got nil")
	}
}

func TestBrowseMessagesBackfillsBytesMessageBody(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"status": 200,
				"value": []map[string]any{
					{
						"messageId":  "ID:msg-1",
						"timestamp":  float64(1721000000000),
						"bodyLength": float64(11),
					},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value":  []string{"hello world"},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	msgs, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if msgs[0].ID != "ID:msg-1" {
		t.Errorf("msgs[0].ID = %q, want %q", msgs[0].ID, "ID:msg-1")
	}
	if msgs[0].Preview != "hello world" {
		t.Errorf("msgs[0].Preview = %q, want %q — body not backfilled from browse()", msgs[0].Preview, "hello world")
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (browseMessages + browse backfill)", callCount)
	}
}

// TestBrowseMessagesFallbackFullObject verifies that when browseMessages()
// fails and browse() returns full message objects (ActiveMQ 5.18+ format),
// the fallback produces fully-populated messages: ID, body, headers, timestamp.
func TestBrowseMessagesFallbackFullObject(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"status": 500,
				"error":  "java.lang.IllegalStateException: Error while extracting clientID",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":200,"value":[{` +
			`"JMSMessageID":"ID:host-1234-100-1:1:1:1:1",` +
			`"Text":"hello world",` +
			`"JMSType":null,` +
			`"JMSCorrelationID":null,` +
			`"JMSTimestamp":"2026-08-06T14:59:07Z",` +
			`"JMSDeliveryMode":"PERSISTENT",` +
			`"JMSPriority":4,` +
			`"JMSRedelivered":false,` +
			`"JMSDestination":"queue://myQueue",` +
			`"JMSExpiration":0,` +
			`"JMSReplyTo":null,` +
			`"JMSXGroupID":null,` +
			`"JMSXGroupSeq":0,` +
			`"JMSXUserID":null,` +
			`"StringProperties":{},` +
			`"IntProperties":{},` +
			`"LongProperties":{},` +
			`"ByteProperties":{},` +
			`"ShortProperties":{},` +
			`"FloatProperties":{},` +
			`"DoubleProperties":{},` +
			`"BooleanProperties":{}` +
			`}]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	msgs, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if msgs[0].ID != "ID:host-1234-100-1:1:1:1:1" {
		t.Errorf("ID = %q, want full JMSMessageID", msgs[0].ID)
	}
	if msgs[0].Preview != "hello world" {
		t.Errorf("Preview = %q, want %q", msgs[0].Preview, "hello world")
	}
	if got, _ := msgs[0].RawFields["text"].(string); got != "hello world" {
		t.Errorf("RawFields[text] = %q, want %q", got, "hello world")
	}
	if got, _ := msgs[0].RawFields["jMSDeliveryMode"].(any); got == nil {
		t.Error("RawFields[jMSDeliveryMode] is nil")
	}
	if msgs[0].Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
}

func TestBrowseMessagesFallback(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"status": 500,
				"error":  "java.lang.IllegalStateException: Error while extracting clientID",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value":  []string{"hello world"},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	msgs, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if msgs[0].ID != "" {
		t.Errorf("msgs[0].ID = %q, want empty string (fallback has no ID)", msgs[0].ID)
	}
	if msgs[0].Preview != "hello world" {
		t.Errorf("msgs[0].Preview = %q, want %q", msgs[0].Preview, "hello world")
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (browseMessages + browse fallback)", callCount)
	}
}

// TestBrowseMessagesFiltersFallbackPath verifies BrowseMessages applies
// filter (via filterMessages) to the browse()-fallback result too, not just
// the browseMessagesFull() path.
func TestBrowseMessagesFiltersFallbackPath(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"status": 500,
				"error":  "java.lang.IllegalStateException: Error while extracting clientID",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value":  []string{"first", "second"},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	msgs, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{MaxCount: 1})
	if err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1 (MaxCount filter applied to fallback result)", len(msgs))
	}
}

func TestBrowseMessagesFallbackBothFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 500,
			"error":  "operation failed",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.BrowseMessages(context.Background(), "myQueue", queue.MessageFilter{})
	if err == nil {
		t.Fatal("BrowseMessages() expected error when both operations fail, got nil")
	}
	if got := err.Error(); got == "" {
		t.Error("error message is empty")
	}
}
