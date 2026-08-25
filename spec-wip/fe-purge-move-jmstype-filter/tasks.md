# Tasks

1. [x] Generalize `Host.ScanJMSTypes` to take an explicit `queueName`;
   add `Host.MessagesQueueName()`. Update `App`'s implementations,
   `MessageFilter.startScan()`'s call site, and every fake/stub
   (`internal/app/host_test.go`, `internal/dialog/hosttest_test.go`,
   `internal/dialog/messagefilter_test.go`,
   `internal/view/testfake_test.go`).
2. [x] New `JMSTypePrompt` dialog (`tui/internal/dialog/jmstypeprompt.go`)
   plus `jmstypeprompt_test.go`.
3. [ ] Wire `JMSTypePrompt` into `app.go` (construction + page
   registration) and `queues.go` (new constructor parameter; `p`/`M`
   handlers route through it, branching on filtered vs. unfiltered);
   update `queues_test.go`.
4. [ ] Manual verification via the `verify-live` skill against a real
   broker (both backends) — confirm unfiltered purge/move-all are
   unchanged, and filtered purge/move-all only affect matching messages
   (record what was checked here).
5. [ ] Merge-back: update `spec/09-queue-message-actions/spec.md` (JMS
   Type filter step for purge/move-all) and add a short cross-reference
   note to `spec/08-message-browser-and-detail/spec.md` where it
   mentions `Host.ScanJMSTypes` (signature now takes an explicit queue
   name); delete `spec-wip/fe-purge-move-jmstype-filter/`.
