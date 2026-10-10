# Tasks: JMS Type fields lose typed text to the type scan

See `spec.md` and `plan.md`. Each task needs its own approval before it's
implemented, and gets checked off only once implemented, tested and
pushed.

Live checks use the `verify-live` skill against the local broker, with
an isolated `HOME` and a separate tmux server (`tmux -L <name>`) so the
developer's real `~/.cloudtui` — whose active connection is a shared
remote environment — is never used. A disposable queue with a few
TypeA messages is enough.

1. [x] **Fix the racing test** (plan §2). The theming test's
   `JMSTypePrompt` case blocks the automatic scan. First, so `-race` is
   clean before CI starts enforcing it.
   *Check:* `go test -race ./internal/dialog/` passes; reverting the
   one-line fix brings the race back.
   *Done 2026-10-10.* The test fix alone left a second, real race:
   `startScan`'s goroutine read `jp.queueName` while a reopened prompt's
   `Show` wrote it. Fixed by copying the name before starting the
   goroutine (spec.md amended). Each fix alone still fails `-race`;
   both together pass, three runs in a row, and `go test ./...` passes.

2. [x] **CI runs the TUI tests with `-race` on Ubuntu** (plan §3). New
   `task test:tui:race`; the `tui` job runs it on Linux and `task
   test:tui` on macOS/Windows.
   *Check:* `task test:tui:race` passes locally; the PR's CI shows the
   Linux job running with `-race` and passing.
   *Done 2026-10-10.* Locally: 19 packages pass, `actionlint` clean. CI
   run 38009270849: `tui (ubuntu-latest)` ran `task: [test:tui:race] go
   test -race ./...` and passed (1m45s); Windows/macOS ran `go test
   ./...`; all 12 checks green.

3. [x] **Shared JMS Type autocomplete, wired into the purge/move-all
   prompt** (plan §1). New `jmstypeautocomplete.go` (`chosen` tracking,
   autocompleted callback, Enter/Tab input capture). `JMSTypePrompt`
   uses it; its sentinel detection, Enter-on-empty capture,
   `handleScanResult`'s field clearing and `continueNow`'s
   "still scanning" refusal go. Tests as in plan §1 for the prompt,
   including defect 1 (text typed during the automatic scan survives);
   obsolete tests replaced. Mutation-checked.
   *Live check:* `p` on the queue, type `TypeB` immediately → it's still
   there after the scan; Enter → confirm dialog for `TypeB` (then No);
   `p` again, Tab on the empty field → no scan, Enter → confirm for all
   messages (No); `p`, Down onto `TypeA`, Enter → field shows `TypeA`;
   Down onto the scan entry, Enter → scan runs, status shows it.
   *Done 2026-10-10.* `jmsTypeAutocomplete` also records the entry the
   arrow keys last reached (`highlighted`), which tests navigate by —
   tview's sticky highlight makes "press Down N times" unreliable.
   Removed tests: the two `onJMSTypeChanged` ones, "refuses while the
   field holds the sentinel", and the Enter-on-empty capture test
   (replaced by key-path tests through the real input handling).
   Mutation-checked: disabling the capture fails 3 tests; restoring
   `handleScanResult`'s `SetText("")` fails 2. `go test -race ./...`
   clean. Live (scratch `HOME`, local broker, 5 × TypeA): typing `TypeB`
   right after `p` survived the scan; Enter → `Purge …? All TypeB
   messages…` (No); Tab on an empty field → no scan, then Enter → `All
   messages…` (No); Down + Enter → field `TypeA`. The scan entry was
   checked through `mq-proxy` because a local Jolokia scan of 5
   messages finishes too fast to see its status: Down onto it + Enter →
   a `maxCount=5000` browse in the `mq-proxy` log, field still empty.
   Where Down starts depends on tview's sticky highlight (sometimes it
   wraps to `TypeA` first) — tview behavior, not changed here.

4. [ ] **Message filter uses the shared autocomplete** (plan §1).
   `MessageFilter`'s JMS Type field uses the helper; its sentinel
   detection, `handleScanResult`'s field clearing and `apply()`'s
   "still scanning" refusal go. Tests as in plan §1 for the filter
   (incl. Tab moving form focus). Mutation-checked.
   *Live check:* `f` inside the queue, type `TypeB`, Tab ×4 to Apply,
   Enter → title shows `type=TypeB`; `f`, Tab from the empty field →
   focus moves to From, no scan; Down onto the scan entry, Enter → scan
   runs.

5. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke test
   (isolated `HOME` + separate tmux server), and the live checks from
   tasks 3–4 in one session. Remove the disposable queue.
