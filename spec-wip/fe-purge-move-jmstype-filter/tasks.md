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
5. [ ] Merge-back: update `spec/09-queue-message-actions/spec.md` (JMS
   Type filter step for purge/move-all) and add a short cross-reference
   note to `spec/08-message-browser-and-detail/spec.md` where it
   mentions `Host.ScanJMSTypes` (signature now takes an explicit queue
   name); delete `spec-wip/fe-purge-move-jmstype-filter/`.
