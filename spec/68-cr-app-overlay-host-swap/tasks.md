# Tasks — CR 68: swap 8 overlays to `host ui.Host`

1. [x] Swap `confirm.go`'s `confirmDialog` to `host ui.Host` per
   plan.md's substitution table. `gofmt -l`, `go vet ./...`,
   `go build ./...`, `go test ./...` all clean.

2. [x] Swap `movepicker.go`'s `movePicker` to `host ui.Host`, including
   the constructor-closure rename noted in plan.md. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

3. [x] Swap `sendmessage.go`'s `sendMessageOverlay` to `host ui.Host`,
   including the constructor-closure rename noted in plan.md. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

4. [x] Swap `messagefilter.go`'s `messageFilter` to `host ui.Host`,
   including updating `show()`/`close()` to use `MessagesFilter()`/
   `FocusMessages()` instead of reaching into `messagesV` directly
   (the two extra call sites plan.md flagged). `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

5. [x] Swap `timerangemodal.go`'s `timeRangeModal` to `host ui.Host`
   per plan.md's per-file notes. `gofmt -l`, `go vet ./...`,
   `go build ./...`, `go test ./...` all clean.

6. [x] Swap `datadogsettings.go`'s `datadogEditor` to `host ui.Host`
   per plan.md's per-file notes (leaving the `App`-side
   `SaveDatadogConfig` method itself untouched). `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

7. [x] Swap `settings.go`'s `themePicker` (only — not `settingsView`)
   to `host ui.Host` per plan.md's per-file notes. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

8. [x] Swap `awsprofiles.go`'s `awsProfilesPicker` to `host ui.Host`
   per plan.md's per-file notes. `gofmt -l`, `go vet ./...`,
   `go build ./...`, `go test ./...` all clean.

9. [x] Live-verify (`verify-live` skill) per spec.md's sample:
   `confirmDialog` (a delete confirmation), `movePicker` (moving a
   message), `themePicker` (switching theme via the picker), and
   `timeRangeModal` (open + apply a relative preset) as full checks;
   `sendMessageOverlay`, `messageFilter`, `datadogEditor`,
   `awsProfilesPicker` as quick smoke re-checks (already covered fully
   in CR 66/67). Record what was checked here.

   Verified live via tmux against the real broker/AWS/Datadog:
   - `themePicker`: opened via Settings, correctly showed `dark`
     starred, switched to `cyberpunk` — theme changed and picker
     closed cleanly.
   - `timeRangeModal`: opened via Datadog Logs (`t`), applied the "1h"
     relative preset — real data loaded, tabs/list rendering correct.
   - `movePicker`: opened via "move queue" (`M`) on the queues list —
     populated with real queue names via `Backend().List()`; filter
     narrowed correctly to zero matches on a nonsense string.
   - `confirmDialog`: triggered via purge (`p`) on a queue — dialog
     showed the correct message, "No" was default-focused, cancelling
     left the queue untouched.
   - Smoke re-checks: sent a message (`sendMessageOverlay` +
     `ReloadAfterSend`), applied and cleared a message filter
     (`messageFilter`), opened `datadogEditor` (correctly prefilled
     from `Config().Datadog`), opened `awsProfilesPicker` (populated
     via `ListAWSProfiles`) — all correct, no regressions from CR 66/
     67's behavior.
   - Restored theme to `dark` afterward (changed via the `themePicker`
     check) and deleted the smoke-test messages from the "orders"
     scratch queue, leaving both as found.

10. [x] Final verification pass: `grep -rn '\.app\.' tui/internal/app/{confirm,movepicker,sendmessage,messagefilter,timerangemodal,datadogsettings}.go` and the `themePicker`/`awsProfilesPicker` sections of `settings.go`/`awsprofiles.go` return nothing; `gofmt -l tui/` and `go vet ./...` clean repo-wide; `go build ./...` and `go test ./...` pass repo-wide. No commit needed unless this surfaces something to fix.

    Confirmed: zero remaining `.app.` access in all 8 files, all checks clean.
