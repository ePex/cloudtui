package export

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

func jolokiaMessage() queue.Message {
	return queue.Message{
		ID:            "ID:1",
		JMSType:       "OrderCreated",
		CorrelationID: "corr-1",
		Timestamp:     time.Date(2026, 10, 10, 15, 30, 0, 123_000_000, time.FixedZone("CEST", 2*3600)),
		Preview:       "trimmed",
		RawFields: map[string]any{
			"text":             `{"id":1,"note":"<b>"}`,
			"jMSCorrelationID": "corr-1",
			"jMSPriority":      float64(4),
			"jMSRedelivered":   false,
			"jMSReplyTo":       nil,
			"properties": map[string]any{
				"tenant": map[string]any{"data": []any{float64('a'), float64('b')}},
				"count":  float64(3),
			},
		},
	}
}

func TestNewRecord(t *testing.T) {
	r := NewRecord("orders", jolokiaMessage())

	if r.Queue != "orders" || r.MessageID != "ID:1" || r.JMSType != "OrderCreated" || r.CorrelationID != "corr-1" {
		t.Errorf("record = %+v", r)
	}
	if r.Timestamp != "2026-10-10T13:30:00.123Z" {
		t.Errorf("timestamp = %q, want RFC 3339 in UTC with milliseconds", r.Timestamp)
	}
	if r.Body == nil || *r.Body != `{"id":1,"note":"<b>"}` {
		t.Errorf("body = %v, want the full body, not the preview", r.Body)
	}
	want := map[string]any{"Priority": float64(4), "JMSRedelivered": false, "tenant": "ab", "count": "3"}
	if len(r.Headers) != len(want) {
		t.Errorf("headers = %v, want %v (unset JMS headers and the correlation ID left out)", r.Headers, want)
	}
	for k, v := range want {
		if r.Headers[k] != v {
			t.Errorf("headers[%q] = %v, want %v", k, r.Headers[k], v)
		}
	}
}

func TestNewRecordOmitsWhatIsUnknown(t *testing.T) {
	m := queue.Message{ID: "ID:2", JMSType: "bytes", JMSTypeInferred: true} // no body, no timestamp, no raw fields

	line, err := json.Marshal(NewRecord("orders", m))
	if err != nil {
		t.Fatal(err)
	}

	got := string(line)
	want := `{"queue":"orders","messageId":"ID:2","headers":{},"body":null}`
	if got != want {
		t.Errorf("record = %s, want %s (inferred type, unset correlation ID and timestamp omitted; headers {}, body null)", got, want)
	}
}

func readLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec map[string]any
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("line %q isn't valid JSON: %v", sc.Text(), err)
		}
		out = append(out, rec)
	}
	return out
}

func TestWriteFileOneLinePerMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.ndjson")
	msgs := []queue.Message{jolokiaMessage(), {ID: "ID:2"}, {ID: "ID:3"}}

	if err := WriteFile(path, "orders", msgs); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 3 || lines[0]["messageId"] != "ID:1" || lines[2]["messageId"] != "ID:3" {
		t.Errorf("lines = %v, want the 3 messages in order", lines)
	}
	raw, _ := os.ReadFile(path)
	if !strings.HasSuffix(string(raw), "\n") || strings.Contains(string(raw), "\r") {
		t.Error("file should end every line with \\n and contain no \\r on any OS")
	}
	if !strings.Contains(string(raw), `<b>`) { // HTML-escaped, < and > would be unicode escapes
		t.Error("bodies should stay readable: < and > were HTML-escaped")
	}
}

func TestWriteFileReplacesAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.ndjson")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(path, "orders", []queue.Message{{ID: "ID:1"}}); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if lines := readLines(t, path); len(lines) != 1 || lines[0]["messageId"] != "ID:1" {
		t.Errorf("lines = %v, want the new export", lines)
	}
}

func TestWriteFileFailureLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing-dir", "out.ndjson")

	if err := WriteFile(path, "orders", []queue.Message{{ID: "ID:1"}}); err == nil {
		t.Fatal("WriteFile() into a missing directory: error = nil")
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("left behind %v", entries)
	}
}

// TestWriteFileCleansUpWhenTheFinalMoveFails: the temp file is written
// fine, but moving it into place fails (path is a directory) — the temp
// file must not be left behind, and what was at path stays as it was.
func TestWriteFileCleansUpWhenTheFinalMoveFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "taken")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(path, "orders", []queue.Message{{ID: "ID:1"}}); err == nil {
		t.Fatal("WriteFile() onto a directory: error = nil")
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "taken" {
		t.Errorf("dir holds %v, want only the untouched 'taken' (no temp file left)", entries)
	}
	if _, err := os.Stat(filepath.Join(path, "keep")); err != nil {
		t.Errorf("existing content damaged: %v", err)
	}
}

func TestDefaultFileName(t *testing.T) {
	now := time.Date(2026, 10, 10, 17, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	tests := []struct{ queue, want string }{
		{"orders", "orders-20261010T153000Z.ndjson"},
		{`a/b\c:d*e?f"g<h>i|j`, "a_b_c_d_e_f_g_h_i_j-20261010T153000Z.ndjson"},
	}
	for _, tt := range tests {
		if got := DefaultFileName(tt.queue, now); got != tt.want {
			t.Errorf("DefaultFileName(%q) = %q, want %q", tt.queue, got, tt.want)
		}
	}
}

// setHome points the home directory at dir on every OS (HOME on Unix,
// USERPROFILE on Windows — os.UserHomeDir reads the one for its OS).
func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestDefaultDir(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	if got := DefaultDir(); got != home {
		t.Errorf("without Downloads: DefaultDir() = %q, want home %q", got, home)
	}

	downloads := filepath.Join(home, "Downloads")
	if err := os.Mkdir(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DefaultDir(); got != downloads {
		t.Errorf("with Downloads: DefaultDir() = %q, want %q", got, downloads)
	}
}
