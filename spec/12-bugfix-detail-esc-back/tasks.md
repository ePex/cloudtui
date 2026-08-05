# Tasks — Bugfix 12: Esc in message detail view does not return to message list

Plan: [plan.md](plan.md)

1. [x] **Fix Esc handler** — replace `ShowPage("messages")` with
   `SwitchToPage("messages")` in `message_detail.go`.
