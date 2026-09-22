# Plan: keep CDATA sections when formatting snippet XML

## The cause

`formatXML` (`internal/snippet/format.go`) builds a node tree from
`xml.Decoder.RawToken()` and writes it back with `writeXMLNode`.
`encoding/xml` returns a CDATA section as a plain `xml.CharData` token,
the same type as ordinary text, with its content already unwrapped. So
`writeXMLNode` escapes it like any text (`xml.EscapeText`), and the
`<![CDATA[ … ]]>` markers are lost.

## Approach: remember each CDATA section's source text

The decoder can't tell CDATA from text, but its input position can.
`Decoder.InputOffset()` is the byte offset just past the last returned
token. Reading it before and after each `RawToken()` gives that token's
exact source span in `body`.

- **In the second (`RawToken`) pass,** take `start := decoder.InputOffset()`
  before each `RawToken()` and `end := decoder.InputOffset()` after it.
  For a `CharData` token, look at `body[start:end]`:
  - If it starts with `<![CDATA[` and ends with `]]>`, it's a CDATA
    section. `encoding/xml` returns each CDATA section as its own token,
    separate from surrounding text. The node keeps that span verbatim in
    a new field, `xmlNode.raw`.
  - Otherwise it's ordinary text, handled as today.
- **Fallback (spec):** if an offset is out of range, or goes backwards
  (`start > end`, `end > len(body)`), `formatXML` returns `("", false)`,
  so the body stays unformatted rather than risking a rewritten CDATA
  section.

## Using the kept source

- **`writeXMLNode`:** a node with `raw` set is written as `raw`,
  untouched.
- **A CDATA node is never whitespace.** Today, whitespace-only
  `CharData` is:
  - ignored by `hasMixedContent`
  - dropped between child elements by `writeXMLNode`
  - treated specially at the top level by `isXMLWhitespace`

  A CDATA section containing only spaces or line breaks is still
  content, so all three treat a node with `raw` set as non-whitespace
  text. Two consequences:
  - An element with a CDATA section **and** child elements counts as
    mixed content, and the document is left unchanged (spec).
  - An element whose only content is CDATA has no element child, so
    `writeXMLNode` keeps it on one line (spec), exactly as today for
    plain text.
- `isXMLWhitespace` takes the node rather than the token, since the
  `raw` field lives on the node. It's only called from `formatXML` for
  top-level nodes.

## Files

- `tui/internal/snippet/format.go`:
  - `xmlNode.raw`, and the offset capture in `formatXML`
  - the three whitespace/mixed-content checks
  - writing `raw` in `writeXMLNode`
- `tui/internal/snippet/format_test.go`: new cases (below).
- `spec/22-message-snippets/spec.md` (merge-back): the File format note
  changes from "CDATA sections are written as escaped text" to "CDATA
  sections are kept exactly as written"; the `<b/>` and quote-style
  notes stay.

No new dependencies.

## Testing (`format_test.go`, table-driven)

- **Kept byte for byte, with the rest indented:**
  - a CDATA section holding an XML document (with `<`, `>`, `&` and
    quotes) in a nested element
  - line breaks and indentation inside a CDATA section
  - a whitespace-only CDATA section
  - two CDATA sections in one element, and CDATA next to plain text in
    an element without child elements
- **Left unchanged:** an element holding a CDATA section **and** a child
  element (mixed content), and a whitespace-only CDATA section next to a
  child element.
- **Stable:** formatting an already-formatted CDATA document again
  changes nothing.
- **No regressions:** all existing `FormatBody` cases pass unchanged, in
  particular plain text inside elements is still escaped as before.
- **Mutation checks:**
  - drop the `raw` write (CDATA escaped again)
  - treat whitespace-only CDATA as whitespace (it gets dropped)
- **The fallback can't be tested.** For input the validator accepts,
  `Decoder.InputOffset()` is always consistent, so no input can trigger
  it. Its mutation check confirmed that: removing it fails no test. It's
  kept as a defensive guard, with a comment in the code saying so.
- **Live check:** save and edit a snippet with an embedded CDATA
  payload in the Snippets view, preview it, and send it through **Load
  snippet…**. The message on the queue keeps its CDATA section.
