package jolokia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
