# Implementation plan: Copy a message from the detail view

## Approach

1. Add a plain-text clipboard formatter for a queue name and `queue.Message`. Include the same summary values, mapped header labels/values, sorted PropertiesText entries, and full body used by the detail view. Reuse `decodePropertyValue` for ActiveMQ ByteSequence properties and `prettyJSON` for body readability. Keep formatting free of tview color tags and preserve the detail view's existing empty/binary placeholders.
2. Add `c` handling to `MessageDetailView`: format the currently rendered message, send it through the existing `Host.CopyToClipboard`, and set a success status that names the queue but not the message contents. Keep the view and scroll position unchanged.
3. Add `c: copy message` to `MessageDetailView.Shortcuts()` so the context panel advertises the action.
4. Update `spec/08-message-browser-and-detail/spec.md` and the README to describe the shortcut and copied fields.

## Files and tests

- `tui/internal/view/message_detail.go`: clipboard text formatter, shortcut handling, and shortcut hint.
- `tui/internal/view/message_detail_test.go`: formatter cases for queue/summary, mapped headers, sorted properties, full body, JSON formatting, binary messages, and clipboard shortcut/status behavior.
- `spec/08-message-browser-and-detail/spec.md` and `README.md`: document copy behavior and the `c` key.

## Decisions and trade-offs

- Copy a readable record rather than a raw backend map. The content matches human-readable fields on the detail page and avoids leaking backend implementation fields that the UI does not expose.
- Use the full body from the loaded message, not the truncated message-list preview. Format valid JSON the same way the detail page already does.
- Reuse the existing OSC 52 clipboard capability. This adds no dependency and behaves consistently with other detail-view copy actions.
- Copying is synchronous and local: it performs no broker request and leaves the current view in place.

## Verification

- Add unit tests for output content and ordering, plus the key action's clipboard/status behavior.
- Run `gofmt` and `task test:tui`; manually verify `c` copies the complete record and that copy leaves the user on the same detail view.
