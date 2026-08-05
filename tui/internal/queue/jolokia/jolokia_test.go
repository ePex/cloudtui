package jolokia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
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
		// bulk read response: QueueSize=3, ConsumerCount=1 for foo; 0,0 for bar
		json.NewEncoder(w).Encode([]map[string]any{
			{"status": 200, "value": int64(3)},
			{"status": 200, "value": int64(1)},
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
