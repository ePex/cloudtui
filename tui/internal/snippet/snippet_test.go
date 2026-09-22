package snippet

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		data string
		want Snippet
	}{
		{
			name: "no front matter",
			data: `{"orderId": 42}`,
			want: Snippet{Body: `{"orderId": 42}`},
		},
		{
			name: "front matter",
			data: "---\njmsType: OrderCreated\n---\n{\"orderId\": 42}\n",
			want: Snippet{JMSType: "OrderCreated", Body: "{\"orderId\": 42}\n"},
		},
		{
			name: "CRLF delimiters keep body bytes",
			data: "---\r\njmsType: OrderCreated\r\n---\r\nline one\r\nline two",
			want: Snippet{JMSType: "OrderCreated", Body: "line one\r\nline two"},
		},
		{
			name: "body containing delimiter lines",
			data: "---\njmsType: Doc\n---\na\n---\nb\n",
			want: Snippet{JMSType: "Doc", Body: "a\n---\nb\n"},
		},
		{
			name: "unknown keys kept in Extra",
			data: "---\njmsType: T\nauthor: someone\n---\nbody",
			want: Snippet{JMSType: "T", Body: "body", Extra: "jmsType: T\nauthor: someone\n"},
		},
		{
			name: "comment kept in Extra",
			data: "---\n# team snippet\njmsType: T\n---\nbody",
			want: Snippet{JMSType: "T", Body: "body", Extra: "# team snippet\njmsType: T\n"},
		},
		{
			name: "only unknown keys, no jmsType",
			data: "---\nauthor: someone\n---\nbody",
			want: Snippet{Body: "body", Extra: "author: someone\n"},
		},
		{
			name: "lone jmsType has no Extra even when quoted",
			data: "---\njmsType: \"T\"\n---\nbody",
			want: Snippet{JMSType: "T", Body: "body"},
		},
		{
			name: "null front matter",
			data: "---\n~\n---\nbody",
			want: Snippet{Body: "body"},
		},
		{
			name: "empty front matter",
			data: "---\n---\nbody",
			want: Snippet{Body: "body"},
		},
		{
			name: "empty body",
			data: "---\njmsType: T\n---\n",
			want: Snippet{JMSType: "T"},
		},
		{
			name: "closing delimiter at end of file without newline",
			data: "---\njmsType: T\n---",
			want: Snippet{JMSType: "T"},
		},
		{
			name: "delimiter not on the first line is body",
			data: "hello\n---\njmsType: T\n---\n",
			want: Snippet{Body: "hello\n---\njmsType: T\n---\n"},
		},
		{
			name: "first line with trailing text is not a delimiter",
			data: "--- \njmsType: T\n---\n",
			want: Snippet{Body: "--- \njmsType: T\n---\n"},
		},
		{
			name: "empty file",
			data: "",
			want: Snippet{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.data))
			if err != nil {
				t.Fatalf("Parse: unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Parse = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{
			name:    "missing closing delimiter",
			data:    "---\njmsType: T\nbody",
			wantErr: "no closing",
		},
		{
			name:    "invalid YAML",
			data:    "---\njmsType: [unclosed\n---\nbody",
			wantErr: "front matter",
		},
		{
			name:    "jmsType is not a string",
			data:    "---\njmsType:\n  nested: map\n---\nbody",
			wantErr: "front matter",
		},
		{
			name:    "front matter is a list, not a mapping",
			data:    "---\n- a\n- b\n---\nbody",
			wantErr: "mapping",
		},
		{
			name:    "front matter is a plain value",
			data:    "---\njust text\n---\nbody",
			wantErr: "mapping",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.data))
			if err == nil {
				t.Fatal("Parse: expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Parse error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		in   Snippet
		want string
	}{
		{
			name: "no JMS type writes body only",
			in:   Snippet{Body: `{"a":1}`},
			want: `{"a":1}`,
		},
		{
			name: "JMS type writes front matter",
			in:   Snippet{JMSType: "OrderCreated", Body: "body"},
			want: "---\njmsType: OrderCreated\n---\nbody",
		},
		{
			name: "no trailing newline added",
			in:   Snippet{JMSType: "T", Body: "x"},
			want: "---\njmsType: T\n---\nx",
		},
		{
			name: "body starting with delimiter gets empty front matter",
			in:   Snippet{Body: "---\nkey: value\n"},
			want: "---\n---\n---\nkey: value\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Format(tt.in)); got != tt.want {
				t.Errorf("Format = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	tests := []Snippet{
		{},
		{Body: "plain text"},
		{JMSType: "OrderCreated", Body: "{\n  \"orderId\": 42\n}\n"},
		{JMSType: "yes", Body: "YAML would read an unquoted yes as a bool"},
		{JMSType: "#hash", Body: "a leading # would start a comment"},
		{JMSType: "a: b", Body: "colon in the type"},
		{JMSType: "T", Body: "a\n---\nb"},
		{Body: "---\nlooks like front matter\n---\n"},
		{Body: "---"},
		{JMSType: "T", Body: "crlf\r\nbody\r\n"},
	}
	for _, want := range tests {
		got, err := Parse(Format(want))
		if err != nil {
			t.Errorf("Parse(Format(%#v)): unexpected error: %v", want, err)
			continue
		}
		if got != want {
			t.Errorf("Parse(Format(%#v)) = %#v", want, got)
		}
	}
}

// TestFormatKeepsUnknownKeysAndComments covers editing a snippet whose
// front matter the app only partly understands: whatever happens to
// jmsType, every other key, its value, the key order, and comments
// (including those attached to jmsType itself) survive.
func TestFormatKeepsUnknownKeysAndComments(t *testing.T) {
	const file = "---\n" +
		"# Shared by the payments team.\n" +
		"\n" +
		"author: someone # who to ask\n" +
		"# The broker-side type.\n" +
		"jmsType: OrderCreated\n" +
		"tags: [orders, eu]\n" +
		"---\n" +
		"{}"
	parsed, err := Parse([]byte(file))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	tests := []struct {
		name        string
		jmsType     string
		wantType    string
		wantLines   []string // must all appear in the written front matter
		absentLines []string // must not appear
	}{
		{
			name:     "unchanged",
			jmsType:  "OrderCreated",
			wantType: "OrderCreated",
			wantLines: []string{"# Shared by the payments team.", "author: someone # who to ask",
				"# The broker-side type.", "jmsType: OrderCreated", "tags: [orders, eu]"},
		},
		{
			name:     "changed keeps position and comment",
			jmsType:  "OrderUpdated",
			wantType: "OrderUpdated",
			wantLines: []string{"author: someone # who to ask", "# The broker-side type.",
				"jmsType: OrderUpdated", "tags: [orders, eu]"},
			absentLines: []string{"jmsType: OrderCreated"},
		},
		{
			name:        "cleared removes only jmsType",
			jmsType:     "",
			wantType:    "",
			wantLines:   []string{"# Shared by the payments team.", "author: someone # who to ask", "tags: [orders, eu]"},
			absentLines: []string{"jmsType"},
		},
		{
			name:      "needs quoting",
			jmsType:   "yes",
			wantType:  "yes",
			wantLines: []string{"author: someone # who to ask", "tags: [orders, eu]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edited := parsed
			edited.JMSType = tt.jmsType
			out := string(Format(edited))
			for _, line := range tt.wantLines {
				if !strings.Contains(out, line+"\n") {
					t.Errorf("written file lacks %q:\n%s", line, out)
				}
			}
			for _, line := range tt.absentLines {
				if strings.Contains(out, line) {
					t.Errorf("written file still contains %q:\n%s", line, out)
				}
			}
			if !strings.HasSuffix(out, "---\n{}") {
				t.Errorf("body not written verbatim after the front matter:\n%s", out)
			}
			// Key order is kept: author before tags (and jmsType, when
			// present, in between).
			if a, g := strings.Index(out, "author:"), strings.Index(out, "tags:"); a < 0 || g < 0 || a > g {
				t.Errorf("key order changed:\n%s", out)
			}

			reparsed, err := Parse([]byte(out))
			if err != nil {
				t.Fatalf("Parse(Format(...)): %v", err)
			}
			if reparsed.JMSType != tt.wantType || reparsed.Body != "{}" {
				t.Errorf("reparsed = (JMSType %q, Body %q), want (%q, %q)", reparsed.JMSType, reparsed.Body, tt.wantType, "{}")
			}
		})
	}
}

// TestFormatAddsJMSTypeToExtra covers setting a JMS Type on a snippet
// whose front matter had none: it's added as the first key, the rest
// stays.
func TestFormatAddsJMSTypeToExtra(t *testing.T) {
	parsed, err := Parse([]byte("---\nauthor: someone\n---\nbody"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	parsed.JMSType = "T"
	if got, want := string(Format(parsed)), "---\njmsType: T\nauthor: someone\n---\nbody"; got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
}

// TestParseFormatIsStable checks that writing back what Parse returned
// reproduces the same Snippet — Extra is already normalized, so a save
// without edits never keeps rewriting the file differently.
func TestParseFormatIsStable(t *testing.T) {
	files := []string{
		"---\njmsType: T\n---\nbody",
		"---\n# c\njmsType: T\nauthor: x\n---\nbody",
		"---\n# doc comment\n\nauthor:   spaced\nnested:\n    deep: 1\njmsType: 'T'\n# trailing\n---\nbody",
		"---\nauthor: x\n---\n---\nbody starting with a delimiter",
		"---\n---\nbody",
	}
	for _, f := range files {
		first, err := Parse([]byte(f))
		if err != nil {
			t.Fatalf("Parse(%q): %v", f, err)
		}
		second, err := Parse(Format(first))
		if err != nil {
			t.Fatalf("Parse(Format(Parse(%q))): %v", f, err)
		}
		if second != first {
			t.Errorf("not stable for %q:\n first  %#v\n second %#v", f, first, second)
		}
	}
}
