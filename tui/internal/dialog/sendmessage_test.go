package dialog

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/snippet"
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

// ── Load snippet ─────────────────────────────────────────────────────────

// newSendMessageFixture builds an open send-message overlay whose picker
// reads from a temp library holding created.json (with a JMS Type) and
// plain.txt (without one).
func newSendMessageFixture(t *testing.T) (*SendMessageOverlay, *testHost) {
	t.Helper()
	root := t.TempDir()
	writeSnippetFile(t, root, "created.json", "---\njmsType: OrderCreated\n---\n{\"id\":1}")
	writeSnippetFile(t, root, "plain.txt", "just a body")
	host := newTestHost()
	confirm := NewConfirmDialog(host)
	picker := NewSnippetPicker(host, snippet.NewStore(root))
	sm := NewSendMessageOverlay(host, picker, confirm)
	sm.Show("orders", func() {})
	return sm, host
}

// sendMessageFields returns the overlay's field values in form order.
func sendMessageFields(sm *SendMessageOverlay) [5]string {
	return [5]string{
		sm.jmsTypeItem.GetText(),
		sm.correlationIDItem.GetText(),
		sm.groupIDItem.GetText(),
		sm.headersItem.GetText(),
		sm.bodyItem.GetText(),
	}
}

func TestSendMessageLoadSnippetButton(t *testing.T) {
	sm, host := newSendMessageFixture(t)
	idx := sm.form.GetButtonIndex("Load snippet…")
	if idx < 0 {
		t.Fatal("no Load snippet… button")
	}
	if sm.form.GetButtonIndex("Submit") != 0 {
		t.Error("Submit is no longer button 0")
	}

	sm.form.GetButton(idx).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if !sm.snippetPicker.Visible() || host.focused != sm.snippetPicker.list {
		t.Error("Load snippet… did not open and focus the snippet picker")
	}
}

func TestSendMessageSetFromSnippetOnlyTouchesTypeAndBody(t *testing.T) {
	sm, _ := newSendMessageFixture(t)
	sm.correlationIDItem.SetText("corr-1")
	sm.groupIDItem.SetText("group-1")
	sm.headersItem.SetText("x: y", false)

	sm.setFromSnippet(snippet.Snippet{JMSType: "OrderCreated", Body: "{}"})

	want := [5]string{"OrderCreated", "corr-1", "group-1", "x: y", "{}"}
	if got := sendMessageFields(sm); got != want {
		t.Errorf("fields = %q, want %q", got, want)
	}
}

func TestSendMessageSetFromSnippetWithoutTypeClearsType(t *testing.T) {
	sm, _ := newSendMessageFixture(t)
	sm.jmsTypeItem.SetText("Old")
	sm.setFromSnippet(snippet.Snippet{Body: "b"})
	if got := sm.jmsTypeItem.GetText(); got != "" {
		t.Errorf("JMS Type = %q, want empty", got)
	}
}

func TestSendMessageLoadIntoEmptyFormSkipsConfirm(t *testing.T) {
	sm, host := newSendMessageFixture(t)
	corrID := sm.correlationIDItem.GetText()
	sm.openSnippetPicker()
	pickerSelect(t, sm.snippetPicker, "created.json")

	if sm.confirm.Visible() {
		t.Error("confirmation shown for an empty form")
	}
	if got := sm.jmsTypeItem.GetText(); got != "OrderCreated" {
		t.Errorf("JMS Type = %q, want OrderCreated", got)
	}
	if got := sm.bodyItem.GetText(); got != `{"id":1}` {
		t.Errorf("Body = %q, want {\"id\":1}", got)
	}
	if got := sm.correlationIDItem.GetText(); got != corrID {
		t.Errorf("Correlation ID changed from %q to %q", corrID, got)
	}
	if host.focused != sm.form {
		t.Error("focus not returned to the form")
	}
	if !strings.Contains(host.contextHint, "next field") {
		t.Errorf("context hint = %q, want the form's hint back", host.contextHint)
	}
}

func TestSendMessageLoadIntoFilledFormAsksFirst(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(*SendMessageOverlay)
	}{
		{"JMS Type filled", func(sm *SendMessageOverlay) { sm.jmsTypeItem.SetText("Mine") }},
		{"Body filled", func(sm *SendMessageOverlay) { sm.bodyItem.SetText("mine", false) }},
	} {
		t.Run(tt.name+" / No keeps fields", func(t *testing.T) {
			sm, host := newSendMessageFixture(t)
			tt.setup(sm)
			before := sendMessageFields(sm)
			sm.openSnippetPicker()
			pickerSelect(t, sm.snippetPicker, "created.json")

			if !sm.confirm.Visible() {
				t.Fatal("no confirmation for a filled form")
			}
			selectConfirmItem(sm.confirm, 0)

			if got := sendMessageFields(sm); got != before {
				t.Errorf("fields = %q, want unchanged %q", got, before)
			}
			if host.focused != sm.form || host.focusMainCalls != 0 {
				t.Errorf("focus = %v, FocusMain calls = %d; want the form, 0", host.focused, host.focusMainCalls)
			}
		})
		t.Run(tt.name+" / Yes replaces", func(t *testing.T) {
			sm, host := newSendMessageFixture(t)
			tt.setup(sm)
			sm.openSnippetPicker()
			pickerSelect(t, sm.snippetPicker, "plain.txt")
			selectConfirmItem(sm.confirm, 1)

			if got := sm.jmsTypeItem.GetText(); got != "" {
				t.Errorf("JMS Type = %q, want cleared (snippet has none)", got)
			}
			if got := sm.bodyItem.GetText(); got != "just a body" {
				t.Errorf("Body = %q, want %q", got, "just a body")
			}
			if host.focused != sm.form {
				t.Error("focus not returned to the form")
			}
		})
	}
}

func TestSendMessagePickerEscReturnsToForm(t *testing.T) {
	sm, host := newSendMessageFixture(t)
	sm.jmsTypeItem.SetText("Mine")
	before := sendMessageFields(sm)
	sm.openSnippetPicker()
	pickerKey(sm.snippetPicker, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if got := sendMessageFields(sm); got != before {
		t.Errorf("fields = %q, want unchanged %q", got, before)
	}
	if host.focused != sm.form || !sm.Visible() {
		t.Error("Esc in the picker didn't return focus to the still-open form")
	}
}

// TestSendMessageLoadSnippetButtonFitsOverlay renders the form at the
// size app.go gives the send-message overlay (90×26), so a third button
// that no longer fits on the button row would be caught here.
func TestSendMessageLoadSnippetButtonFitsOverlay(t *testing.T) {
	sm, _ := newSendMessageFixture(t)
	text := renderedScreenText(t, sm.Primitive(), 90, 26)
	for _, label := range []string{"Submit", "Cancel", "Load snippet…"} {
		if !strings.Contains(text, label) {
			t.Errorf("rendered overlay is missing the %q button", label)
		}
	}
}
