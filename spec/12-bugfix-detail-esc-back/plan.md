# Plan — Bugfix 12: Esc in message detail view does not return to message list

## Fix

In `tui/internal/app/message_detail.go`, change the Esc/Backspace handler from:

```go
a.pages.ShowPage("messages")
```

to:

```go
a.pages.SwitchToPage("messages")
```

`SwitchToPage` hides all pages and shows only the target, while `ShowPage` only
makes the target visible without hiding others.

## Files touched

- `tui/internal/app/message_detail.go` — one-line change in Esc handler
