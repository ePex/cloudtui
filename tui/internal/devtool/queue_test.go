package devtool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestAddQueueSendsCorrectJMXRequest(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := r.Header.Get("Origin"); got != "http://localhost" {
			t.Errorf("Origin header = %q, want %q", got, "http://localhost")
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("basic auth = (%q, %q, %v), want (admin, secret, true)", user, pass, ok)
		}
		json.NewEncoder(w).Encode(map[string]any{"status": 200, "value": nil})
	}))
	defer srv.Close()

	cfg := config.QueueConfig{BrokerName: "localhost", URL: srv.URL, Username: "admin", Password: "secret"}
	if err := AddQueue(context.Background(), cfg, "my.test.queue"); err != nil {
		t.Fatalf("AddQueue() error = %v", err)
	}

	if got := gotBody["operation"]; got != "addQueue(java.lang.String)" {
		t.Errorf("operation = %v, want addQueue(java.lang.String)", got)
	}
	if got := gotBody["mbean"]; got != "org.apache.activemq:type=Broker,brokerName=localhost" {
		t.Errorf("mbean = %v, want org.apache.activemq:type=Broker,brokerName=localhost", got)
	}
	args, ok := gotBody["arguments"].([]any)
	if !ok || len(args) != 1 || args[0] != "my.test.queue" {
		t.Errorf("arguments = %v, want [\"my.test.queue\"]", gotBody["arguments"])
	}
}

func TestRemoveQueueSendsCorrectOperation(t *testing.T) {
	var gotOp string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotOp, _ = body["operation"].(string)
		json.NewEncoder(w).Encode(map[string]any{"status": 200})
	}))
	defer srv.Close()

	cfg := config.QueueConfig{BrokerName: "localhost", URL: srv.URL}
	if err := RemoveQueue(context.Background(), cfg, "my.test.queue"); err != nil {
		t.Fatalf("RemoveQueue() error = %v", err)
	}
	if want := "removeQueue(java.lang.String)"; gotOp != want {
		t.Errorf("operation = %q, want %q", gotOp, want)
	}
}

func TestExecBrokerOpReturnsErrorOnJolokiaFailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": 404,
			"error":  "javax.management.InstanceNotFoundException",
		})
	}))
	defer srv.Close()

	cfg := config.QueueConfig{BrokerName: "localhost", URL: srv.URL}
	err := RemoveQueue(context.Background(), cfg, "does.not.exist")
	if err == nil {
		t.Fatal("RemoveQueue() error = nil, want non-nil for a Jolokia failure status")
	}
}

func TestExecBrokerOpReturnsErrorOnTransportFailure(t *testing.T) {
	cfg := config.QueueConfig{BrokerName: "localhost", URL: "http://127.0.0.1:1"} // nothing listens here
	if err := AddQueue(context.Background(), cfg, "x"); err == nil {
		t.Fatal("AddQueue() error = nil, want non-nil when the server is unreachable")
	}
}
