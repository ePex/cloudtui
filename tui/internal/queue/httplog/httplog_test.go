package httplog

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// logBuffer collects log lines as JSON (safe for concurrent writes).
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// lines returns every logged line, decoded.
func (b *logBuffer) lines(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(b.String()), "\n") {
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

// onlyLine returns the one logged line, failing unless there's exactly one.
func (b *logBuffer) onlyLine(t *testing.T) map[string]any {
	t.Helper()
	lines := b.lines(t)
	if len(lines) != 1 {
		t.Fatalf("logged %d lines, want 1:\n%s", len(lines), b.String())
	}
	return lines[0]
}

// newTestClient returns an http.Client whose transport logs into a
// buffer, configured by setup.
func newTestClient(setup func(*Transport)) (*http.Client, *logBuffer) {
	logs := &logBuffer{}
	tr := &Transport{
		Backend:    "jolokia",
		Connection: "local",
		Logger:     slog.New(slog.NewJSONHandler(logs, nil)),
	}
	if setup != nil {
		setup(tr)
	}
	return &http.Client{Transport: tr}, logs
}

func wantAttr(t *testing.T, line map[string]any, key string, want any) {
	t.Helper()
	if got := line[key]; got != want {
		t.Errorf("%s = %#v, want %#v (line %v)", key, got, want, line)
	}
}

func TestTransportLogsASuccessfulRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status":200,"value":"ok"}`)
	}))
	defer srv.Close()
	client, logs := newTestClient(func(tr *Transport) {
		tr.Describe = func(*http.Request) string { return "read QueueSize" }
		tr.AppStatus = func([]byte) (string, bool) { return "200", true }
		tr.AppStatusKey = "jolokiaStatus"
	})

	resp, err := client.Post(srv.URL+"/api/jolokia", "application/json", strings.NewReader(`{"type":"read"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != `{"status":200,"value":"ok"}` {
		t.Errorf("client got body %q, want the server's body unchanged", body)
	}
	line := logs.onlyLine(t)
	wantAttr(t, line, "msg", Message)
	wantAttr(t, line, "level", "INFO")
	wantAttr(t, line, "backend", "jolokia")
	wantAttr(t, line, "connection", "local")
	wantAttr(t, line, "method", "POST")
	wantAttr(t, line, "url", srv.URL+"/api/jolokia")
	wantAttr(t, line, "op", "read QueueSize")
	wantAttr(t, line, "status", float64(200))
	wantAttr(t, line, "jolokiaStatus", "200")
	wantAttr(t, line, "bytes", float64(len(body)))
	if _, ok := line["duration"]; !ok {
		t.Errorf("no duration in %v", line)
	}
}

func TestTransportWarnsOnFailures(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		appStatus string
		appOK     bool
	}{
		{"HTTP 500", http.StatusInternalServerError, "", true},
		{"application error inside HTTP 200", http.StatusOK, "404", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				io.WriteString(w, "{}")
			}))
			defer srv.Close()
			client, logs := newTestClient(func(tr *Transport) {
				tr.AppStatus = func([]byte) (string, bool) { return c.appStatus, c.appOK }
			})

			resp, err := client.Get(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()

			line := logs.onlyLine(t)
			wantAttr(t, line, "level", "WARN")
			wantAttr(t, line, "status", float64(c.status))
		})
	}
}

func TestTransportWarnsWhenThereIsNoResponse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	url := "http://" + ln.Addr().String() + "/api/jolokia"
	ln.Close() // nothing listens there any more
	client, logs := newTestClient(nil)

	if _, err := client.Get(url); err == nil {
		t.Fatal("request to a closed port succeeded")
	}

	line := logs.onlyLine(t)
	wantAttr(t, line, "level", "WARN")
	if e, _ := line["error"].(string); e == "" {
		t.Errorf("no error in %v", line)
	}
	if _, ok := line["status"]; ok {
		t.Errorf("a status logged for a request without response: %v", line)
	}
}

func TestTransportLogsACancelledRequestAsCancelled(t *testing.T) {
	arrived := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		<-r.Context().Done() // never answers; the client gives up
	}))
	defer srv.Close()
	client, logs := newTestClient(nil)

	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	done := make(chan error, 1)
	go func() {
		_, err := client.Do(req)
		done <- err
	}()
	select {
	case <-arrived:
	case <-time.After(2 * time.Second):
		t.Fatal("request never reached the server")
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled request succeeded")
	}

	line := logs.onlyLine(t)
	wantAttr(t, line, "level", "INFO")
	wantAttr(t, line, "result", "cancelled")
}

// TestTransportNeverLogsSecrets: the password (in the Authorization
// header and in the URL) and the bodies don't appear anywhere in the
// log, whatever Describe/AppStatus see.
func TestTransportNeverLogsSecrets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"value":"response-secret"}`)
	}))
	defer srv.Close()
	client, logs := newTestClient(func(tr *Transport) {
		tr.Describe = func(req *http.Request) string {
			RequestBody(req)
			return "exec sendTextMessage"
		}
		tr.AppStatus = func([]byte) (string, bool) { return "200", true }
	})

	url := strings.Replace(srv.URL, "http://", "http://admin:url-secret@", 1) + "/api/jolokia"
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"arguments":["body-secret"]}`))
	req.SetBasicAuth("admin", "header-secret")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	out := logs.String()
	for _, secret := range []string{"url-secret", "header-secret", "body-secret", "response-secret", "Authorization", "YWRtaW46"} {
		if strings.Contains(out, secret) {
			t.Errorf("log contains %q:\n%s", secret, out)
		}
	}
	wantAttr(t, logs.onlyLine(t), "url", srv.URL+"/api/jolokia")
}

// TestTransportLeavesTheRequestBodyIntact: Describe reads the body to
// name the operation; the server must still get all of it.
func TestTransportLeavesTheRequestBodyIntact(t *testing.T) {
	const sent = `{"type":"exec","operation":"removeMessage(java.lang.String)","arguments":["ID:1"]}`
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()
	var described []byte
	client, _ := newTestClient(func(tr *Transport) {
		tr.Describe = func(req *http.Request) string {
			described = RequestBody(req)
			return "exec removeMessage"
		}
	})

	resp, err := client.Post(srv.URL, "application/json", strings.NewReader(sent))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if string(got) != sent {
		t.Errorf("server got body %q, want %q", got, sent)
	}
	if string(described) != sent {
		t.Errorf("Describe saw body %q, want %q", described, sent)
	}
}
