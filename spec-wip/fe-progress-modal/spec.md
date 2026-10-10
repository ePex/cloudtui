# Feature: blocking progress modal for slow queue operations

Date: 2026-10-10

## What and why

Loading and changing queues can take seconds, most of all through a
proxy connection: up to the proxy backend's ~60s retry window
(spec/11) when it's slow or unreachable. Today the user gets little
feedback. The queue list shows a "Loading queues…" row; delete, move
and purge show nothing until the result lands in the status line. In
the meantime other keys still work, so it's easy to start a second
operation on top of the first.

This feature adds a **blocking progress modal**, shown while those
operations run, in both the TUI and the web console (spec/21):

- a **Nyan Cat style rainbow progress bar** when the operation has a
  real count (bulk delete/move of selected messages: the TUI sends one
  request per message, the web console one per batch of 25, so
  "3 of 10" is honest);
- a **spinner** when it has no count (one opaque request, or a load),
  and as the fallback wherever the rainbow bar can't be drawn;
- **Esc** to cancel;
- a way to **preview** both looks without a broker.

## Which operations

| Operation | Where | Indicator |
|---|---|---|
| Load the queue list (open, `r`, connection switch, auto-refresh) | TUI queue list; web queue list | spinner |
| Load a queue's messages (open, refresh, filter) | TUI message list; web message list incl. "Load more" | spinner |
| Purge (with or without JMS Type) | TUI queue list and message list; web queue list | spinner |
| Move all (with or without JMS Type) | TUI queue list; web queue list | spinner |
| Delete / move **marked or selected** messages | TUI message list; web message list (more than 25 selected — see below) | rainbow bar, `3 of 10` |
| Delete / move a **single** message (cursor row, detail view) | TUI; web | spinner |

Not covered: sending a message, the JMS Type scan, the move picker's
own queue list, the export (spec/23), and the AWS/Datadog views.

## The modal

- Centered, titled with the operation, e.g. `Deleting 10 message(s)
  from "orders"` or `Loading queues`. It sits on top of everything,
  including the view underneath, which stays visible.
- **Blocking:** while it is up, every key except **Esc** is ignored (no
  view switching, no `:` prompt, no second action). Ctrl+C still quits
  the app.
- **No flicker on fast calls:** the modal only appears once the
  operation has run for **250 ms**. Once shown, it stays up for at
  least **400 ms**, so it doesn't flash. Keys are blocked from the
  start, not only once the modal shows.
- It closes when the operation finishes. The result is reported the way
  it is today (status line, view reload, error row); the modal adds no
  result screen of its own.
- A last line says `Esc to cancel`, or what Esc will do (below).

### Rainbow bar (counted operations)

- Modeled on the JetBrains "Nyan Progress Bar" plugin: a rainbow trail
  (red, orange, yellow, green, blue, purple bands) grows from the left
  as the operation progresses, with a small cat drawn at its head. The
  cat has a two-frame animation, and the remaining track shows
  twinkling stars on a dark background.
- TUI: drawn with half-block characters, so six bands fit in three text
  rows; the cat is a few cells of half-block "pixels". The rainbow and
  cat colors are fixed (they don't follow the theme); the modal's
  border and text do follow the theme (spec/04).
- Below the bar: `3 of 10` plus, when some failed, `(1 failed)`. Failed
  attempts count as done, so the bar always reaches the end.

### Spinner (uncounted operations, and the fallback)

- An animated spinner in the theme's accent color plus the title, e.g.
  `⠹ Loading queues…`.
- **Fallback:** if the terminal reports fewer than 256 colors, the
  counted operations use the spinner too, with `3 of 10` next to it.

### Cancel with Esc

What can be stopped depends on what's already been sent:

- **Loads** (queues, messages): Esc abandons the load at once; the
  request is cancelled. The view keeps whatever it showed before and
  says `Loading cancelled — press r to retry` where the data would be
  (TUI: the table row; web: the view's info line).
- **Bulk delete/move:** Esc stops after the message currently in
  flight. The modal's last line changes to `Stopping after the current
  message…`. The status line then reports what happened, e.g.
  `Deleted 3 of 10 message(s); cancelled`, and the list reloads.
- **Purge, move all, single delete/move:** a single request that's
  already sent can't be taken back. Esc switches the last line to
  `Stopping after the current request…`; a purge or move-all that runs
  in several rounds (proxy fallback, spec/09) stops before the next
  round. The result says `(cancelled)` if anything was skipped.

## Preview without a broker

- **TUI:** `:progress-demo` in the command prompt opens the modal with a
  fake operation. It works with any connection, or none, and with any
  theme. First a counted run (`1 of 20` … `20 of 20`, about 6 s), then
  an uncounted spinner run (about 4 s), then it closes. Esc ends it
  early. `:progress-demo spinner` forces the fallback look, so it can be
  checked in a full-color terminal. Like the other `:` commands it shows
  up in the prompt's autocomplete; it isn't listed on the Home screen.
- **Web console:** opening the page with `#progress-demo` (e.g.
  `mq-console.html#progress-demo`) runs the same two-part demo without a
  connection.

## Web console specifics

- The same overlay, built with HTML/CSS (no new dependency): a rainbow
  bar with a small pixel cat, or a CSS spinner; the same 250 ms / 400 ms
  timing, title, count line and Esc behavior. It blocks clicks on the
  page underneath. The animation is plain CSS and runs only while the
  overlay is visible.
- **Reduced motion:** when the OS asks for reduced motion
  (`prefers-reduced-motion`), the bar and cat stay still (the bar still
  grows with progress) and the spinner becomes a static `Working…`.
- **Bulk delete/move in batches:** today the console sends a bulk
  delete/move as **one** request listing every selected message
  (spec/21), so there's nothing to count. It now sends them in
  **batches of 25** messages per request, one batch after the other,
  using the same request shape. That keeps the request count low
  (500 messages = 20 requests; mq-proxy checks Basic auth on every
  request) while giving real progress:
  - the bar and count advance per batch: `25 of 100`, `50 of 100`, …;
  - a batch's response says which of its messages were actually deleted
    or moved, so failures are counted exactly: `(3 failed)`;
  - Esc stops before the next batch: `Deleted 50 of 100 message(s);
    cancelled`;
  - 25 or fewer selected is a single request, shown with the spinner
    (there's no step to show).
- Loads use `fetch` with an abort signal, so Esc really cancels them.

## Out of scope

- Progress counts inside a purge or move-all (the broker reports no
  progress).
- Showing the modal for the operations listed under "Not covered".
- A setting to turn the modal off or choose the style.
- Sound, or the cat's music.
