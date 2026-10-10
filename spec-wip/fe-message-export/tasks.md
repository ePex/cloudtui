# Tasks: export a queue's messages to a file

See `spec.md` and `plan.md`. Each task needs its own approval before it's
implemented, and gets checked off only once implemented, tested and
pushed.

Live checks use the `verify-live` skill: local broker, isolated `HOME`
(passed inline in the tmux session command, or a separate tmux server),
never the developer's real `~/.cloudtui`. Both backends where the task
says so: Jolokia (`admin`/`admin` on the local broker) and this repo's
`mq-proxy`. Exported files go to a scratch directory and are inspected
with `jq`.

1. [x] **Move the shared message fields into `internal/queue`** (plan
   §1). `queue.HeaderFields()`, `queue.DecodePropertyValue`,
   `(Message).Body()`; the detail view and its clipboard text use them.
   No behavior change — existing detail-view tests must pass unchanged;
   new unit tests for the moved functions.
   *Done 2026-10-10.* New `internal/queue/fields.go` (+ tests); the
   detail view's snippet save, clipboard text and render now call
   `Body()`/`HeaderFields()`/`DecodePropertyValue` (−73/+21 lines there).
   Existing view tests pass unchanged; `Body()` mutation (empty body
   reported as present) fails its test. `go test -race ./...` clean. No
   spec referenced the moved helpers.

2. [x] **`internal/export` package** (plan §2). `Record`/`NewRecord`,
   `WriteFile` (temp file + rename), `DefaultFileName`, `DefaultDir`,
   `MaxMessages`. Unit tests as in plan §2, mutation-checked.
   *Done 2026-10-10.* Details settled here: timestamps carry
   milliseconds (`2026-10-10T13:30:00.123Z`) and are omitted when
   unknown; the correlation ID isn't repeated inside `headers`; HTML
   escaping is off so bodies stay readable; the file is created
   owner-readable only (it can hold message contents). The "existing
   file stays intact" case is tested by making the final move fail
   (target is a directory) — the temp file is cleaned up and the target
   untouched. `DefaultDir` tests set both `HOME` and `USERPROFILE`
   (CI runs Windows too). Mutations caught: HTML escaping on, no
   temp-file cleanup, inferred JMS type kept. `go test -race ./...`
   clean.

3. [x] **TUI export flow, `e` in the queue list** (plan §3). Shared
   `exportFlow` (path prompt → overwrite confirm → collect → write →
   status line), wired into `QueuesView` with a fresh browse capped at
   `MaxMessages` plus the pending count. Tests as in plan §3 for the
   flow and the queue list.
   *Live check (both backends):* `e` on a 10-message queue → default
   path prefilled, Enter → `Exported 10 message(s) to …`, file has 10
   valid JSON lines with full bodies and the real JMS types; `e` on a
   1,600-message queue → `… 400 of 1,600, the broker's browse limit`;
   `e` again with the same path → overwrite confirm: No reopens the
   prompt, Yes overwrites.
   *Done 2026-10-10.* The flow continues from the prompt's `onClose`
   (after it handed focus back), not from `onSubmit`, so the overwrite
   confirm keeps focus. Two wordings changed after seeing them live
   (spec.md amended): the result line puts the "of <total>" note before
   the path (a long path cut it off), and the confirm asks `Overwrite
   the existing file <name>?` (a long path pushed the question out of
   the three-row dialog; now covered by a rendering test).
   `fakeQueueBackend` gained a `browseFn`. Mutations caught: no
   overwrite check (the No test), path-first confirm wording. `go test
   -race ./...` clean. Live, `mq-proxy`: 10-message queue → `Exported
   10 message(s)`, file 10 valid lines, real `TypeA`, full bodies,
   owner-only; 1,600-message queue → `Exported 400 of 1600 message(s)
   (the broker's browse limit)`, 400 lines; overwrite confirm: No →
   prompt reopens with the same path, Yes → file replaced. Jolokia: the
   1,600 queue → same "400 of 1600" note (from the pending count —
   Jolokia never reports `hasMore`); 10-message queue → 10 lines, with
   the JMS headers Jolokia provides (delivery mode, destination,
   priority, …). Over Jolokia the queue list also shows the broker's
   `ActiveMQ.Statistics.Destination.*` queues (mq-proxy hides them);
   exporting one gives 0 messages, correctly.

4. [x] **TUI: `e` in the message list** (plan §3). Marked messages, or
   else the shown ones; `MessagesView` keeps the last load's `hasMore`/
   pending count for the incomplete note. Tests for marked, unmarked
   with quick search, and the note.
   *Live check:* mark 3 messages, `e` → file has exactly those 3; no
   marks with a quick search active → file has exactly the shown rows;
   on the 1,600-message queue unmarked → the "of" note.
   *Done 2026-10-10.* The note's reason here is "only what the list
   shows"; an empty list says `No messages to export` instead of opening
   the prompt. The shortcut panel holds 12 entries and `e` made 13, so
   `a` and `n` now share one line (`a/n  mark all/none`, like the queue
   list's `o/O`) — both keys work as before. Mutations caught: marks
   ignored, all loaded messages exported instead of the shown ones.
   `go test -race ./...` clean. Live via `mq-proxy`: 3 marked → file
   with exactly those 3 (`m10 m9 m8`); quick search `m1` → the 2 shown
   rows (`m10`, `m1`); the 1,600-message queue unmarked → `Exported 400
   of 1600 message(s) (only what the list shows)`, matching the title's
   `(showing 400 of 1600)`.

5. [x] **Web console Export button** (plan §4). `export.js` with
   `node --test` tests, `truncated` in `api.js`, the toolbar button and
   info line in `messages.js`/`index.html`.
   *Live check (built `dist/index.html` against `mq-proxy`, served over
   `http://localhost` with that origin allowed in mq-proxy's CORS
   settings, since the browser automation can't open `file://`):*
   Export with 2 selected → downloaded file has those 2; no selection on
   a 10-message queue → all 10; on the 1,600-message queue → info line
   `… 400 of 1600 …`; every line parses as JSON.

6. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke test
   (isolated), and the live checks from tasks 3–5 in one session.
   Remove scratch queues and files; stop `mq-proxy`.
