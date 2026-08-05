# Tasks — FE 07: Queue message browser

Plan: [plan.md](plan.md)

1. [x] **`queue.Message` + extend `Backend`** — add `Message` struct
   (`ID`, `JMSType string`, `CorrelationID string`, `Timestamp time.Time`,
   `Preview string`) to `backend.go`; add `BrowseMessages(ctx context.Context,
   queueName string) ([]Message, error)` to the `Backend` interface; update
   `fakeQueueBackend` in `queues_test.go`.

2. [x] **Jolokia `BrowseMessages`** — implement `BrowseMessages` in
   `jolokia.go` using a single `exec` POST for `browseMessages()`; parse
   `messageId`, `jMSType` (infer from `text`/`bodyLength` if absent),
   `jMSCorrelationID`, `timestamp` (epoch ms → `time.Time`), and `text`
   (truncated to 80 chars; `"(binary)"` when absent); tests in
   `jolokia_test.go`: happy path, HTTP error, Jolokia error status.

3. [x] **`messagesView`** — create `tui/internal/app/messages.go`:
   bordered table, styled header (ID / TYPE / CORR.ID / TIMESTAMP / PREVIEW),
   rows selectable, sorted by timestamp descending; `load()` calls
   `BrowseMessages` in a goroutine; errors logged + shown in table;
   Esc/Backspace returns to queues; `r` refreshes. Tests in
   `messages_test.go`: header labels, column count (5), shortcut `r` present.

4. [x] **App wiring** — in `app.go`: add `messagesV *messagesView` field;
   construct `newMessagesView(a)` in `New()` and add primitive as page
   `"messages"` (not in `a.views`); add `openMessages(queueName string)`
   method; wire `queuesView.table.SetSelectedFunc` after both views exist.

5. [x] **`reapplyTheme`** — repaint messages table background, border, and
   title colors in `theme.go`.

6. [x] **Manual verification** — run broker + TUI; navigate queues, press
   Enter on a queue; confirm messages view opens with correct title; confirm
   columns, timestamp format, and preview; Esc returns to queues; `r`
   refreshes; theme switch repaints correctly.
