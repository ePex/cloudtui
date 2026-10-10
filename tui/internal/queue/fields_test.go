package queue

import "testing"

func TestDecodePropertyValue(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"ByteSequence decodes to UTF-8", map[string]any{"data": []any{float64('h'), float64('i')}}, "hi"},
		{"plain string", "plain", "plain"},
		{"number", float64(42), "42"},
		{"map without data", map[string]any{"other": 1}, "map[other:1]"},
		{"data that isn't a list", map[string]any{"data": "x"}, "map[data:x]"},
		{"data with a non-number", map[string]any{"data": []any{"x"}}, "map[data:[x]]"},
	}
	for _, tt := range tests {
		if got := DecodePropertyValue(tt.in); got != tt.want {
			t.Errorf("%s: DecodePropertyValue() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestMessageBody(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]any
		wantBody string
		wantOK   bool
	}{
		{"text body", map[string]any{"text": `{"id":1}`}, `{"id":1}`, true},
		{"empty text", map[string]any{"text": ""}, "", false},
		{"no text field (binary)", map[string]any{"properties": map[string]any{}}, "", false},
		{"no raw fields", nil, "", false},
	}
	for _, tt := range tests {
		body, ok := Message{RawFields: tt.raw, Preview: "trimmed"}.Body()
		if body != tt.wantBody || ok != tt.wantOK {
			t.Errorf("%s: Body() = %q, %v; want %q, %v", tt.name, body, ok, tt.wantBody, tt.wantOK)
		}
	}
}

func TestHeaderFields(t *testing.T) {
	fields := HeaderFields()
	seen := map[string]bool{}
	for _, f := range fields {
		if f.Label == "" || f.Key == "" || seen[f.Label] {
			t.Errorf("bad or duplicate field %+v", f)
		}
		seen[f.Label] = true
	}
	if last := fields[len(fields)-1]; last.Key != "properties" {
		t.Errorf("last field = %+v, want the properties map last (the detail view renders it as a block)", last)
	}
}
