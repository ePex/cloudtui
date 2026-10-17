package jolokia

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/queue/httplog"
)

const ordersMBean = "org.apache.activemq:type=Broker,brokerName=localhost,destinationType=Queue,destinationName=orders"

func TestDescribeRequest(t *testing.T) {
	cases := []struct {
		name   string
		method string
		url    string
		body   string
		want   string
	}{
		{
			name:   "single exec, arguments left out",
			method: http.MethodPost,
			body:   `{"type":"exec","mbean":"` + ordersMBean + `","operation":"removeMessage(java.lang.String)","arguments":["ID:secret-id"]}`,
			want:   "exec removeMessage on orders",
		},
		{
			name:   "single read",
			method: http.MethodPost,
			body:   `{"type":"read","mbean":"` + ordersMBean + `","attribute":"QueueSize"}`,
			want:   "read QueueSize on orders",
		},
		{
			name:   "an MBean that names no queue",
			method: http.MethodPost,
			body:   `{"type":"read","mbean":"org.apache.activemq:type=Broker,brokerName=localhost","attribute":"BrokerVersion"}`,
			want:   "read BrokerVersion on org.apache.activemq:type=Broker,brokerName=localhost",
		},
		{
			name:   "bulk read of two queues",
			method: http.MethodPost,
			body: `[` +
				`{"type":"read","mbean":"a","attribute":"QueueSize"},{"type":"read","mbean":"a","attribute":"ConsumerCount"},` +
				`{"type":"read","mbean":"a","attribute":"EnqueueCount"},{"type":"read","mbean":"a","attribute":"DequeueCount"},` +
				`{"type":"read","mbean":"b","attribute":"QueueSize"},{"type":"read","mbean":"b","attribute":"ConsumerCount"},` +
				`{"type":"read","mbean":"b","attribute":"EnqueueCount"},{"type":"read","mbean":"b","attribute":"DequeueCount"}]`,
			want: "bulk: 8 × read QueueSize/ConsumerCount/EnqueueCount/DequeueCount",
		},
		{
			name:   "mixed bulk",
			method: http.MethodPost,
			body:   `[{"type":"read","mbean":"a","attribute":"QueueSize"},{"type":"exec","mbean":"a","operation":"purge()"},{"type":"read","mbean":"b","attribute":"QueueSize"}]`,
			want:   "bulk: 3 × read QueueSize, exec purge",
		},
		{
			name:   "queue search",
			method: http.MethodGet,
			url:    "http://broker/api/jolokia/search/org.apache.activemq:type=Broker,destinationName=*",
			want:   "search",
		},
		{
			name:   "not JSON",
			method: http.MethodPost,
			body:   "not json",
			want:   "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			url := c.url
			if url == "" {
				url = "http://broker/api/jolokia"
			}
			req, err := http.NewRequest(c.method, url, strings.NewReader(c.body))
			if err != nil {
				t.Fatal(err)
			}
			if got := describeRequest(req); got != c.want {
				t.Errorf("describeRequest() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAppStatus(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		want   string
		wantOK bool
	}{
		{"single 200", `{"status":200,"value":3}`, "200", true},
		{"single error", `{"status":404,"error":"InstanceNotFoundException"}`, "404", false},
		{"bulk, all 200", `[{"status":200},{"status":200},{"status":200}]`, "200×3", true},
		{"bulk with errors", `[{"status":200},{"status":404},{"status":200}]`, "200×2, 404×1", false},
		{"no status", `{"value":3}`, "", true},
		{"not JSON", `<html>Server Error</html>`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := appStatus([]byte(c.body))
			if got != c.want || ok != c.wantOK {
				t.Errorf("appStatus(%s) = (%q, %v), want (%q, %v)", c.body, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// newLoggedTestClient is newTestClient with request logging into a
// buffer, one JSON line per request.
func newLoggedTestClient(t *testing.T, url string) (*Client, *bytes.Buffer) {
	t.Helper()
	c := newTestClient(url).WithRequestLog("local")
	var logs bytes.Buffer
	tr, ok := c.http.Transport.(*httplog.Transport)
	if !ok {
		t.Fatalf("WithRequestLog left transport %T, want *httplog.Transport", c.http.Transport)
	}
	tr.Logger = slog.New(slog.NewJSONHandler(&logs, nil))
	return c, &logs
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

// TestWithRequestLogLogsList: the queue list is a search and a bulk
// read, so two lines, each naming the backend and the connection.
func TestWithRequestLogLogsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]any{"status": 200, "value": searchMBeans})
			return
		}
		items := make([]map[string]any, 8)
		for i := range items {
			items[i] = map[string]any{"status": 200, "value": int64(0)}
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()
	c, logs := newLoggedTestClient(t, srv.URL)

	if _, err := c.List(t.Context()); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	lines := logLines(t, logs)
	if len(lines) != 2 {
		t.Fatalf("logged %d lines, want 2 (search, bulk read):\n%s", len(lines), logs)
	}
	wantOps := []string{"search", "bulk: 8 × read QueueSize/ConsumerCount/EnqueueCount/DequeueCount"}
	wantStatus := []string{"200", "200×8"}
	for i, line := range lines {
		if line["level"] != "INFO" || line["backend"] != "jolokia" || line["connection"] != "local" {
			t.Errorf("line %d = %v, want INFO for backend jolokia, connection local", i, line)
		}
		if line["op"] != wantOps[i] {
			t.Errorf("line %d op = %v, want %q", i, line["op"], wantOps[i])
		}
		if line["jolokiaStatus"] != wantStatus[i] {
			t.Errorf("line %d jolokiaStatus = %v, want %q", i, line["jolokiaStatus"], wantStatus[i])
		}
	}
	if strings.Contains(logs.String(), "admin") {
		t.Errorf("log contains the user name or password:\n%s", logs)
	}
}

// TestWithRequestLogWarnsOnAJolokiaError: Jolokia answers HTTP 200 for
// a failed operation; the line is a WARN with Jolokia's own status.
func TestWithRequestLogWarnsOnAJolokiaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": 403, "error": "access denied"})
	}))
	defer srv.Close()
	c, logs := newLoggedTestClient(t, srv.URL)

	if _, err := c.List(t.Context()); err == nil {
		t.Fatal("List() succeeded on a Jolokia error")
	}

	lines := logLines(t, logs)
	if len(lines) != 1 {
		t.Fatalf("logged %d lines, want 1:\n%s", len(lines), logs)
	}
	if lines[0]["level"] != "WARN" || lines[0]["status"] != float64(200) || lines[0]["jolokiaStatus"] != "403" {
		t.Errorf("line = %v, want WARN with HTTP status 200 and jolokiaStatus 403", lines[0])
	}
}
