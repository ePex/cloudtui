# Format JSON and XML snippet bodies

Date: 2026-09-22

## Summary

When a snippet body contains valid JSON or XML, cloudtui formats it for readability. The formatted body is shown in the snippet editor and written to disk when the snippet is opened for editing, and formatted bodies are stored whenever snippets are saved. Other text and invalid JSON/XML remain unchanged.

## Motivation

Compact JSON/XML is common in broker messages and hard to read or edit in the snippet library. Formatting on editor open makes existing compact snippets readable immediately, while normalizing the file on disk keeps the shared snippet library readable outside cloudtui too.

## Scope

- Format valid JSON bodies with stable indentation and preserve their JSON values.
- Format valid XML bodies with stable indentation while preserving their XML content and structure.
- Apply formatting when opening a snippet in the library editor and write the normalized content back to the same file as part of that open operation.
- Apply the same formatting before writing snippets from the editor, the Save as Snippet action, and any other snippet save path.
- Format only the body; preserve JMS Type and all unknown front-matter keys/comments according to the existing snippet file-format behavior.
- Leave plain text and bodies that are not valid JSON/XML unchanged.

## Out of scope

- Formatting message bodies in the message detail view or send dialog independently of snippets.
- Adding user settings, a manual format command, or a new dependency.
- Changing JSON/XML payload semantics, snippet naming, or front-matter parsing.

## Formatting rules

- When a body could be interpreted as both formats, use JSON first; otherwise try XML.
- Use two-space indentation for both formats.
- Format only when parsing succeeds. Invalid JSON/XML is retained byte-for-byte and is not an error.
