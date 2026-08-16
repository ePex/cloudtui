package jolokia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

func newTestClient(url string) *Client {
	cfg := config.QueueConfig{
		BrokerName: "localhost",
		URL:        url,
		Username:   "admin",
		Password:   "admin",
	}
	return NewClient(cfg)
}

// searchMBeans is the list returned by the fake search endpoint.
var searchMBeans = []string{
	"org.apache.activemq:type=Broker,brokerName=localhost,destinationType=Queue,destinationName=foo",
	"org.apache.activemq:type=Broker,brokerName=localhost,destinationType=Queue,destinationName=bar",
}

func TestListHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// search response
			json.NewEncoder(w).Encode(map[string]any{
				"status": 200,
				"value":  searchMBeans,
			})
			return
		}
		// bulk read: QueueSize=3, ConsumerCount=1, EnqueueCount=10, DequeueCount=7 for foo
		//            0, 0, 0, 0 for bar
		json.NewEncoder(w).Encode([]map[string]any{
			{"status": 200, "value": int64(3)},
			{"status": 200, "value": int64(1)},
			{"status": 200, "value": int64(10)},
			{"status": 200, "value": int64(7)},
			{"status": 200, "value": int64(0)},
			{"status": 200, "value": int64(0)},
			{"status": 200, "value": int64(0)},
			{"status": 200, "value": int64(0)},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	summaries, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}
	if summaries[0].Name != "foo" {
		t.Errorf("summaries[0].Name = %q, want %q", summaries[0].Name, "foo")
	}
	if summaries[0].PendingCount != 3 {
		t.Errorf("summaries[0].PendingCount = %d, want 3", summaries[0].PendingCount)
	}
	if summaries[0].ConsumerCount != 1 {
		t.Errorf("summaries[0].ConsumerCount = %d, want 1", summaries[0].ConsumerCount)
	}
	if summaries[0].EnqueueCount != 10 {
		t.Errorf("summaries[0].EnqueueCount = %d, want 10", summaries[0].EnqueueCount)
	}
	if summaries[0].DequeueCount != 7 {
		t.Errorf("summaries[0].DequeueCount = %d, want 7", summaries[0].DequeueCount)
	}
	if summaries[1].Name != "bar" {
		t.Errorf("summaries[1].Name = %q, want %q", summaries[1].Name, "bar")
	}
}

func TestListHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.List(context.Background())
	if err == nil {
		t.Fatal("List() expected error for HTTP 500, got nil")
	}
}

func TestListJolokiaErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 404,
			"error":  "MBean not found",
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.List(context.Background())
	if err == nil {
		t.Fatal("List() expected error for Jolokia status 404, got nil")
	}
}

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
	args, _ := capturedBody["arguments"].([]interface{})
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
	if err := c.SendMessage(context.Background(), "myQueue", "hello world"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	wantOp := "sendTextMessage(java.util.Map,java.lang.String,java.lang.String,java.lang.String)"
	if got := capturedBody["operation"]; got != wantOp {
		t.Errorf("operation = %q, want %q", got, wantOp)
	}
	args, _ := capturedBody["arguments"].([]interface{})
	// args: [{}, body, username, password]
	if len(args) < 4 {
		t.Fatalf("arguments len = %d, want 4", len(args))
	}
	if args[1] != "hello world" {
		t.Errorf("arguments[1] (body) = %v, want \"hello world\"", args[1])
	}
	if args[2] != "admin" {
		t.Errorf("arguments[2] (username) = %v, want \"admin\"", args[2])
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
	err := c.SendMessage(context.Background(), "myQueue", "hello")
	if err == nil {
		t.Fatal("SendMessage() expected error for Jolokia status 500, got nil")
	}
}

func TestPurgeQueueDirectOperation(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value":  true,
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.PurgeQueue(context.Background(), "myQueue")
	if err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
	if requestCount != 1 {
		t.Errorf("requestCount = %d, want 1 (only purgeQueue())", requestCount)
	}
}

func TestPurgeQueueRemoveMatchingFallback(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"status": 500,
				"error":  "No operation purgeQueue found",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"value":  float64(3),
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.PurgeQueue(context.Background(), "myQueue")
	if err != nil {
		t.Fatalf("PurgeQueue() error = %v", err)
	}
	if requestCount != 2 {
		t.Errorf("requestCount = %d, want 2 (purgeQueue + removeMatchingMessages)", requestCount)
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
	if got, _ := msgs[0].RawFields["jMSDeliveryMode"].(interface{}); got == nil {
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
