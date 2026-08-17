# Tasks — CR 67: declare `Host`, add `App`'s wrapper methods

1. [x] Create `internal/ui/host.go` with the `Host` interface (20
   methods) per plan.md. `gofmt -l`, `go build ./...` (compiles
   standalone — nothing implements it yet).

2. [x] Create `internal/app/host.go` with all 12 new wrapper methods
   plus `var _ ui.Host = (*App)(nil)` per plan.md. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean —
   confirms `*App` satisfies `Host` with no other code changes yet.

3. [x] Update `sendmessage.go`'s `doSend` to call `a.ReloadAfterSend(queueName)`
   instead of the inline reload block. `gofmt -l`, `go vet ./...`,
   `go build ./...`, `go test ./...` all clean.

4. [x] Update `messagefilter.go`'s `apply` and `clear` to call
   `a.ApplyMessagesFilter(...)` (with the parsed filter / a zero
   `queue.MessageFilter{}` respectively) instead of the inline
   filter-set/updateTitle/load sequence, per plan.md's noted reordering.
   `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...` all
   clean.

5. [x] Live-verify (`verify-live` skill) the two behavior-adjacent call
   sites:
   - Send a message to a queue — confirm the queues view and (if open
     on the same queue) messages view both refresh.
   - Apply a message filter — confirm the messages table updates to the
     filtered set.
   - Clear a message filter — confirm the table reloads unfiltered.
   Record what was checked here.

   Verified live via tmux against the real broker, using the "orders"
   scratch queue:
   - Sent a test message via the send-message overlay — status bar
     showed "Message sent to \"orders\"" and the messages table
     immediately showed the new message (`ReloadAfterSend` confirmed).
   - Applied a filter (Max Count: 1) — title updated to
     "(filter: max=1)" and the table narrowed to exactly 1 row
     (`ApplyMessagesFilter`'s apply path confirmed; also confirms
     `MessagesFilter()`'s getter, since the form correctly prefilled
     from it on reopen).
   - Cleared the filter — title reset to "(filter: max=500)" (the
     default) and the table reloaded (`ApplyMessagesFilter`'s clear
     path confirmed).
   - Deleted the test message afterward to leave "orders" as found (0
     pending, matching its state before this session).

   Hit the same `tview.Form` focus-memory gotcha as CR 66 (worse here,
   since some Tab presses appeared not to move focus at all — turned
   out focus previously landed one field further than assumed after a
   prior interaction). Resolved with the same technique: probe with a
   throwaway character/backtab sequence before trusting a Tab count.

6. [x] Final verification pass: `gofmt -l tui/` and `go vet ./...` clean
   repo-wide, `go build ./...` and `go test ./...` pass repo-wide. No
   commit needed unless this surfaces something to fix.
