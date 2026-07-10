package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

func newTestBackend(t *testing.T, handler http.HandlerFunc) (*Backend, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	backend, err := New(server.URL, "user", "pass")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return backend, server
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatal(err)
	}
}

func requireBasicAuth(t *testing.T, r *http.Request) {
	t.Helper()
	user, pass, ok := r.BasicAuth()
	if !ok || user != "user" || pass != "pass" {
		t.Errorf("request missing expected Basic Auth: ok=%v user=%q pass=%q", ok, user, pass)
	}
}

func TestListSuccess(t *testing.T) {
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		requireBasicAuth(t, r)
		writeJSON(t, w, http.StatusOK, []map[string]any{
			{"name": "foo", "pendingCount": 3, "consumerCount": 1},
		})
	})

	got, err := backend.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []queue.Summary{{Name: "foo", PendingCount: 3, ConsumerCount: 1}}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("List() = %+v, want %+v", got, want)
	}
}

func TestListError(t *testing.T) {
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, map[string]string{"message": "bad credentials"})
	})

	_, err := backend.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bad credentials") {
		t.Errorf("List() error = %v, want it to mention %q", err, "bad credentials")
	}
}

func TestBrowseSuccess(t *testing.T) {
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("limit"), "50"; got != want {
			t.Errorf("limit query param = %q, want %q", got, want)
		}
		writeJSON(t, w, http.StatusOK, []map[string]any{
			{"id": "1", "body": "hello", "properties": map[string]string{"k": "v"}},
		})
	})

	got, err := backend.Browse(context.Background(), "myqueue", 50)
	if err != nil {
		t.Fatalf("Browse() error = %v", err)
	}
	want := []queue.Message{{ID: "1", Body: "hello", Properties: map[string]string{"k": "v"}}}
	if len(got) != 1 || got[0].ID != want[0].ID || got[0].Body != want[0].Body {
		t.Errorf("Browse() = %+v, want %+v", got, want)
	}
}

func TestBrowseNotFound(t *testing.T) {
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "queue not found: myqueue"})
	})

	_, err := backend.Browse(context.Background(), "myqueue", 10)
	if err == nil || !strings.Contains(err.Error(), "queue not found") {
		t.Errorf("Browse() error = %v, want it to mention %q", err, "queue not found")
	}
}

func TestSendSuccess(t *testing.T) {
	var gotBody map[string]any
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		requireBasicAuth(t, r)
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
	})

	err := backend.Send(context.Background(), "myqueue", "hello", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotBody["body"] != "hello" {
		t.Errorf("sent body = %v, want %q", gotBody["body"], "hello")
	}
}

func TestSendError(t *testing.T) {
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusInternalServerError, map[string]string{"message": "broker unreachable"})
	})

	err := backend.Send(context.Background(), "myqueue", "hello", nil)
	if err == nil || !strings.Contains(err.Error(), "broker unreachable") {
		t.Errorf("Send() error = %v, want it to mention %q", err, "broker unreachable")
	}
}

func TestPurgeSuccess(t *testing.T) {
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		requireBasicAuth(t, r)
		w.WriteHeader(http.StatusNoContent)
	})

	if err := backend.Purge(context.Background(), "myqueue"); err != nil {
		t.Errorf("Purge() error = %v", err)
	}
}

func TestPurgeNotFound(t *testing.T) {
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "queue not found: myqueue"})
	})

	err := backend.Purge(context.Background(), "myqueue")
	if err == nil || !strings.Contains(err.Error(), "queue not found") {
		t.Errorf("Purge() error = %v, want it to mention %q", err, "queue not found")
	}
}

func TestMoveSuccess(t *testing.T) {
	var gotBody map[string]any
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		requireBasicAuth(t, r)
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	max := 5
	err := backend.Move(context.Background(), "source", "target", &max)
	if err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if gotBody["targetQueue"] != "target" {
		t.Errorf("sent targetQueue = %v, want %q", gotBody["targetQueue"], "target")
	}
	if gotBody["maxMessages"] != float64(5) {
		t.Errorf("sent maxMessages = %v, want %v", gotBody["maxMessages"], 5)
	}
}

func TestMoveNotFound(t *testing.T) {
	backend, _ := newTestBackend(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "queue not found: source"})
	})

	err := backend.Move(context.Background(), "source", "target", nil)
	if err == nil || !strings.Contains(err.Error(), "queue not found") {
		t.Errorf("Move() error = %v, want it to mention %q", err, "queue not found")
	}
}
