// Package snippet reads and writes reusable message snippets: plain files
// under ~/.cloudtui/snippets/ holding an optional YAML front-matter block
// (only jmsType today) followed by the raw message body.
package snippet

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// delimiter is the line that opens and closes a snippet's front matter.
const delimiter = "---"

// Snippet is a reusable message: its JMS Type (empty = none) and body.
type Snippet struct {
	JMSType string
	Body    string
}

// frontMatter is the YAML shape between the delimiters. Unknown keys are
// ignored on decode, so later versions can add keys.
type frontMatter struct {
	JMSType string `yaml:"jmsType,omitempty"`
}

// Parse decodes a snippet file. Front matter is recognized only when the
// very first line is exactly "---"; it ends at the next line that is
// exactly "---", and every byte after that line is the body. Delimiter
// lines may end in "\n" or "\r\n". Without front matter, all of data is
// the body.
func Parse(data []byte) (Snippet, error) {
	first, rest, ok := cutLine(data)
	if !ok || !isDelimiter(first) {
		return Snippet{Body: string(data)}, nil
	}

	var yamlPart []byte
	remaining := rest
	for {
		line, next, ok := cutLine(remaining)
		if isDelimiter(line) {
			var fm frontMatter
			if err := yaml.Unmarshal(yamlPart, &fm); err != nil {
				return Snippet{}, fmt.Errorf("parsing snippet front matter: %w", err)
			}
			return Snippet{JMSType: fm.JMSType, Body: string(next)}, nil
		}
		if !ok {
			return Snippet{}, errors.New("parsing snippet: front matter has no closing \"---\" line")
		}
		yamlPart = append(yamlPart, line...)
		yamlPart = append(yamlPart, '\n')
		remaining = next
	}
}

// Format encodes s as a snippet file. Front matter is written when s has
// a JMS Type — and also, empty, when the body's own first line is "---",
// so Parse doesn't mistake the body for front matter on the way back in.
// The body is written byte for byte.
func Format(s Snippet) []byte {
	first, _, _ := cutLine([]byte(s.Body))
	if s.JMSType == "" && !isDelimiter(first) {
		return []byte(s.Body)
	}

	var b bytes.Buffer
	b.WriteString(delimiter + "\n")
	if s.JMSType != "" {
		// Marshaling a struct with a single string field cannot fail.
		out, _ := yaml.Marshal(frontMatter{JMSType: s.JMSType})
		b.Write(out)
	}
	b.WriteString(delimiter + "\n")
	b.WriteString(s.Body)
	return b.Bytes()
}

// cutLine splits data at its first "\n", returning the line (without its
// "\n" or a trailing "\r") and everything after it. ok is false when data
// has no "\n", in which case line is all of data and rest is empty.
func cutLine(data []byte) (line, rest []byte, ok bool) {
	line, rest, ok = bytes.Cut(data, []byte("\n"))
	if ok {
		line = bytes.TrimSuffix(line, []byte("\r"))
	}
	return line, rest, ok
}

// isDelimiter reports whether line is exactly the front-matter delimiter.
func isDelimiter(line []byte) bool {
	return string(line) == delimiter
}
