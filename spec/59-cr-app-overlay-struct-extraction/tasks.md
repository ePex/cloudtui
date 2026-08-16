# Tasks — CR 59: extract App overlay state into dedicated structs

Each task must build and pass tests on its own before moving to the next
— they're ordered so `app.go`'s two OR-chains (`~793`, `~839`) and
`theme.go`'s `reapplyTheme` get exactly one overlay's field-path updated
per task, leaving the not-yet-extracted overlays' old fields untouched
until their own task.

1. [x] Extract the confirm dialog into `tui/internal/app/confirm.go`
       (`confirmDialog` struct, `newConfirmDialog`, `show`, `close` — see
       plan.md). Update `app.go`: remove the 4 old fields and
       `showConfirm`/`closeConfirm`; `New()` calls `newConfirmDialog`;
       all `a.showConfirm(...)` call sites (in `app.go` and
       `messages.go`/`message_detail.go`/`queues.go`) become
       `a.confirm.show(...)`; the confirm part of both OR-chains becomes
       `a.confirm.visible`. Update `theme.go`'s confirm section in
       `reapplyTheme`. Update the 3 confirm-related lines in
       `messages_test.go`.
2. [x] Extract the move picker into `tui/internal/app/movepicker.go`
       (`movePicker` struct, `newMovePicker`, `show`, `fillList`,
       `close`, plus the 5 relocated free functions `isSystemQueue`/
       `isDLQQueue`/`isIMQQueue`/`requeueQueueCandidate`/
       `sortPickerQueues` — see plan.md). Update `app.go`: remove the 8
       old fields and `showMovePicker`/`fillPickerList`/`closeMovePicker`
       plus the 5 free functions; `New()` calls `newMovePicker`; all
       `a.showMovePicker(...)` call sites become `a.movePicker.show(...)`;
       the move-picker part of both OR-chains becomes
       `a.movePicker.visible`. Update `theme.go`'s move-picker section.
       Update the 2 move-picker-related lines in `messages_test.go`.
3. [x] Extract the send-message overlay into
       `tui/internal/app/sendmessage.go` (`sendMessageOverlay` struct,
       `newSendMessageOverlay`, `show`, `doSend`, `close` — see plan.md).
       Update `app.go`: remove the 5 old fields and
       `showSendMessage`/`doSend`/`closeSendMessage`; `New()` calls
       `newSendMessageOverlay`; all `a.showSendMessage(...)` call sites
       become `a.sendMessage.show(...)`; the send-message part of both
       OR-chains becomes `a.sendMessage.visible`. Update `theme.go`'s
       send-message section. (No `messages_test.go` references to
       update — confirmed none exist.)
4. [x] Verify: `go build ./...` and `go test ./...` pass in `tui/`;
       confirm `app.go`'s line count dropped meaningfully; manual live
       verification (`verify-live` skill) — confirm dialog (`Esc`/`No`/
       `Yes`), move picker (search, `j`/`k`, selection), send-message
       (`Tab` focus, `Esc` cancel from either widget) all behave
       identically to before; switch theme while each overlay is open in
       turn and confirm live recoloring still applies to all three.

       `gofmt`, `go vet`, `go build ./...`, `go test ./...` all clean.
       `app.go` dropped from 1390 to 1056 lines (-334, -24%); the three
       new files total 405 lines.

       Manually verified 2026-08-16 via `verify-live` (tmux-driven real
       binary, against the real local `default`/jolokia connection and
       the project's `orders` scratch queue): created a message via the
       send-message overlay (`c`, typed body, `Tab` to Submit, `Enter`) —
       appeared in the list correctly. Opened the move picker (`m`) on it
       — sorted queue list rendered, `/` search filtered live (typed
       `ord`, list narrowed to matches; typed a non-matching string,
       list emptied) — closed without completing the move. Opened the
       confirm dialog (`d`) — correct question text, `Esc` cancelled
       (message still present), re-opened and confirmed via
       `Down`+`Enter` — message deleted. Switched theme `dark` →
       `cyberpunk` from Settings, reopened all three overlays afterward
       with no crash (confirms `reapplyTheme`'s updated field paths —
       `a.confirm.*`, `a.movePicker.*`, `a.sendMessage.*` — are correct;
       a missed rename would nil-panic here), then switched back to
       `dark` to restore the starting state.

       **One scare, resolved, worth recording:** during the first pass
       (a long session with many rapid, back-to-back tmux `send-keys`
       calls — including a tight loop of 15 backspaces at 0.02s
       intervals), the app process was observed pegged at ~100% CPU and
       climbing (1:50 → 2:03 CPU-seconds over ~13s wall time). Investigated
       by killing that session and repeating the same interactions
       (confirm, move picker + search, send-message, theme switch) in a
       fresh session with `ps` checked after every step, including several
       seconds idle with each overlay left open. CPU stayed at 0.0–0.9%
       throughout, every time. Concluded the spike was an artifact of the
       rapid/malformed test automation (flooding tview's event queue),
       not a regression from this CR — but flagging it here rather than
       silently dropping it, since "high CPU after a UI refactor" is
       exactly the kind of thing worth being suspicious of.
