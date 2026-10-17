package proxy

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/queue/httplog"
)

// newLoggedTestClient is newTestClient with request logging into a
// buffer, one JSON line per request.
func newLoggedTestClient(t *testing.T, h http.Handler) (*Client, *bytes.Buffer, func()) {
	t.Helper()
	c, cleanup := newTestClient(t, h)
	c.WithRequestLog("staging")
	tr, ok := c.httpClient.Transport.(*httplog.Transport)
	if !ok {
		t.Fatalf("WithRequestLog left transport %T, want *httplog.Transport", c.httpClient.Transport)
	}
	var logs bytes.Buffer
	tr.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	return c, &logs, cleanup
}

// logLines decodes every JSON log line.
func logLines(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line %q isn't JSON: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func TestWithRequestLogLogsEachRequest(t *testing.T) {
	c, logs, cleanup := newLoggedTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
	}))
	defer cleanup()

	if _, err := c.List(t.Context()); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if _, _, err := c.BrowseMessages(t.Context(), "orders", queue.MessageFilter{JMSType: "OrderCreated", MaxCount: 10}); err != nil {
		t.Fatalf("BrowseMessages() error = %v", err)
	}

	lines := logLines(t, logs)
	if len(lines) != 2 {
		t.Fatalf("logged %d lines, want 2:\n%s", len(lines), logs)
	}
	wantOps := []string{"/api/management/command/list-queues", "/api/management/command/list-messages"}
	for i, line := range lines {
		if line["level"] != "INFO" || line["backend"] != "proxy" || line["connection"] != "staging" {
			t.Errorf("line %d = %v, want INFO for backend proxy, connection staging", i, line)
		}
		if line["op"] != wantOps[i] {
			t.Errorf("line %d op = %v, want %q", i, line["op"], wantOps[i])
		}
	}
	if url, _ := lines[1]["url"].(string); !strings.Contains(url, "orders") || !strings.Contains(url, "OrderCreated") {
		t.Errorf("browse URL %q, want the queue name and JMS type filter in it", url)
	}
	if strings.Contains(logs.String(), "pass") {
		t.Errorf("log contains the password:\n%s", logs)
	}
}

// TestWithRequestLogLogsTheWarmupRetry: a GET whose first attempt gets
// no response is retried (doRequest); both attempts are logged, the
// failed one as a WARN.
func TestWithRequestLogLogsTheWarmupRetry(t *testing.T) {
	var calls atomic.Int32
	c, logs, cleanup := newLoggedTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close() // no response at all, like a proxy still warming up
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
	}))
	defer cleanup()

	if _, err := c.List(t.Context()); err != nil {
		t.Fatalf("List() error = %v (the retry should have succeeded)", err)
	}

	lines := logLines(t, logs)
	if len(lines) != 2 {
		t.Fatalf("logged %d lines, want 2 (failed attempt, retry):\n%s", len(lines), logs)
	}
	if lines[0]["level"] != "WARN" || lines[0]["error"] == nil {
		t.Errorf("first attempt = %v, want a WARN with the error", lines[0])
	}
	if lines[1]["level"] != "INFO" || lines[1]["status"] != float64(http.StatusOK) {
		t.Errorf("retry = %v, want INFO with status 200", lines[1])
	}
}
