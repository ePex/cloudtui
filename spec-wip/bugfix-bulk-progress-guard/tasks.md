# Tasks: bulk delete/move can repeat for hours when the server over-reports

See `spec.md` and `plan.md`. Each task needs its own approval before it's
implemented, and gets checked off only once implemented, tested and
pushed.

Live checks run against this repo's `mq-proxy` and the local broker, with
an isolated `HOME` and a separate tmux server (`tmux -L <name>`, or `HOME`
passed inline in the session command), never the developer's real
`~/.cloudtui`.

1. [x] **Count only messages with an ID; requests return IDs** (plan §1).
   `deleteOnce`/`moveOnce` return the non-empty message IDs; single
   requests count those, so `RemoveMessage`/`MoveMessage` report "not
   found" for an ID-less entry. `cappedServer` gets unique IDs across
   rounds and a `noID` option. Tests: ID-less entries don't count;
   single-message calls with only an ID-less entry → "not found".
   Mutation-checked.
   *Done 2026-10-10.* The round loop still takes a count for now
   (`len(ids)`); task 2 changes its signature. Mutation (counting every
   entry again) fails both new tests; all existing round tests pass
   with unique IDs. `go test -race ./...` clean.

2. [x] **Stop repeating when a round makes no progress** (plan §2).
   `inRounds` tracks seen IDs, counts only new ones, and returns
   `errBulkNoProgress` (with both numbers in the message) when a round
   reports entries but nothing new. `cappedServer` gets a `stuck`
   option. Tests as in plan §3. Mutation-checked (without the check the
   stuck test hits the 1,000-request cap).
   *Done 2026-10-10.* One refinement found by the tests: the IDs in the
   final no-progress round were counted in an earlier round but are
   evidently still there, so they come back off the total — a purge of
   1,000 real messages plus 1 stuck one reports 1,000 removed, not
   1,001. The stuck-message scenario is an inline fake server rather
   than a `cappedServer` option (clearer for that one test). Task 1's
   ID-less test now also expects the no-progress stop, since ID-less
   entries keep coming after the real messages run out. Mutation:
   without the stop, all 4 tests run the full 1,000 requests.
   `go test -race ./...` clean.

3. [x] **Regression run.** `task test`, `task test:tui:race`, smoke test
   (isolated), and live through `mq-proxy`: purge and move-all on a
   1,000+ message queue still empty it and report the right count.
   Remove the scratch queues; stop `mq-proxy`.
   *Done 2026-10-10.* `task test` exit 0 (mq-proxy-web 43/43), `task
   test:tui:race` exit 0, smoke test PASS (isolated; the real
   `cloudtui.log` untouched). Live through `mq-proxy`, 600 × TypeA +
   600 × TypeB: move-all to a second queue → `Moved 1200 message(s) …`,
   source 0, target 1,201 — in 4 `move-messages` requests of ~2 s each
   (mq-proxy's transacted move hands them out in batches; the loop
   completes it and the new no-progress check didn't misfire); then
   purge of the 1,201-message target → `Purged 1201 message(s) …`,
   empty. Queues and their statistics queues removed; `mq-proxy`
   stopped.
   *Manual check by the user*, against the other server before its own
   fix (if available): purge a queue where a live consumer holds some
   messages → stops within seconds with the "incomplete" error instead
   of repeating for hours.
