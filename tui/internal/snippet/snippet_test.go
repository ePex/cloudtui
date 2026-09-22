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
			name: "unknown keys ignored",
			data: "---\njmsType: T\nauthor: someone\n---\nbody",
			want: Snippet{JMSType: "T", Body: "body"},
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
