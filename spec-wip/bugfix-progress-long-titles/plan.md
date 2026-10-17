# Plan: progress modal cuts off long queue names

Implements `spec.md` in this folder.

## Where the text is cut today

- `dialog.ProgressModal.Draw` (`internal/dialog/progress.go`) lays out
  fixed rows inside the border (58 columns, text 56 wide): title on row
  0, bar on rows 2–4, count on row 5, detail on rows 6–7, footer on row
  8. The title (and in the spinner look, `⠹ <title>…  3 of 10` on row
  3) goes through `tview.Print` once, which stops at the edge.
- `wrapLines(text, width, maxRows)` wraps the detail at spaces; a word
  longer than `width` is cut to `width-1` characters plus "…".
- `opRunner.draw` (`internal/app/operation.go`) sizes the modal as
  `min(ProgressModalWidth, w) × min(ProgressModalHeight, h)`, centered.

## Approach

### Wrapping (`wrapLines`)

A word longer than `width` is broken into `width`-long pieces (the
last piece starting the next line's text as usual) instead of cut. The
`maxRows` limit and its "…" on the last row stay. Runes, not bytes, as
today.

### Layout from the content

New constants: `progressTitleMaxRows = 4`, `progressDetailMinRows = 2`,
`progressDetailMaxRows = 3`. `ProgressModalHeight` (11) stays as the
height for a one-line title and short detail, so a modal for an
ordinary operation looks exactly as before.

A new `layout(rainbow bool) progressLayout` computes, for the modal's
current text and its fixed text width (`ProgressModalWidth - 4`):

- **rainbow look:** `titleLines` = `wrapLines(title, …, 4)`.
- **spinner look:** `titleLines` = `wrapLines(title + "…" + "  " +
  count, …, 4)`, the first line two columns shorter for the spinner
  and its space (the spinner is drawn in front of line 1, in the accent
  color, as today).
- `detailLines` = `wrapLines(detail, …, 3)`; the modal keeps room for
  at least 2 detail rows even when there are fewer (as today, so the
  modal doesn't jump when the SSO text appears), 3 when needed.

Rows inside the border, with `t` title lines and `d` detail rows:

- **rainbow:** title on rows `0…t-1`, a blank row, the bar on rows
  `t+1…t+3`, the count on `t+4`, detail on `t+5…t+4+d`, footer on
  `t+5+d`. With `t = 1, d = 2` that's today's layout exactly.
- **spinner:** the title block sits where the title, bar and count
  would be (`t+5` rows), centered vertically; with one line that's row
  3, as today.

Inner height `t + 6 + d`, so `Height(screen) int` = that plus 2 for the
border (rainbow vs spinner decided by the existing `rainbow(screen)`).
`Draw` uses the same `layout` so the two can't disagree.

### Sizing (`opRunner.draw`)

`height := min(m.Height(screen), h)` instead of the constant;
centering is unchanged. On a short terminal the modal is clipped as
today (`Draw` already skips rows beyond its height).

`SetDetail`'s doc comment changes from "up to two rows" to three.

## Files

- `tui/internal/dialog/progress.go` (+ `progress_test.go`): the
  wrapping, `layout`, `Height`, `Draw`.
- `tui/internal/app/operation.go` (+ test): size from `Height`.
- Merge-back: spec/24 (the modal's layout, "60×11" becomes "60 wide,
  11 rows for a one-line title").

## Testing

- `TestWrapLines` gains: a word longer than the width is broken across
  lines, not cut; a broken word still respects `maxRows` and its "…".
- Rendering on a `SimulationScreen` (existing helpers in
  `progress_test.go`):
  - a long title with two long queue names: every character of the
    title appears in the drawn rows (joined in order), on several
    lines, none past the border; the bar starts below the last title
    line; `Height` is 11 + (lines − 1);
  - the same in the spinner look: the spinner on line 1, the whole
    `title…  count` text drawn;
  - an SSO detail with a code and a long URL: the full URL appears, on
    3 detail rows;
  - a short title: the exact same rows as before (bar on rows 2–4,
    count on 5, footer on 8) and `Height` 11.
- `opRunner` test: with a long title the modal's rect is taller than
  11 and still centered; with a short title it's 60×11 as before.
- Mutation checks: cutting long words again; a fixed height in
  `opRunner.draw`; the title printed on one line; detail capped at 2.
- Live (tmux, isolated HOME, local broker): two scratch queues with
  long dotted names, the source seeded with enough messages (a few
  hundred) that a move takes longer than the modal's 250 ms delay
  (spec/24) on the local broker. Move all, then marked messages, from
  one to the other, capturing the screen while the modal shows: each
  title fully visible, the modal grown, nothing cut. The spinner look
  the same way in a terminal reporting fewer than 256 colors
  (`TERM=xterm` in the tmux session). Operations too fast to show the
  modal locally (a single delete, an empty purge) and the SSO detail
  (needs AWS) are covered by the rendering tests.
