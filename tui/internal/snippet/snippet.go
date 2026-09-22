// Package snippet reads and writes reusable message snippets: plain files
// under ~/.cloudtui/snippets/ holding an optional YAML front-matter block
// (jmsType, plus any keys the app doesn't know, which are kept) followed
// by the raw message body.
package snippet

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// delimiter is the line that opens and closes a snippet's front matter.
const delimiter = "---"

// jmsTypeKey is the one front-matter key the app reads and writes.
const jmsTypeKey = "jmsType"

// Snippet is a reusable message: its JMS Type (empty = none) and body.
type Snippet struct {
	JMSType string
	Body    string
	// Extra is the whole front matter, as normalized YAML, whenever it
	// holds more than a lone jmsType — other keys or comments — and ""
	// otherwise. Format uses it as a template and only sets, adds, or
	// removes jmsType in it, so keys and comments the app doesn't know
	// (e.g. an author: a teammate added) survive an edit.
	Extra string
}

// frontMatter is the canonical shape of a front matter that holds only
// jmsType.
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
			jmsType, extra, err := parseFrontMatter(yamlPart)
			if err != nil {
				return Snippet{}, fmt.Errorf("parsing snippet front matter: %w", err)
			}
			return Snippet{JMSType: jmsType, Body: string(next), Extra: extra}, nil
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
// a JMS Type or Extra — and also, empty, when the body's own first line
// is "---", so Parse doesn't mistake the body for front matter on the way
// back in. The body is written byte for byte.
func Format(s Snippet) []byte {
	fm := frontMatterText(s)
	first, _, _ := cutLine([]byte(s.Body))
	if fm == "" && !isDelimiter(first) {
		return []byte(s.Body)
	}

	var b bytes.Buffer
	b.WriteString(delimiter + "\n")
	b.WriteString(fm)
	b.WriteString(delimiter + "\n")
	b.WriteString(s.Body)
	return b.Bytes()
}

// parseFrontMatter reads jmsType from the front matter's YAML and returns
// the whole front matter as normalized YAML in extra — or "" when it
// holds nothing but a lone jmsType (or nothing at all). The front matter
// must be a mapping (or empty), and jmsType must be a scalar.
func parseFrontMatter(yamlPart []byte) (jmsType, extra string, err error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(yamlPart, &doc); err != nil {
		return "", "", err
	}
	root, err := mappingRoot(&doc, false)
	if err != nil {
		return "", "", err
	}
	if root != nil {
		if i := keyIndex(root, jmsTypeKey); i >= 0 {
			if err := root.Content[i+1].Decode(&jmsType); err != nil {
				return "", "", err
			}
		}
	}
	if isPlainFrontMatter(&doc, root) {
		return jmsType, "", nil
	}
	extra, err = encodeFrontMatter(&doc)
	if err != nil {
		return "", "", err
	}
	return jmsType, extra, nil
}

// isPlainFrontMatter reports whether doc holds nothing Format wouldn't
// reproduce on its own: no comments anywhere, and either no keys or just
// a jmsType pair (however it's quoted).
func isPlainFrontMatter(doc, root *yaml.Node) bool {
	if hasComments(doc) {
		return false
	}
	if root == nil {
		return len(doc.Content) == 0 || !hasComments(doc.Content[0])
	}
	if hasComments(root) {
		return false
	}
	switch len(root.Content) {
	case 0:
		return true
	case 2:
		return root.Content[0].Value == jmsTypeKey && !hasComments(root.Content[0]) && !hasComments(root.Content[1])
	default:
		return false
	}
}

// hasComments reports whether n itself carries a comment.
func hasComments(n *yaml.Node) bool {
	return n.HeadComment != "" || n.LineComment != "" || n.FootComment != ""
}

// frontMatterText returns the YAML to write between s's delimiters ("" for
// none): s.Extra with jmsType set to s.JMSType — added at the top if
// missing, removed if s.JMSType is empty — or, without Extra, just the
// jmsType line. Extra always comes from Parse, so it's valid YAML; should
// it not be, it's ignored rather than written out broken.
func frontMatterText(s Snippet) string {
	if s.Extra == "" {
		return plainFrontMatter(s.JMSType)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(s.Extra), &doc); err != nil {
		return plainFrontMatter(s.JMSType)
	}
	root, err := mappingRoot(&doc, true)
	if err != nil {
		return plainFrontMatter(s.JMSType)
	}
	setJMSType(root, s.JMSType)
	text, err := encodeFrontMatter(&doc)
	if err != nil {
		return plainFrontMatter(s.JMSType)
	}
	return text
}

// plainFrontMatter is the front matter for a lone jmsType ("" for none).
func plainFrontMatter(jmsType string) string {
	if jmsType == "" {
		return ""
	}
	// Marshaling a struct with a single string field cannot fail; it
	// also quotes values YAML would otherwise misread ("yes", "#x", ...).
	out, _ := yaml.Marshal(frontMatter{JMSType: jmsType})
	return string(out)
}

// mappingRoot returns doc's top-level mapping. An empty document (or a
// null one) has none: mappingRoot returns nil, or — with create — adds an
// empty mapping and returns that. Any other kind of root is an error.
func mappingRoot(doc *yaml.Node, create bool) (*yaml.Node, error) {
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
	}
	if len(doc.Content) == 0 || doc.Content[0].Tag == "!!null" {
		if !create {
			return nil, nil
		}
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("front matter must be a mapping of keys to values")
	}
	return root, nil
}

// keyIndex returns the index of key's key node in mapping's Content
// (its value is at index+1), or -1.
func keyIndex(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// setJMSType sets jmsType in mapping to value, keeping the pair's position
// and comments; adds it as the first pair if missing, and removes it
// (comments included) when value is empty.
func setJMSType(mapping *yaml.Node, value string) {
	i := keyIndex(mapping, jmsTypeKey)
	switch {
	case value == "" && i >= 0:
		mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
	case value == "":
	case i >= 0:
		v := mapping.Content[i+1]
		v.Kind, v.Tag, v.Value = yaml.ScalarNode, "!!str", value
		if v.Style != yaml.SingleQuotedStyle && v.Style != yaml.DoubleQuotedStyle {
			v.Style = 0
		}
	default:
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: jmsTypeKey}
		val := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
		mapping.Content = append([]*yaml.Node{key, val}, mapping.Content...)
	}
}

// encodeFrontMatter encodes doc as YAML with a two-space indent; an empty
// document or mapping encodes as "".
func encodeFrontMatter(doc *yaml.Node) (string, error) {
	if len(doc.Content) == 0 && doc.HeadComment == "" && doc.FootComment == "" {
		return "", nil
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	if out := b.String(); out != "{}\n" && out != "null\n" {
		return out, nil
	}
	return "", nil
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
