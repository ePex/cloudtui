# Tasks: blocking progress modal

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`tmux -L`), never against the real `~/.cloudtui`.

1. [ ] **Modal primitive** (plan §1). `dialog/progress.go`: `ProgressModal`
   with the pure `rainbowCells` / `catCells` / `spinnerRune` helpers,
   theme support, and `SimulationScreen` tests: rainbow at 0 %, 50 %,
   100 % on a 256-color screen; spinner plus `3 of 10` on a 16-color
   screen and for `total == 0`; footer and detail lines.
   Not wired into the app yet.

2. [ ] **Operation runner and `:progress-demo`** (plan §2, §5).
   `ui.Operation` / `ui.Host.StartOperation`, `app/operation.go`
   (one operation at a time, 250/400 ms timing with injected timers,
   ticker only while visible, key blocking with Esc and Ctrl+C), the
   `"progress"` page above `"confirm"`, the test doubles, and the
   `progress-demo` / `progress-demo spinner` prompt commands.
   Tests: timing, supersede, keys swallowed, Esc for both kinds, the
   prompt commands (incl. autocomplete).
   *Live check (no broker needed):* `:progress-demo` shows the rainbow
   bar filling 1…20 with the cat at its head, then the spinner; other
   keys do nothing; Esc ends it early. `:progress-demo spinner` shows
   the spinner with `3 of 20`-style counts. Check it in two themes and
   at a narrow terminal width (60 columns). Also run it with
   `TERM=xterm` (16 colors) to see the automatic fallback.

3. [ ] **Mutations never cut off mid-request** (plan §4). Proxy: round
   requests use `context.WithoutCancel`, `inRounds` returns the total
   plus `context.Canceled` when stopped between rounds. Both backends:
   every mutating HTTP request uses `context.WithoutCancel`. Tests: an
   `httptest` server that holds a round until the test cancels, then
   answers — the round's result still counts and no further round
   starts; the same for a Jolokia mutation.

4. [ ] **Queue list and message list loads** (plan §3). `QueuesView.Load`
   and `MessagesView.Load` run as abortable operations; Esc shows
   `Loading cancelled — press r to retry`; a superseded queue reload
   can't close the newer modal; SSO re-auth text also goes to the
   modal's detail line. Tests for each.
   *Live check (proxy backend):* point a scratch proxy connection at a
   port that accepts connections but never answers (e.g. `nc -l 8111`;
   a closed port fails too fast to see anything), open the queue list →
   spinner `Loading queues` after a moment; Esc → retry row; `r` →
   modal again. A fast local load shows no modal (no flash).

5. [ ] **Purge, move all, single and bulk delete/move** (plan §3).
   Purge (queue list and message list) and move all as stoppable
   operations reporting `(cancelled)`; `deleteMarked` / `moveMarked`
   with `Step` per message, stop before the next message, and
   `Deleted 3 of 10 message(s); cancelled`; the detail view's single
   delete/move. Tests for each, including failures counting as done.
   *Live check (both backends):* mark ~200 messages on a scratch queue
   and delete → rainbow bar counts up with `n of 200`; Esc partway →
   stops after the current message, status line and reloaded list
   agree. Purge the 1,600-message scratch queue → spinner; on the proxy
   backend Esc stops before the next round.

6. [ ] **Smoke test and TUI regression** (plan §Testing). Make
   `tui/scripts/smoke-test.sh` wait for operations to finish before
   sending the next key; `task test`, `task test:tui:race`, smoke test
   (isolated).

7. [ ] **Web: overlay, demo and loads** (plan §6). `src/progress.js`
   with `createShowTimer` (unit-tested), the overlay markup/CSS in
   `index.html` (rainbow, pixel cat, stars, spinner, reduced motion),
   `apiCall` passing `opts.signal`, abort handling, `#progress-demo`,
   and the queue list / message list / Load more loads as abortable
   operations with the cancelled info line.
   *Live check (Chrome, page served over http://localhost):*
   `#progress-demo` plays counted then spinner with no connection; Esc
   ends it; clicks on the page underneath do nothing while it shows;
   with "reduce motion" emulated in DevTools the bar and cat are still.
   A slow load shows the spinner; Esc cancels it.

8. [ ] **Web: purge, move all, single and batched bulk actions**
   (plan §6). `chunk` / `runBatches` (unit-tested: counts, failures
   from the response, stop between batches); Delete/Move selected in
   batches of 25 (>25 counted, ≤25 spinner); Purge / Move all and the
   detail view's single delete/move under the overlay.
   *Live check:* select 100 messages, Delete selected → bar steps
   25/50/75/100; Esc after the first batch → `Deleted 25 of 100
   message(s); cancelled`, the list and queue counts agree.

9. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke test
   (isolated), and the live checks from tasks 2, 4, 5, 7 and 8 in one
   session. Remove scratch queues; stop mq-proxy and any local page
   server.
