package snippet

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

// FormatBody pretty-prints valid JSON or XML bodies with two-space
// indentation. JSON takes precedence when a body is valid as both. Bodies
// that are neither format, or XML whose mixed text content could be
// changed by pretty-printing, are returned unchanged.
func FormatBody(body string) string {
	if json.Valid([]byte(body)) {
		var formatted bytes.Buffer
		if err := json.Indent(&formatted, []byte(body), "", "  "); err == nil {
			return formatted.String()
		}
	}

	formatted, ok := formatXML(body)
	if !ok {
		return body
	}
	return formatted
}

type xmlNode struct {
	token    xml.Token
	children []*xmlNode
}

// formatXML parses a complete XML document and re-encodes its tokens with
// indentation. It leaves mixed-content documents alone: inserted whitespace
// between text and child elements could otherwise change their value.
func formatXML(body string) (string, bool) {
	// Token validates XML names, namespace bindings, and matching tags. A
	// second RawToken pass below preserves the prefixes used by the source.
	validator := xml.NewDecoder(strings.NewReader(body))
	for {
		if _, err := validator.Token(); err == io.EOF {
			break
		} else if err != nil {
			return "", false
		}
	}

	decoder := xml.NewDecoder(strings.NewReader(body))
	var roots []*xmlNode
	var stack []*xmlNode
	var elementNames []string
	rootElements := 0

	for {
		token, err := decoder.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false
		}
		token = cloneXMLToken(token)
		node := &xmlNode{token: token}
		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, node)
		} else {
			roots = append(roots, node)
		}

		switch token.(type) {
		case xml.StartElement:
			if len(stack) == 0 {
				rootElements++
			}
			stack = append(stack, node)
			start := token.(xml.StartElement)
			elementNames = append(elementNames, start.Name.Local)
		case xml.EndElement:
			end := token.(xml.EndElement)
			if len(stack) == 0 || len(elementNames) == 0 || elementNames[len(elementNames)-1] != end.Name.Local {
				return "", false
			}
			stack = stack[:len(stack)-1]
			elementNames = elementNames[:len(elementNames)-1]
		}
	}
	if len(stack) != 0 || rootElements != 1 {
		return "", false
	}

	if hasMixedContent(roots) {
		return "", false
	}

	var out bytes.Buffer
	for i, root := range roots {
		if i > 0 {
			out.WriteByte('\n')
		}
		if err := writeXMLNode(&out, root, 0); err != nil {
			return "", false
		}
	}
	return out.String(), true
}

func cloneXMLToken(token xml.Token) xml.Token {
	switch token := token.(type) {
	case xml.StartElement:
		attrs := append([]xml.Attr(nil), token.Attr...)
		return xml.StartElement{Name: token.Name, Attr: attrs}
	case xml.EndElement:
		return token
	case xml.CharData:
		return append(xml.CharData(nil), token...)
	case xml.Comment:
		return append(xml.Comment(nil), token...)
	case xml.Directive:
		return append(xml.Directive(nil), token...)
	case xml.ProcInst:
		return xml.ProcInst{Target: token.Target, Inst: append([]byte(nil), token.Inst...)}
	default:
		return token
	}
}

// hasMixedContent reports whether any element contains both non-whitespace
// character data and child elements, for which inserted indentation is data.
func hasMixedContent(nodes []*xmlNode) bool {
	for _, node := range nodes {
		if _, isElement := node.token.(xml.StartElement); isElement {
			hasText, hasElement := false, false
			for _, child := range node.children {
				switch token := child.token.(type) {
				case xml.CharData:
					if strings.TrimSpace(string(token)) != "" {
						hasText = true
					}
				case xml.StartElement:
					hasElement = true
				}
			}
			if hasText && hasElement {
				return true
			}
		}
		if hasMixedContent(node.children) {
			return true
		}
	}
	return false
}

func writeXMLNode(out *bytes.Buffer, node *xmlNode, depth int) error {
	switch token := node.token.(type) {
	case xml.StartElement:
		out.WriteByte('<')
		out.WriteString(xmlName(token.Name))
		for _, attr := range token.Attr {
			out.WriteByte(' ')
			out.WriteString(xmlName(attr.Name))
			out.WriteString(`="`)
			var escaped bytes.Buffer
			if err := xml.EscapeText(&escaped, []byte(attr.Value)); err != nil {
				return err
			}
			out.WriteString(strings.ReplaceAll(escaped.String(), `"`, `&#34;`))
			out.WriteByte('"')
		}
		out.WriteByte('>')
		prettyChildren := hasElementChild(node.children)
		for _, child := range node.children {
			if _, isEnd := child.token.(xml.EndElement); isEnd {
				continue
			}
			if text, isText := child.token.(xml.CharData); isText && prettyChildren && strings.TrimSpace(string(text)) == "" {
				continue
			}
			if prettyChildren {
				out.WriteByte('\n')
				writeXMLIndent(out, depth+1)
			}
			if err := writeXMLNode(out, child, depth+1); err != nil {
				return err
			}
		}
		if prettyChildren {
			out.WriteByte('\n')
			writeXMLIndent(out, depth)
		}
		out.WriteString("</")
		out.WriteString(xmlName(token.Name))
		out.WriteByte('>')
	case xml.EndElement:
		return nil // End tokens are emitted by their matching StartElement.
	case xml.CharData:
		return xml.EscapeText(out, token)
	case xml.Comment:
		out.WriteString("<!--")
		out.Write(token)
		out.WriteString("-->")
	case xml.Directive:
		out.WriteString("<!")
		out.Write(token)
		out.WriteByte('>')
	case xml.ProcInst:
		out.WriteString("<?")
		out.WriteString(token.Target)
		if len(token.Inst) > 0 {
			out.WriteByte(' ')
			out.Write(token.Inst)
		}
		out.WriteString("?>")
	default:
		return strconv.ErrSyntax
	}
	return nil
}

func hasElementChild(nodes []*xmlNode) bool {
	for _, node := range nodes {
		if _, ok := node.token.(xml.StartElement); ok {
			return true
		}
	}
	return false
}

func xmlName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return name.Space + ":" + name.Local
}

func writeXMLIndent(out *bytes.Buffer, depth int) {
	for range depth * 2 {
		out.WriteByte(' ')
	}
}
