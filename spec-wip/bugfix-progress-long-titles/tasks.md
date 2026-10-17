# Tasks: progress modal cuts off long queue names

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`tmux -L <name>`), never against the real `~/.cloudtui`, and confirm
on the captured screen that the target is a scratch queue before every
destructive key.

1. [x] **Break long words in `wrapLines`** (plan "Wrapping").
   `internal/dialog/progress.go` + `progress_test.go`: a word longer
   than the width is broken into width-long pieces, not cut; `maxRows`
   and its "…" stay. `TestWrapLines` gains those cases. Mutation-check.
   *Done (2026-10-17):* a word longer than a row starts a new row and
   is broken into width-long pieces; the last piece continues as the
   current line, so the next word can follow it. The old per-row cut
   (`…` on an overlong row) is gone, since no row can be overlong any
   more. One existing case changed its expectation:
   `wrapLines("abcdefghijkl", 5, 2)` was `["abcd…"]` and is now
   `["abcde", "fghi…"]`. New cases: a word broken over three rows, an
   exact multiple of the width, a long word after a short one (starts
   a new row), a quoted queue name, the "…" beyond `maxRows`, and
   multi-byte runes. Mutation-checked, each failing `TestWrapLines`:
   the word cut again; pieces one rune short; the word continuing the
   current line; byte length instead of rune length. Two first
   attempts didn't count: a guard against an empty last row and a
   `len(runes) == 0` check survived because they can't trigger (the
   loop always leaves the word's last piece), so both were removed;
   and a first "bytes" mutation was a no-op, redone. `go test ./...`,
   gofmt and vet pass.

2. [x] **Layout and height from the content** (plan "Layout from the
   content").
   `progress.go`: `layout`, `Height(screen)`, `Draw` using it, the
   title wrapped (up to 4 lines) in both looks, detail up to 3 rows.
   Rendering tests on a `SimulationScreen`: a long title drawn in full
   with the bar below it and `Height` grown; the spinner look the same;
   an SSO detail with a long URL drawn in full on 3 rows; a short title
   giving exactly today's rows and `Height` 11. Mutation-check.
   *Done (2026-10-17):* `progressLayout` holds the wrapped title (in the
   spinner look the whole spinner line: spinner rune, title, "…" and
   count, wrapped together so the spinner stays first) and detail;
   `Height` and `Draw` both use it. The test helper `drawModal` now
   draws at the modal's own `Height` (still 60×11 for every existing
   test). Tests: `…WrapsALongTitle` (two long dotted queue names: the
   title's text in full over its lines, no bar among them, the bar,
   count, detail and footer each on its row below, `Height` 11 +
   extra lines), `…WrapsALongTitleInTheSpinnerLook`,
   `…CutsATitleLongerThanFourLines`, `…WrapsTheDetail` (rewritten: the
   SSO message in the app's format, `… Verify code ABCD-EFGH at
   https://device.sso.eu-central-1.amazonaws.com/`, in full on 3 rows,
   modal 12 rows; it used to expect the cut "…"),
   `…CutsADetailLongerThanThreeRows`, `…ShortTitleKeepsItsLayout`. A
   URL longer than a row (56) starts its own row, so a message with
   one can still need a fourth row and end in "…"; the device URL the
   CLI prints is shorter. Mutation-checked, each failing at least one
   test: the title on one line (rainbow; spinner); detail capped at
   two rows; no minimum detail rows; no title line limit; `Height`
   constant; the bar, count, detail or footer at today's fixed rows;
   only the first spinner line drawn. The count, detail and footer
   positions first survived (the long-title test only looked for the
   text anywhere); the test now checks each row. `go test ./...`,
   gofmt and vet pass. The app still sizes the modal 60×11 (task 3).

3. [x] **Size the modal from `Height`** (plan "Sizing").
   `internal/app/operation.go` + test: a long title gives a taller,
   still centered modal; a short one 60×11.
   *Live check (isolated HOME, local broker):* two scratch queues with
   long dotted names, the source seeded with a few hundred messages.
   Move all, then marked messages, from one to the other, capturing
   the screen while the modal shows: the title in full, the modal
   grown, nothing cut. The same with `TERM=xterm` for the spinner look.
   *Done (2026-10-17):* `opRunner.draw` sizes the modal
   `min(m.Height(screen), h)`. Tests: `TestModalGrowsForALongTitle`
   (taller than 11, centered on 24 rows, both queue names drawn in
   full; a short title stays 60×11) and `TestModalIsClippedToAShortScreen`
   (an 8-row screen: the modal takes exactly those 8 rows). Mutation-
   checked: the fixed height again fails the first; no clipping
   survived until the short-screen test was added, then fails it.
   Live (scratch HOME, own tmux server, local broker): queues
   `scratch.progresscheck.orders.fulfilment.backend.eu` and
   `dlq.scratch.progresscheck.orders.fulfilment.backend.eu`, the source
   seeded with 400 messages; every destructive step confirmed on screen
   (the picker filtered to the one scratch target). Moving all 400
   marked messages shows the modal for about a second; the screen was
   captured every 0.1 s. Rainbow look (`TERM=xterm-256color`): "Moving
   400 message(s) from" / both quoted names on their own lines, the bar
   below, `333 of 400`, the modal 13 rows. Spinner look: `TERM=xterm`
   alone still gave the rainbow, because the shell's
   `COLORTERM=truecolor` makes tcell report full color; with
   `COLORTERM` unset and `TERM=xterm-16color`: `⠼ Moving 400 message(s)
   from`, both names in full, the "…" after the second, the count
   wrapped to its own line, the modal 14 rows. The first move (a plain
   move-all, no confirmation step) finished before a capture was
   running; the moves back and forth after it were captured. Scratch
   queues removed. `go test ./...`, gofmt and vet pass.

4. [x] **Regression run.** `task test`, `task test:tui:race`, smoke
   test (isolated). Remove the scratch setup.
   *Done (2026-10-17):* `task test` (Go, mq-proxy, 78/78 web tests)
   and `task test:tui:race` pass. The smoke test passes against the
   local broker with a scratch HOME and its own tmux server (a
   `tmux -L` wrapper on `PATH`); the scratch HOME was unchanged
   afterwards. No smoke or scratch queues left, nothing on 8080, no
   scratch processes; scratch setup removed.
