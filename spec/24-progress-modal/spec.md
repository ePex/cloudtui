# Blocking progress modal

_Condensed from spec-wip/fe-progress-modal. See that PR for the
incremental history and the reasoning behind each decision._

## Purpose

Loading and changing queues can take seconds, most of all through a
proxy connection: up to the proxy backend's ~60s retry window
(spec/11) when it's slow or unreachable. While such an operation runs,
both the TUI and the AMQ web console (spec/21) show a **blocking
progress modal**:

- a **Nyan Cat style rainbow progress bar** when the operation has a
  real count (bulk delete/move of marked or selected messages);
- otherwise a **spinner** — in the web console, the pixel cat running
  around a circle;
- **Esc** to cancel;
- a way to **preview** both looks without a broker.

It also stops a second operation from being started on top of the
first: while the modal is up, nothing else can be done.

## Behavior / user flow

### Which operations

| Operation | Where | Indicator |
|---|---|---|
| Load the queue list (open, `r`, connection switch, auto-refresh) | TUI queue list; web queue list (incl. right after connecting) | spinner |
| Load a queue's messages (open, refresh, filter) | TUI message list; web message list incl. "Load more" | spinner |
| Purge (with or without JMS Type) | TUI queue list and message list; web queue list | spinner |
| Move all (with or without JMS Type) | TUI queue list; web queue list | spinner |
| Delete / move **marked or selected** messages | TUI message list; web message list (more than 25 selected) | rainbow bar, `3 of 10` |
| Delete / move a **single** message (cursor row, detail view), and 25 or fewer selected on the web | TUI; web | spinner |

Not covered: sending a message, the JMS Type scan, the move picker's
own queue list, the export (spec/23), the web console's connect check,
and the AWS/Datadog views.

### The modal

- Centered, titled with the operation, e.g. `Deleting 10 message(s)
  from "orders"`, `Moving all messages from "orders" to "dlq"`,
  `Purging created messages from "orders"` or `Loading queues`. It sits
  on top of everything; the view underneath stays visible.
- **Blocking:** while an operation runs, every key except **Esc** is
  ignored (no view switching, no `:` prompt, no second action), and on
  the web clicks on the page are caught too. Ctrl+C still quits the
  TUI.
- **No flicker:** the modal only appears once the operation has run for
  **250 ms**, and then stays at least **400 ms**. Keys are blocked from
  the start, not only once it shows. An operation that follows straight
  on (e.g. the reload after a delete) takes the visible modal over
  instead of closing and reopening it.
- It closes when the operation finishes. The result is reported where
  it always was (TUI status line or table row; web info/error line);
  the modal adds no result screen of its own.
- The last line says `Esc to cancel`, or, once pressed on something
  that can't be cut off, what happens instead (below).

### Rainbow bar (counted operations)

- Modeled on the JetBrains "Nyan Progress Bar" plugin: a rainbow trail
  (red, orange, yellow, green, blue, purple bands) grows from the left
  with progress, with a small pixel cat (tart body, gray head and paws)
  at its head. The cat's tail and legs alternate between two frames;
  the rest of the track is a navy night sky with twinkling stars. The
  trail ends under the middle of the cat.
- Below the bar: `3 of 10`, plus `(1 failed)` when some failed. Failed
  attempts count as done, so the bar always reaches the end.
- The rainbow, cat and sky colors are fixed; the modal's border and
  text follow the theme (spec/04) or the page's light/dark palette.

### Spinner (uncounted operations, and the TUI's fallback)

- **TUI:** a braille spinner (`⠋⠙⠹…`) in the theme's accent color next
  to the title: `⠹ Loading queues…`. If the terminal reports fewer than
  256 colors, counted operations use it too, with `3 of 10` next to the
  title.
- **Web console:** the same pixel cat runs clockwise around a small
  starry disc, nose first and feet toward the middle, legs animating,
  trailing a rainbow arc that fades out behind it.

### Cancel with Esc

What can be stopped depends on what's already been sent:

