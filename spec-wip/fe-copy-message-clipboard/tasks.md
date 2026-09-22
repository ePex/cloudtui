# Tasks: Copy a message from the detail view

1. [x] Add a plain-text message clipboard formatter and unit tests for queue/summary fields, mapped headers, sorted properties, full body, JSON formatting, and binary messages.
2. [x] Wire the `c` shortcut to copy the current message and report success; test clipboard contents, status, shortcut hint, and that the detail view stays open.
3. [x] Update the message detail living spec and README to document the copied content and shortcut.
4. [x] Run `gofmt` and the full TUI test suite. Manual check for a live session: open a message detail view, press `c`, paste into another app, and confirm the queue, full headers/properties, and body are present while the detail view remains open.
