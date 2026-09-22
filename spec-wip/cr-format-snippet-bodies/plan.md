# Implementation plan: Format JSON and XML snippet bodies

## Approach

1. Add a body formatter in `internal/snippet`, with no new dependency. Try JSON first using Go's standard JSON decoder/encoder and two-space indentation, then XML using `encoding/xml` with equivalent indentation. If neither parser accepts the whole body, return it unchanged. Preserve JSON values and XML structure/content, including declarations, comments, and text; do not trim or normalize unrecognized text.
2. Apply formatting at the shared persistence boundary (`Store.Save`) so every write path stores normalized bodies, including the editor, Save as Snippet dialog, and overwrite flows. Keep the existing front-matter encoder responsible for preserving JMS Type, extra keys, and comments.
3. When the library view opens a snippet in the editor, format the loaded body and persist the normalized snippet before showing the editor if its body changes. If the write fails, report the error through the existing status path and leave the editor closed. Do not make preview or send-picker reads write to disk.
4. Format JSON/XML bodies in the library preview for display without writing to disk.
5. Update `spec/22-message-snippets/spec.md` and the README to describe formatting in the preview, normalization on editor open, and formatting on save.

## Files and tests

- `tui/internal/snippet/format.go` (new): JSON/XML detection and formatting helper.
- `tui/internal/snippet/format_test.go` (new): compact JSON/XML, already formatted input, invalid and plain text, precedence, and semantic preservation cases.
- `tui/internal/snippet/store.go` and `store_test.go`: ensure all `Save` writes persist formatted bodies while preserving front matter and existing overwrite behavior.
- `tui/internal/view/snippets.go` and `snippets_test.go`: normalize only when entering edit, verify the file is rewritten before the editor opens, and verify a write failure reports an error without opening it.
- `tui/internal/view/snippets.go` and `snippets_test.go`: format preview content without changing the file.
- `tui/internal/dialog/snippeteditor_test.go` and `snippetsave_test.go`: ensure editor and Save as Snippet persistence use formatted bodies, including overwrite confirmation paths.
- `spec/22-message-snippets/spec.md` and `README.md`: document the resulting behavior and formatter limits.

## Decisions and trade-offs

- Formatting at `Store.Save` covers all write paths and avoids inconsistent output. The editor-open path calls the same persistence behavior only when formatting changes the body.
- Read-only loads used by preview and the send picker remain read-only; merely browsing does not rewrite files.
- The library preview formats valid JSON/XML for display while keeping its source file unchanged.
- Use standard-library parsers/encoders to avoid a new dependency. XML preservation is the main technical risk: implementation must verify round-tripping of declarations, namespaces, comments, attributes, and mixed text before choosing the exact token-formatting approach. If `encoding/xml` cannot preserve those faithfully, retain XML unchanged in those cases rather than changing message meaning.
- Formatting errors are non-errors for the user: a body that is not valid JSON or XML stays untouched. Filesystem write errors during editor-open normalization are surfaced, and the editor does not open with content that could not be normalized on disk.

## Verification

- Add focused unit tests for formatting, persistence, preservation of front matter, and the editor-open failure path.
- Run `gofmt`, `task test:tui`, and the relevant repository checks after implementation; manually verify a compact JSON and XML snippet is formatted in the editor and on disk, while plain text and malformed payloads remain unchanged.
