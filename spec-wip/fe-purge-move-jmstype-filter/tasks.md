# Tasks

1. [x] Generalize `Host.ScanJMSTypes` to take an explicit `queueName`;
   add `Host.MessagesQueueName()`. Update `App`'s implementations,
   `MessageFilter.startScan()`'s call site, and every fake/stub
   (`internal/app/host_test.go`, `internal/dialog/hosttest_test.go`,
   `internal/dialog/messagefilter_test.go`,
   `internal/view/testfake_test.go`).
2. [x] New `JMSTypePrompt` dialog (`tui/internal/dialog/jmstypeprompt.go`)
   plus `jmstypeprompt_test.go`.
3. [x] Wire `JMSTypePrompt` into `app.go` (construction + page
   registration) and `queues.go` (new constructor parameter; `p`/`M`
   handlers route through it, branching on filtered vs. unfiltered);
   update `queues_test.go`.
4. [x] Manual verification via the `verify-live` skill against a real
   broker (both backends) — confirm unfiltered purge/move-all are
   unchanged, and filtered purge/move-all only affect matching messages
   (record what was checked here).

   **Bug found and fixed during this step**: pressing `p`/`M` then
   immediately Enter on the blank JMS Type field — the documented
   "leave it blank to proceed unfiltered" path — did not proceed
   unfiltered at all. Root cause: this prompt has no free suggestion
   tier (unlike `MessageFilter`'s JMS Type field), so the scan-trigger
   sentinel is unavoidably the *sole* autocomplete entry on an untouched
   field, and `Show()` opens that drop-down immediately (needed to avoid
   the separate stale-cache bug found in the previous PR). tview's
   `InputField` accepts whatever's highlighted in an open drop-down on
   Enter before ever reaching `SetDoneFunc` — so the very first Enter on
   a blank field always accepted the sentinel and triggered an unwanted
   scan, never reaching `continueNow()` with an empty `jmsType`. Fixed
   with `field.SetInputCapture`: when `event.Key() == tcell.KeyEnter`
   and the field is genuinely empty, call `continueNow()` directly and
   swallow the event — `SetInputCapture` runs before `InputField`'s own
   `InputHandler` (per `tview.Box`'s own doc comment), so this only
   intercepts the specific "nothing typed" case; typing (even partial
   prefix filtering) or navigating into the drop-down with arrows still
   goes through tview's normal accept-then-second-Enter flow, same as
   every other autocomplete field in this app. Covered by
   `TestJMSTypePromptEnterOnBlankFieldContinuesWithoutScanning`
   (confirmed to actually panic — no capture installed at all — without
   the fix, not just fail an assertion).

   Verified end-to-end against a real local ActiveMQ broker
   (`localhost:8161`), on **both backends**: Jolokia (`default`
   connection) thoroughly across all four scenarios, plus a spot-check
   of the filtered-purge scenario on mq-proxy (`local-mq-proxy`
   connection, via `task dev:proxy:start`) — the backend distinction
   matters here per spec.md's "Preserving the existing unfiltered path"
   section. Seeded disposable queues (`task test:queue:add`/`remove`)
   with mixed JMS types via direct Jolokia `sendTextMessage` JMX calls
   (same technique as the previous PR's live verification):
   - **Purge, unfiltered** (`jmsfilter-verify-purge-unfilt`, 3 mixed
     messages): blank field + Enter (after the fix) went straight to
     the confirm dialog worded "All messages will be deleted."; on Yes,
     all 3 messages were removed (pending 3→0, dequeued 0→3) — identical
     to pre-feature behavior.
   - **Purge, filtered** (`jmsfilter-verify-purge-filt`, 2×
     `OrderCreated` + 1× `PaymentFailed`): typed `OrderCreated`, confirm
     dialog correctly worded "All OrderCreated messages will be
     deleted."; on Yes, only the 2 `OrderCreated` messages were removed
     (pending 3→1, dequeued 0→2), confirmed by opening the messages view
     and seeing only the `PaymentFailed` message remained.
   - **Move-all, unfiltered** (`jmsfilter-verify-move-unfilt` →
     `jmsfilter-verify-move-unfilt-dst`, 3 mixed messages): blank field
     + Enter went straight to the move-picker (no confirm step, as
     before); selecting the target moved all 3 (source 3→0 pending/3
     dequeued, destination 0→3 pending).
   - **Move-all, filtered** (`jmsfilter-verify-move-filt` →
     `jmsfilter-verify-move-filt-dst`, 2× `OrderCreated` + 1×
     `PaymentFailed`): typed `OrderCreated`, selected the target; only
     the 2 `OrderCreated` messages moved (source 3→1 pending/2 dequeued,
     destination 0→2 pending) — confirmed by opening both queues'
     messages views: source held only `PaymentFailed`, destination held
     both `OrderCreated` messages.
   - **mq-proxy spot-check** (`jmsfilter-verify-proxy-purge`, 2×
     `OrderCreated` + 1× `PaymentFailed`, via the `local-mq-proxy`
     connection): filtered purge with `OrderCreated` correctly removed
     only the 2 matching messages (pending 3→1, dequeued 0→2, confirm
     dialog worded identically), confirming `DeleteMessages`'s existing
     mq-proxy implementation (already tested at the unit level in an
     earlier PR) works correctly end-to-end through this new UI path too.

   Cleanup: removed all 7 disposable test queues, stopped mq-proxy, and
   restored `config.yaml`'s `activeConnection` to `default` (switching
   connections for the mq-proxy spot-check changed it, same as the
   previous PR's live verification).

   **Second bug found and fixed, reported by the user directly testing
   this branch**: pressing `M`/`p` on a queue rendered the JMS Type
   prompt with the sentinel suggestion — the *only* visible content on a
   fresh open — smashed into and unreadable against the box's own bottom
   border, reading as an empty/broken overlay rather than "↻ Scan up to
   5000 messages for JMS types". Root cause: the overlay's declared
   height (`ui.Centered(a.jmsTypePrompt.Primitive(), 64, 3)`) covered
   only the field itself (border + content + border), with zero spare
   rows — but `tview.InputField.Draw` positions the autocomplete
   drop-down at exactly one row below the field's own content row
   (`ly := y+1`) regardless of the box's declared height, so with height
   3 that row lands exactly on the box's bottom border, and the
   drop-down draws directly on top of it. `MessageFilter`'s equivalent
   overlay (64×16) never hit this because its box has three more form
   fields and a button row below the JMS Type field, giving the
   drop-down room "for free"; `JMSTypePrompt` is a single field with
   nothing else in the box to borrow room from. Fixed by increasing the
   overlay height to 12 (9 spare rows below the field, comfortably
   covering a typical scanned-types list). Reproduced and confirmed
   fixed live, in tmux at a realistic 100×30 terminal size (the original
   bug's live verification pass used a much wider 160-column terminal,
   which likely mattered less for this specific *height* issue than it
   would have for a width one, but is a useful reminder that this
   feature's earlier live verification pass didn't use a size close to
   a plausible default terminal window) — confirmed the sentinel and,
   after a scan, all scanned types now render fully inside the box with
   no border overlap, and that the full accept-then-submit flow (Tab to
   accept a highlighted suggestion, second Enter to actually continue)
   still worked correctly afterward, with no accidental auto-continue
   at any step. This is purely a rendering fix (`app.go`'s overlay
   height constant) — no dialog logic changed.
   **Design change requested after this, still under task 4 (third round
   of feedback)**: the user reported that on a queue with real messages,
   pressing `M` opened a box showing only "↻ Scan up to 5000 messages for
   JMS types" — nothing indicated that entry was something to *select*
   to see real type names, so it read as "nothing appears" or "empty."
   Clarified via a follow-up question that the box itself *was* rendering
   correctly (confirming the overlay-height fix above held) — the gap was
   UX, not rendering: with no free suggestion tier, real type names only
   ever appeared after a deliberate two-step interaction (select the
   sentinel, then a second Enter), which wasn't discoverable. Asked the
   user whether to (a) keep the opt-in design and just make the sentinel
   read more obviously as an action, or (b) auto-scan on open; the user
   chose (b).

   **Implemented**: `Show()` now starts a scan automatically every time
   the prompt opens, capped at a new `jmsTypeAutoScanCount` (500,
   smaller than the opt-in sentinel's `jmsTypeScanCount` = 5000, since
   this one now runs unconditionally on every `p`/`M` press rather than
   only when explicitly requested). The sentinel entry remains, now as a
   "the automatic pass didn't find what I wanted — search wider" option.
   `continueNow()`'s refuse-while-scanning guard had to change from
   checking `jp.scanning` (now true immediately after every `Show()`,
   since that flag is set synchronously before Show() even returns — so
   the old guard would have blocked "leave blank, press Enter" for the
   *entire* duration of every automatic scan) to checking whether the
   field literally holds the scan-trigger sentinel's own text — the only
   condition that's actually unsafe to submit, and one the automatic
   scan never produces (it never touches the field's text at all while
   in flight, unlike the opt-in sentinel path).

   **A real `-race` bug found and fixed in the test suite while updating
   it for this change**: `TestJMSTypePromptContinueNowRefusesWhileScanning`
   (renamed in the process) called `jp.field.SetText(jmsTypeScanSentinel)`
   directly to simulate the sentinel being selected — not realizing this
   fires the field's *real* production `SetChangedFunc` wiring, exactly
   as a live keystroke would, starting a real background scan goroutine.
   With the test's default (non-blocking) fake `ScanJMSTypes`, that
   goroutine's own `SetStatus` call could run concurrently with the
   test's own subsequent `continueNow()` call (also calling `SetStatus`)
   with no synchronization between them — a genuine data race, caught by
   `go test -race` (not by a plain `go test ./...` run, which passed
   throughout). Fixed by giving that test (and auditing every other test
   that calls `Show()` or otherwise triggers a real scan) a
   `blockForever` fake `ScanJMSTypes` so the spawned goroutine never
   proceeds far enough to touch shared state concurrently with the
   test's own assertions — the same technique this file's async tests
   already used in a few places, now applied consistently. Confirmed
   `go test ./internal/dialog/... -run TestJMSTypePrompt -race` passes
   cleanly; confirmed two *unrelated*, pre-existing races elsewhere in
   this repo's test suite (`internal/view/datadoglogs_test.go`,
   `internal/app/host_test.go`) reproduce identically on `main`/this
   branch's `HEAD` with none of this session's changes applied, so they
   were left alone as out of scope for this feature.

   Reproduced the original report and the fix live again (`jmsverify-autoscan`
   test queue with 2 mixed-type messages): pressing `M` now shows real
   type names (`OrderCreated`, `PaymentFailed`) immediately, with no
   action required; pressing Enter on the still-blank field immediately
   after `M` (before waiting for the automatic scan to finish) still
   correctly proceeds unfiltered, straight to the move-picker; selecting
   a real scanned type and completing the flow still correctly moved
   only the matching message to the target queue (confirmed via each
   queue's own messages view afterward, same as this task's earlier
   verification pass). Cleanup: removed the two disposable test queues
   used for this round, no `config.yaml`/connection changes this time
   (Jolokia `default` connection only).

5. [ ] Merge-back: update `spec/09-queue-message-actions/spec.md` (JMS
   Type filter step for purge/move-all) and add a short cross-reference
   note to `spec/08-message-browser-and-detail/spec.md` where it
   mentions `Host.ScanJMSTypes` (signature now takes an explicit queue
   name); delete `spec-wip/fe-purge-move-jmstype-filter/`.
