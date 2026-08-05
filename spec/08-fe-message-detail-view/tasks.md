# Tasks — FE 08: Message detail view

Plan: [plan.md](plan.md)

1. [x] **`RawFields` on `queue.Message`** — add `RawFields map[string]interface{}`
   to the `Message` struct in `backend.go`; populate it in
   `jolokia.BrowseMessages` by storing the raw decoded map after extracting
   the named fields.

2. [x] **`messageDetailView`** — create `tui/internal/app/message_detail.go`:
   bordered `tview.TextView` with dynamic colors and word-wrap, title
   `" Message Details "`; `render(queueName string, msg queue.Message)` builds
   and sets the color-tagged text (Summary / Headers / Body sections); the
   `properties` field is decoded from ActiveMQ ByteSequence objects (float64
   byte arrays → UTF-8) and each property is shown on its own indented line;
   the body is pretty-printed if valid JSON; j/k scroll; Esc/Backspace returns
   to messages page. Tests in `message_detail_test.go`: view constructs
   correctly, title matches, Shortcuts() contains Esc, render does not panic
   on nil RawFields.

3. [x] **App wiring** — in `app.go`: add `messageDetailV *messageDetailView`
   field; construct `newMessageDetailView(a)` in `New()` and register as page
   `"message-detail"`; add `openMessageDetail(queueName string, msg
   queue.Message)` method; wire `messagesView.table.SetSelectedFunc` to call
   `openMessageDetail` for the message at the selected row.

4. [x] **`reapplyTheme`** — repaint detail TextView background, border, and
   title in `theme.go`.

5. [ ] **Manual verification** — run broker + TUI; navigate to a queue,
   open messages, press Enter on a row; confirm detail view opens with correct
   title, all three sections render, long PropertiesText is visible by
   scrolling, Esc returns to messages view; theme switch repaints correctly.
