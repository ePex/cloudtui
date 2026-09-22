# Tasks: Copy a message from the detail view

1. [ ] Add a plain-text message clipboard formatter and unit tests for queue/summary fields, mapped headers, sorted properties, full body, JSON formatting, and binary messages.
2. [ ] Wire the `c` shortcut to copy the current message and report success; test clipboard contents, status, shortcut hint, and that the detail view stays open.
3. [ ] Update the message detail living spec and README to document the copied content and shortcut.
4. [ ] Run formatting and the full TUI test suite; manually verify the copy action from the live detail view.
