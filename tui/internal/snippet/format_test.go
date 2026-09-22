package snippet

import "testing"

func TestFormatBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "compact JSON",
			body: `{"orderId":42,"items":[{"sku":"A-1","quantity":2}]}`,
			want: "{\n  \"orderId\": 42,\n  \"items\": [\n    {\n      \"sku\": \"A-1\",\n      \"quantity\": 2\n    }\n  ]\n}",
		},
		{
			name: "already formatted JSON",
			body: "{\n  \"ok\": true\n}",
			want: "{\n  \"ok\": true\n}",
		},
		{
			name: "compact XML",
			body: `<order id="42"><item sku="A-1">Book</item><item sku="B-2">Pen</item></order>`,
			want: "<order id=\"42\">\n  <item sku=\"A-1\">Book</item>\n  <item sku=\"B-2\">Pen</item>\n</order>",
		},
		{
			name: "XML declaration, comments, and namespace",
			body: `<?xml version="1.0"?><o:order xmlns:o="urn:orders"><!-- kept --><o:id>42</o:id></o:order>`,
			want: "<?xml version=\"1.0\"?>\n<o:order xmlns:o=\"urn:orders\">\n  <!-- kept -->\n  <o:id>42</o:id>\n</o:order>",
		},
		{
			name: "mixed XML text stays unchanged",
			body: `<p>Hello <b>there</b> friend</p>`,
			want: `<p>Hello <b>there</b> friend</p>`,
		},
		{
			name: "XML trailing newline is preserved",
			body: `<root><item/></root>` + "\n",
			want: "<root>\n  <item></item>\n</root>\n",
		},
		{
			name: "xml space preserve stays unchanged",
			body: `<root xml:space="preserve"><item/>  </root>`,
			want: `<root xml:space="preserve"><item/>  </root>`,
		},
		{
			name: "invalid JSON and XML",
			body: `{"unfinished": [1, 2]`,
			want: `{"unfinished": [1, 2]`,
		},
		{
			name: "plain text",
			body: "hello\nworld",
			want: "hello\nworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBody(tt.body); got != tt.want {
				t.Errorf("FormatBody() = %q, want %q", got, tt.want)
			}
		})
	}

	for _, body := range []string{`{"id":1}`, `<root><item/></root>`, `<p>Hello <b>there</b> friend</p>`} {
		if once, twice := FormatBody(body), FormatBody(FormatBody(body)); once != twice {
			t.Errorf("FormatBody is not idempotent for %q: once %q, twice %q", body, once, twice)
		}
	}
}
