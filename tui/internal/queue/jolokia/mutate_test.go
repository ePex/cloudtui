package jolokia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	if err := c.SendMessage(context.Background(), "myQueue", "hello world"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	wantOp := "sendTextMessage(java.util.Map,java.lang.String,java.lang.String,java.lang.String)"
	if got := capturedBody["operation"]; got != wantOp {
		t.Errorf("operation = %q, want %q", got, wantOp)
	}
	args, _ := capturedBody["arguments"].([]any)
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