- **Loads:** Esc cancels the request and closes the modal at once.
  - TUI queue list: the table shows `Loading cancelled — press r to
    retry`. TUI message list: the same row, and the list drops its
    messages and marks (they may be another queue's).
  - Web queue list: the last-loaded list comes back and the info line
    says `Loading cancelled — press Refresh to retry`. Web message
    list: the list is emptied, `… press Apply to retry`. "Load more":
    the loaded messages stay, `… press Load more to retry`.
- **Bulk delete/move of marked/selected messages:** Esc stops before
  the next step — the next message in the TUI, the next batch of 25 on
  the web. The last line changes to `Stopping after the current
  message…` / `Stopping after the current batch…`. The result then says
  what happened, worded the same in both clients:

  | Outcome | Result line |
  |---|---|
  | all done | `Deleted 10 message(s)` (`Deleted message` for one) |
  | stopped | `Deleted 3 of 10 message(s); cancelled` |
  | stopped, some failed | `Deleted 2 of 10 message(s); 1 failed; cancelled` |
  | some failed | `Deleted 9/10 message(s); 1 failed` |

  Moves say `Moved … to "queue"`. The list reloads either way.
- **Purge, move all, single delete/move:** a request that's already
  sent is never cut off. Esc changes the last line to `Stopping after
  the current request…`. A TUI purge or move-all that runs in several
  rounds (proxy backend, spec/11) stops before the next round and its
  result ends in ` (cancelled)` — unless the queue turned out empty
  anyway (below). Jolokia's purge and move-all are one call each and
  run to the end.

### Preview without a broker

- **TUI:** `:progress-demo` in the command prompt (spec/01) plays a fake
  operation: a counted run (`1 of 20` … `20 of 20`, one step every
  300 ms, every 7th "failing"), then an uncounted run of 4 s, then the
  status line says `Progress demo finished` (`Progress demo cancelled`
  after Esc). `:progress-demo spinner` forces the spinner look, to see
  the fallback in a full-color terminal. Works with any connection or
  none, in any theme; listed in the prompt's autocomplete, not on the
  Home screen.
- **Web console:** opening the page with `#progress-demo` (or setting
  that hash later) plays the same two-part demo without a connection;
  the result goes to the visible view's info line. The hash is dropped
  afterwards, so setting it again replays it.

### Web console specifics

- **Reduced motion:** with `prefers-reduced-motion: reduce`, the bar's
  cat and stars stay still (the bar still grows with progress), and the
  orbiting cat gives way to a static `Working…`.
- **Bulk delete/move in batches of 25:** selected messages are sent 25
  per `delete-messages`/`move-messages` request (spec/11's array body,
  one element per message), one request after the other. More than 25
  selected shows the counted bar advancing per batch; 25 or fewer is a
  single request with the spinner. A batch's response lists the
  messages it actually acted on (entries with a message ID); the rest
  of the batch counts as failed, and a request that fails counts its
  whole batch as failed — the remaining batches still run. The last
  error, if any, shows in the error line.

## Implementation notes

### TUI

- **`dialog.ProgressModal`** (`internal/dialog/progress.go`): a custom
  primitive, 60×11 cells (title, bar, count, two detail rows, footer).
  It decides its look from the screen it's drawn on: rainbow bar when
  `screen.Colors() >= 256`, the operation is counted and the spinner
  isn't forced; otherwise the spinner. The bar is three text rows of
  `▀` half-blocks, so six pixel rows hold the six bands. The pixel art
  comes from pure functions (`rainbowCells`, `catLeft`, `trailWidth`,
  `spinnerRune`, `progressCount`), tested on a `SimulationScreen`. The
  sky is exactly xterm color 17 (`#00005f`) so a 256-color terminal
  doesn't map it to gray. The detail rows carry the AWS SSO
  re-authentication text (spec/14) during a queue load, since the modal
  would otherwise cover the row that shows the device code.
- **`ui.Operation` / `ui.Host.StartOperation(title, total, kind)`**:
  `OpAbortable` (loads: Esc cancels `Ctx()` and closes at once) or
  `OpStoppable` (mutations: Esc cancels `Ctx()`, the modal stays until
  the caller's `Finish`). `Step(done, failed)`, `SetDetail`, `Stopped()`,
  `Finish()` (idempotent) are safe from any goroutine.
- **`app.opRunner`** (`internal/app/operation.go`): one operation at a
  time; starting one replaces the current one (its `Finish` becomes a
  no-op), and inherits the modal if it's showing. State behind a mutex;
  timers come from an injected `afterFunc` so tests run without
  sleeping; a 100 ms animation timer runs only while the modal is
  visible. The app's key handler checks for an active operation first.
  The modal never takes focus and isn't a page: the app's after-draw
  hook paints it on top, so focus is untouched when it closes.
- **Views:** `QueuesView.Load` and `MessagesView.Load` pass `op.Ctx()`
  to the backend; a superseded load can't close the newer modal. The
  bulk loop is `runEach(op, ids, do)` (checks `Stopped()` before each
  message, `Step` after each); the result line is `bulkStatus`.
  `unlessQueueEmptied`: after a stop, an unfiltered purge or move-all
  reads the queue's pending count, and at 0 reports as complete (mq-proxy
  often empties the queue in its first request, so the stop only
  skipped the check for more). A filtered one keeps `(cancelled)`, as
  does one whose count can't be read.
- **Mutations are never cut off mid-request** (both backends): every
  mutating request runs with `context.WithoutCancel(ctx)`; the caller's
  context is only checked *between* steps. Proxy: `postJSON` (every POST
  is a mutation); `inRounds` returns the total so far plus
  `context.Canceled` when stopped between rounds. Jolokia: all requests
  inside a mutation use `sendCtx`, purge checks the context before each
  fallback tier (so a stop never leads to a purge by another route), and
  the browse-and-remove fallback and the filtered delete/move loops
  check it before each message and return the count so far with the
  error. Reads keep the caller's context, which is what makes Esc on a
  load immediate.
- **`:progress-demo`** (`internal/app/demo.go`): a goroutine drives the
  two operations; the uncounted one starts before the counted one
  finishes, so it takes the modal over.

### Web console

- **`src/progress.js`**: `startOperation({ title, total, abortable,
  stopUnit })` → `{ signal, step(done, failed), stopped(), finish() }`.
  The runner tracks **several operations at once** (after a change the
  page reloads messages and queues together): the overlay shows the
  newest, stays until all have finished, and one Esc cancels them all.
  Esc aborts every operation's `AbortController` signal; abortable ones
  also close at once. The pure parts are unit-tested:
  `createShowTimer` (the 250/400 ms timing with injected timers),
  `progressCount`, `footerText`, `catSpriteSvg` (both cat frames as one
  SVG, built from the same pixel grid as the TUI's cat), `wait`.
  Keys are swallowed in the capture phase on `document`; the overlay
  (`position: fixed`) catches clicks from the start, invisibly until
  shown.
- **`index.html`**: the bar is one six-stop hard-stop `linear-gradient`
  whose width follows `--p`; stars are drifting radial gradients. The
  orbiting cat is a 96 px starry disc with one element rotating
  (~1.2 s per lap): the six bands as concentric rings cut by a
  `conic-gradient` mask to the arc behind the cat, and the cat sprite
  at 3 px per pixel at 12 o'clock. Only `transform` and
  `background-position` animate, and only while the overlay is shown.
- **`src/api.js`**: `apiCall` passes `opts.signal` to `fetch`; an abort
  rejects with an `AbortError` (`isAbortError`), also when it lands
  while the body is read, so a cancelled request never resolves as an
  empty success. `chunk`, `runBatches` (sequential batches, failures
  from the response, stop before the next batch) and `bulkSummary` (the
  result wording) are pure and unit-tested.

## Out of scope (deliberate)

- Progress counts inside a purge or move-all (the broker reports none).
- The modal for the operations listed under "Not covered".
- A setting to turn the modal off or choose the style.
- An orbiting cat in the TUI: a circle doesn't fit in a few character
  cells at a size where the cat stays recognizable.
- Sound, or the cat's music.
