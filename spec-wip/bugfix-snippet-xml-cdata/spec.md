# Bugfix: snippet XML formatting rewrites CDATA sections

_Date: 2026-09-23_

## What

When the snippet library formats an XML body (spec/22: on save, and
before a snippet opens in the editor), CDATA sections are written back
as escaped text:

```xml
<payload><![CDATA[<order id="1">x < y</order>]]></payload>
```

becomes

```xml
<payload>&lt;order id=&#34;1&#34;&gt;x &lt; y&lt;/order&gt;</payload>
```

That's the same value to an XML parser, but a different message
on the wire. It's also much harder to read, because the whole point of
CDATA is to embed text (often another XML or JSON document) without
escaping it.

## Why

Snippets are messages you send, so their text should stay what you
wrote, apart from harmless indentation. A consumer that compares or
logs the raw payload would see the formatted snippet differ from the
original. Embedding XML in CDATA is common (e.g. SOAP-style envelopes
carrying a document), and formatting currently turns those payloads into
walls of `&lt;` and `&gt;`.

## Scope

- **CDATA sections are kept exactly as written:** the `<![CDATA[` …
  `]]>` markers and everything between them, byte for byte, including
  whitespace and line breaks inside.
- **The rest of the document is still formatted as today.** Elements
  are indented with two spaces, and an element whose only content is a
  CDATA section stays on one line, like an element with plain text:

  ```xml
  <order>
    <payload><![CDATA[<inner a="1">x < y</inner>]]></payload>
  </order>
  ```
- **Mixed content counts CDATA as text.** An element holding both a
  CDATA section (or text) and child elements is mixed content, so the
  document is left unchanged, as today for text.
- **Everything else is unchanged:**
  - JSON formatting
  - which XML is left alone (mixed content, `xml:space="preserve"`,
    malformed XML, several root elements)
  - trailing-newline handling
- **A safe fallback:** if the formatter can't tell a text node's exact
  source, it leaves the whole XML body unformatted rather than risk
  rewriting a CDATA section.
- `spec/22`'s File format section changes its "CDATA sections are
  written as escaped text" note (added by #37) to say they're kept.

## Out of scope

- Other ways formatting re-serializes XML: `<b/>` → `<b></b>`, attribute
  quote style, and entity forms such as `&#34;` vs `&quot;` in ordinary
  text. These keep the meaning and aren't part of this fix; they could
  be a follow-up if they turn out to matter.
- Comments and processing instructions: they're already kept as written.
- Formatting of JSON strings that contain XML.
