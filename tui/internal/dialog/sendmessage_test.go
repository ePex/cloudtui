package dialog

import (
	"regexp"
	"testing"
)

// ── buildSendMessageRequest ──────────────────────────────────────────────

func TestBuildSendMessageRequestRejectsBlankJMSType(t *testing.T) {
	_, err := buildSendMessageRequest("  ", "corr-1", "", "", "hello", func() string { return "generated" })
	if err == nil {
		t.Fatal("buildSendMessageRequest() error = nil, want non-nil for a blank JMS Type")
	}
}

func TestBuildSendMessageRequestRejectsBlankBody(t *testing.T) {
	_, err := buildSendMessageRequest("text", "corr-1", "", "", "   ", func() string { return "generated" })
	if err == nil {
		t.Fatal("buildSendMessageRequest() error = nil, want non-nil for a blank Body")
	}
}

func TestBuildSendMessageRequestFallsBackToGeneratedCorrelationID(t *testing.T) {
	req, err := buildSendMessageRequest("text", "  ", "", "", "hello", func() string { return "generated-id" })
	if err != nil {
		t.Fatalf("buildSendMessageRequest() error = %v", err)
	}
	if req.CorrelationID != "generated-id" {
		t.Errorf("CorrelationID = %q, want %q (a blank field falls back to newID())", req.CorrelationID, "generated-id")
	}
}

func TestBuildSendMessageRequestKeepsProvidedCorrelationID(t *testing.T) {
	req, err := buildSendMessageRequest("text", "corr-1", "", "", "hello", func() string {
		t.Fatal("newID() called even though a Correlation ID was provided")
		return ""
	})
	if err != nil {
		t.Fatalf("buildSendMessageRequest() error = %v", err)
	}
	if req.CorrelationID != "corr-1" {
		t.Errorf("CorrelationID = %q, want %q", req.CorrelationID, "corr-1")
	}
}

func TestBuildSendMessageRequestTrimsAndPassesThroughFields(t *testing.T) {
	req, err := buildSendMessageRequest(" order.created ", " corr-1 ", " group-1 ", "custom: value", "  hello world  ", func() string { return "generated" })
	if err != nil {
		t.Fatalf("buildSendMessageRequest() error = %v", err)
	}
	if req.JMSType != "order.created" {
		t.Errorf("JMSType = %q, want %q", req.JMSType, "order.created")
	}
	if req.GroupID != "group-1" {
		t.Errorf("GroupID = %q, want %q", req.GroupID, "group-1")
	}
	if req.Body != "  hello world  " {
		t.Errorf("Body = %q, want the untrimmed original %q", req.Body, "  hello world  ")
	}
	if req.Headers["custom"] != "value" {
		t.Errorf("Headers[custom] = %q, want %q", req.Headers["custom"], "value")
	}
}

func TestBuildSendMessageRequestSurfacesHeaderParseError(t *testing.T) {
	_, err := buildSendMessageRequest("text", "corr-1", "", "not-a-valid-line", "hello", func() string { return "generated" })
	if err == nil {
		t.Fatal("buildSendMessageRequest() error = nil, want non-nil for a malformed Headers line")
	}
}

// ── parseHeaders ─────────────────────────────────────────────────────────

func TestParseHeadersEmptyTextReturnsNilMap(t *testing.T) {
	headers, err := parseHeaders("")
	if err != nil {
		t.Fatalf("parseHeaders() error = %v", err)
	}
	if headers != nil {
		t.Errorf("headers = %v, want nil for empty text", headers)
	}
}

func TestParseHeadersSkipsBlankLines(t *testing.T) {
	headers, err := parseHeaders("foo: bar\n\n   \nbaz: qux\n")
	if err != nil {
		t.Fatalf("parseHeaders() error = %v", err)
	}
	if len(headers) != 2 || headers["foo"] != "bar" || headers["baz"] != "qux" {
		t.Errorf("headers = %v, want {foo: bar, baz: qux}", headers)
	}
}

func TestParseHeadersTrimsKeyAndValue(t *testing.T) {
	headers, err := parseHeaders("  foo  :  bar  ")
	if err != nil {
		t.Fatalf("parseHeaders() error = %v", err)
	}
	if headers["foo"] != "bar" {
		t.Errorf(`headers["foo"] = %q, want %q`, headers["foo"], "bar")
	}
}

func TestParseHeadersRejectsLineWithoutColon(t *testing.T) {
	_, err := parseHeaders("foo bar")
	if err == nil {
		t.Fatal("parseHeaders() error = nil, want non-nil for a line with no ':' separator")
	}
}

func TestParseHeadersRejectsEmptyKey(t *testing.T) {
	_, err := parseHeaders(": bar")
	if err == nil {
		t.Fatal("parseHeaders() error = nil, want non-nil for a line with an empty key")
	}
}

func TestParseHeadersAllowsEmptyValue(t *testing.T) {
	headers, err := parseHeaders("foo:")
	if err != nil {
		t.Fatalf("parseHeaders() error = %v", err)
	}
	if v, ok := headers["foo"]; !ok || v != "" {
		t.Errorf(`headers["foo"] = %q, ok = %v, want "", true`, v, ok)
	}
}

// ── newCorrelationID ─────────────────────────────────────────────────────

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewCorrelationIDMatchesUUIDv4Shape(t *testing.T) {
	id := newCorrelationID()
	if !uuidV4Pattern.MatchString(id) {
		t.Errorf("newCorrelationID() = %q, want a v4 UUID matching %s", id, uuidV4Pattern.String())
	}
}

func TestNewCorrelationIDIsNotRepeated(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newCorrelationID()
		if seen[id] {
			t.Fatalf("newCorrelationID() returned %q twice across 100 calls", id)
		}
		seen[id] = true
	}
}
