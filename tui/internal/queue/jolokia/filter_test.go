package jolokia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

func TestFilterMessages(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)
	msgs := []queue.Message{
		{ID: "ID:1", JMSType: "order-created", Timestamp: t1},
		{ID: "ID:2", JMSType: "order-cancelled", Timestamp: t2},
		{ID: "ID:3", JMSType: "order-created", Timestamp: t3},
	}

	tests := []struct {
		name   string
		filter queue.MessageFilter
		want   []string
	}{
		{"empty filter matches everything", queue.MessageFilter{}, []string{"ID:1", "ID:2", "ID:3"}},
		{"jmsType", queue.MessageFilter{JMSType: "order-created"}, []string{"ID:1", "ID:3"}},
		{"messageId", queue.MessageFilter{MessageID: "ID:2"}, []string{"ID:2"}},
		{"fromDate", queue.MessageFilter{FromDate: t2}, []string{"ID:2", "ID:3"}},
		{"toDate", queue.MessageFilter{ToDate: t2}, []string{"ID:1", "ID:2"}},
		{"date range", queue.MessageFilter{FromDate: t2, ToDate: t2}, []string{"ID:2"}},
		{"jmsType and date range combined", queue.MessageFilter{JMSType: "order-created", FromDate: t2}, []string{"ID:3"}},
		{"maxCount truncates", queue.MessageFilter{MaxCount: 2}, []string{"ID:1", "ID:2"}},
		{"maxCount larger than matches", queue.MessageFilter{JMSType: "order-created", MaxCount: 10}, []string{"ID:1", "ID:3"}},
		{"no matches", queue.MessageFilter{MessageID: "ID:missing"}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterMessages(msgs, tt.filter)
			ids := make([]string, len(got))
			for i, m := range got {
				ids[i] = m.ID
			}
			if len(ids) != len(tt.want) {
				t.Fatalf("filterMessages() = %v, want %v", ids, tt.want)
			}
			for i := range ids {
				if ids[i] != tt.want[i] {
					t.Errorf("filterMessages() = %v, want %v", ids, tt.want)
					break
				}
			}
		})
	}
}

// jolokiaOp decodes just enough of a Jolokia exec request to route a fake
// handler by operation name.
type jolokiaOp struct {
	Operation string   `json:"operation"`
	Arguments []string `json:"arguments"`
}

func TestClientDeleteMessages(t *testing.T) {
	var removedIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var op jolokiaOp
		if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch {
		case op.Operation == "browseMessages()":
			json.NewEncoder(w).Encode(map[string]any{
				"status": 200,
				"value": []map[string]any{
					{"messageId": "ID:1", "timestamp": int64(1721000000000), "text": "a", "jMSType": "order-created"},
					{"messageId": "ID:2", "timestamp": int64(1721000001000), "text": "b", "jMSType": "order-cancelled"},
					{"messageId": "ID:3", "timestamp": int64(1721000002000), "text": "c", "jMSType": "order-created"},
				},
			})
		case op.Operation == "removeMessage(java.lang.String)":
			removedIDs = append(removedIDs, op.Arguments[0])
			json.NewEncoder(w).Encode(map[string]any{"status": 200, "value": true})
		default:
			t.Fatalf("unexpected operation %q", op.Operation)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	n, err := c.DeleteMessages(context.Background(), "orders", queue.MessageFilter{JMSType: "order-created"})
	if err != nil {
		t.Fatalf("DeleteMessages() error = %v", err)
	}
	if n != 2 {
		t.Errorf("DeleteMessages() = %d, want 2", n)
	}
	if len(removedIDs) != 2 || removedIDs[0] != "ID:1" || removedIDs[1] != "ID:3" {
		t.Errorf("removed IDs = %v, want [ID:1 ID:3]", removedIDs)
	}
}

func TestClientMoveMessages(t *testing.T) {
	var movedArgs [][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var op jolokiaOp
		if err := json.NewDecoder(r.Body).Decode(&op); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		switch {
		case op.Operation == "browseMessages()":
			json.NewEncoder(w).Encode(map[string]any{
				"status": 200,
				"value": []map[string]any{
					{"messageId": "ID:1", "timestamp": int64(1721000000000), "text": "a"},
					{"messageId": "ID:2", "timestamp": int64(1721000001000), "text": "b"},
				},
			})
		case op.Operation == "moveMessageTo(java.lang.String,java.lang.String)":
			movedArgs = append(movedArgs, op.Arguments)
			json.NewEncoder(w).Encode(map[string]any{"status": 200, "value": true})
		default:
			t.Fatalf("unexpected operation %q", op.Operation)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	n, err := c.MoveMessages(context.Background(), "orders", "archive", queue.MessageFilter{MaxCount: 1})
	if err != nil {
		t.Fatalf("MoveMessages() error = %v", err)
	}
	if n != 1 {
		t.Errorf("MoveMessages() = %d, want 1", n)
	}
	if len(movedArgs) != 1 || movedArgs[0][0] != "ID:1" || movedArgs[0][1] != "archive" {
		t.Errorf("moved args = %v, want [[ID:1 archive]]", movedArgs)
	}
}
