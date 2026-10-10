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

1. [ ] **Move the shared message fields into `internal/queue`** (plan
   §1). `queue.HeaderFields()`, `queue.DecodePropertyValue`,
   `(Message).Body()`; the detail view and its clipboard text use them.
   No behavior change — existing detail-view tests must pass unchanged;
   new unit tests for the moved functions.

2. [ ] **`internal/export` package** (plan §2). `Record`/`NewRecord`,
   `WriteFile` (temp file + rename), `DefaultFileName`, `DefaultDir`,
   `MaxMessages`. Unit tests as in plan §2, mutation-checked.

3. [ ] **TUI export flow, `e` in the queue list** (plan §3). Shared
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

4. [ ] **TUI: `e` in the message list** (plan §3). Marked messages, or
   else the shown ones; `MessagesView` keeps the last load's `hasMore`/
   pending count for the incomplete note. Tests for marked, unmarked
   with quick search, and the note.
   *Live check:* mark 3 messages, `e` → file has exactly those 3; no
   marks with a quick search active → file has exactly the shown rows;
   on the 1,600-message queue unmarked → the "of" note.

5. [ ] **Web console Export button** (plan §4). `export.js` with
   `node --test` tests, `truncated` in `api.js`, the toolbar button and
   info line in `messages.js`/`index.html`.
   *Live check (built `dist/index.html` against `mq-proxy`, opened via
   `file://` in Chrome):* Export with 2 selected → downloaded file has
   those 2; no selection on a 10-message queue → all 10; on the
   1,600-message queue → info line `… 400 of 1,600 …`; files valid per
   `jq`.

6. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke test
   (isolated), and the live checks from tasks 3–5 in one session.
   Remove scratch queues and files; stop `mq-proxy`.
