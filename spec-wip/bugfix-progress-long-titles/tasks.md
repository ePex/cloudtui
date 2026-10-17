# Tasks: progress modal cuts off long queue names

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`tmux -L <name>`), never against the real `~/.cloudtui`, and confirm
on the captured screen that the target is a scratch queue before every
destructive key.

1. [ ] **Break long words in `wrapLines`** (plan "Wrapping").
   `internal/dialog/progress.go` + `progress_test.go`: a word longer
   than the width is broken into width-long pieces, not cut; `maxRows`
   and its "…" stay. `TestWrapLines` gains those cases. Mutation-check.

2. [ ] **Layout and height from the content** (plan "Layout from the
   content").
   `progress.go`: `layout`, `Height(screen)`, `Draw` using it, the
   title wrapped (up to 4 lines) in both looks, detail up to 3 rows.
   Rendering tests on a `SimulationScreen`: a long title drawn in full
   with the bar below it and `Height` grown; the spinner look the same;
   an SSO detail with a long URL drawn in full on 3 rows; a short title
   giving exactly today's rows and `Height` 11. Mutation-check.

3. [ ] **Size the modal from `Height`** (plan "Sizing").
   `internal/app/operation.go` + test: a long title gives a taller,
   still centered modal; a short one 60×11.
   *Live check (isolated HOME, local broker):* two scratch queues with
   long dotted names, the source seeded with a few hundred messages.
   Move all, then marked messages, from one to the other, capturing
   the screen while the modal shows: the title in full, the modal
   grown, nothing cut. The same with `TERM=xterm` for the spinner look.

4. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke
   test (isolated). Remove the scratch setup.
