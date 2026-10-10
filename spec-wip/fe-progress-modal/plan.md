# Plan: blocking progress modal

Implements `spec.md` in this folder. TUI first (sections 1–5), then the
web console (section 6).

## 1. TUI: the modal primitive — `internal/dialog/progress.go`

`ProgressModal`, a custom `tview` primitive (embeds `*tview.Box`, own
`Draw`), so the bar can be painted cell by cell:

- State: `title`, `done`, `total` (0 = uncounted), `failed`, `footer`
  (`Esc to cancel` / `Stopping after the current …`), `detail` (an extra
  line, used for the AWS SSO text, below), `frame` (animation step),
  `forceSpinner`.
- `Draw(screen)` decides the look **from the screen it's drawn on**:
  `screen.Colors() >= 256 && total > 0 && !forceSpinner` → rainbow bar,
  otherwise the spinner. No terminal detection elsewhere, and it's
  testable with a `tcell.SimulationScreen` (which can be set to 16
  colors).
- The pixel art is computed by **pure functions** returning a grid of
  cells (`rune`, fg, bg), drawn by `Draw` in one loop:
  - `rainbowCells(width, done, total, frame)` — 3 text rows holding 6
    color bands via `▀` (fg = upper band, bg = lower band); the trail
    covers `width*done/total` columns; stars (`·`, `✦`) on the rest,
    their positions shifting with `frame`.
  - the cat — 10×3 cells of half-block pixels, two frames (tail and
    legs alternate) — is painted by `rainbowCells` too, with its head at
    the trail's end (`catLeft`), or at the bar's start while the trail
    is shorter than the cat.
  - `spinnerRune(frame)` — the braille spinner `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`.
- Colors: rainbow and cat are fixed `tcell.NewRGBColor` values (the
  terminal maps them to 256 colors); border, title, text and spinner use
  the theme (`ui.Themeable`, `ApplyTheme`) like the other dialogs.
- Fixed size: 60×11 cells (title, bar, count, two detail rows for the
  AWS SSO text, footer), centered via `ui.Centered`.

## 2. TUI: running operations — `ui.Host.StartOperation`

New in `internal/ui`:

```go
type OpKind int
const (
    OpAbortable OpKind = iota // loads: Esc cancels Ctx() and closes at once
    OpStoppable               // mutations: Esc cancels Ctx(), the modal
                              // stays until the caller calls Finish
)

type Operation interface {
    Ctx() context.Context      // cancelled by Esc (both kinds) or Finish
    Stopped() bool             // Ctx().Err() != nil
    Step(done, failed int)     // counted progress; safe from any goroutine
    SetDetail(text string)     // extra line (AWS SSO text)
    Finish()                   // closes; idempotent
}

// On ui.Host:
StartOperation(title string, total int, kind OpKind) Operation
```

`internal/app/operation.go` implements it on `*App`:

- **One operation at a time.** Starting one while another runs replaces
  it (the older one's `Finish` becomes a no-op, checked by an operation
  id). That's what a superseded queue reload needs (spec/07 already
  discards the older result).
- **Timing:** a 250 ms timer shows the page; `Finish` hides it, but no
  sooner than 400 ms after it was shown (a second timer). Timers come
  from an injected `afterFunc` (default `time.AfterFunc`) so tests drive
  them without sleeping. The runner's state sits behind a mutex, so
  operations can report from any goroutine; it asks for redraws with
  `go QueueUpdateDraw(…)`, because `QueueUpdateDraw` blocks until the
  UI goroutine runs it and would deadlock when called from there.
- **Animation:** a 100 ms ticker runs only while the modal is visible,
  advancing `frame` and redrawing.
- **Blocking keys:** `onGlobalKey` checks for an active operation
  **first**, from the moment it starts (not only once visible): Esc is
  handled (abortable → cancel ctx, finish; stoppable → cancel ctx,
  change footer, wait for the caller's `Finish`), Ctrl+C is passed through so tview still quits, every
  other key is swallowed. The modal never takes focus, so the view
  underneath keeps its cursor and focus for when it closes.
- The modal is **not** a page: `tview.Pages.ShowPage` moves focus to the
  newly visible page, and hiding it wouldn't put focus back where it
  was. Instead the app's existing after-draw hook (`SetAfterDrawFunc`,
  shared with the clipboard's screen capture) draws it centered on top
  of everything once the normal draw is done.

## 3. TUI: wiring the operations

| Where | Kind | Total | On Esc |
|---|---|---|---|
| `QueuesView.Load` (`queues.go`) | abortable | 0 | discard result, row `Loading cancelled — press r to retry` |
| `MessagesView.Load` (`messages.go`) | abortable | 0 | same row in the messages table |
| `purgeFlow.run` (`purge.go`) | stoppable | 0 | result line gets `(cancelled)` if the backend stopped early |
| `QueuesView` move-all | stoppable | 0 | as purge |
| `deleteMarked` / `moveMarked` | stoppable | n (marked) or 0 (cursor row) | loop checks `Stopped()` before each message; `Step` after each; `Deleted 3 of 10 message(s); cancelled` |
| `MessageDetailView` delete/move | stoppable | 0 | nothing to skip; just finishes |

- Each view calls `Finish` **before** its existing result handling
  (status line, reload, focus), inside the same `QueueUpdateDraw`.
- Loads pass `op.Ctx()` to the backend, so Esc really cancels the
  request. Mutations use `op.Ctx()` only to decide whether to start the
  next step: the bulk loop checks `Stopped()`, and purge/move-all hand
  it to the backend, where only the proxy's between-rounds check reacts
  (section 4). Esc never aborts a mutating request mid-flight and loses
  its count.
- **AWS SSO re-auth during a queue load:** `QueuesView`'s
  `ReauthStatusShower` updates also call `op.SetDetail(text)`, because
  the modal would otherwise cover the row that shows the device code.
- Test doubles: `hosttest_test.go` (dialog) and `testfake_test.go`
  (view) get a fake `StartOperation` that records title/total/steps
  and lets tests press "Esc" (cancel/stop) at a chosen point.

## 4. Mutations are never cut off mid-request — both backends

Purge and move-all on the proxy backend run in rounds (`inRounds`),
checking `ctx.Err()` between rounds, but each round's HTTP request uses
the same `ctx`, so cancelling would abort a round mid-flight. Change:
the round requests use `context.WithoutCancel(ctx)`; only the
between-rounds check sees the cancel, and `inRounds` returns the total
so far plus `context.Canceled`. The purge/move-all flows pass
`op.Ctx()` and report `(cancelled)` for `context.Canceled`.

The same rule for every mutating call in both backends (purge,
move-all, remove, move, the filtered delete/move): the HTTP request
itself runs with `context.WithoutCancel(ctx)`, so a cancelled context
never aborts a mutation that's already been sent. Jolokia's purge and
move-all are a single JMX call each, so there Esc can't skip anything —
they run to the end and report normally. Reads (`List`,
`BrowseMessages`) keep using the caller's context unchanged, which is
what makes Esc on a load immediate.

## 5. `:progress-demo` — `internal/app/promptcommands.go`

Two table entries: `progress-demo` and `progress-demo spinner`. Both
start a fake operation (`demo.go`): a goroutine steps a counted
operation `1 … 20` over ~6 s, then an uncounted one for ~4 s, honoring
Esc. The `spinner` form sets `forceSpinner`. No backend involved, so it
works with no connection.

## 6. Web console

- **`src/progress.js`** (new):
  - `startOperation({ title, total, abortable })` → `{ signal, step(done,
    failed), stopped(), finish() }`. `signal` is an `AbortController`
    signal, aborted by Esc for abortable operations.
  - The 250/400 ms timing is a small pure helper,
    `createShowTimer(setTimeoutFn, clearTimeoutFn, now)`, unit-tested
    with fake timers.
  - Esc via a `keydown` listener on `document` while an operation runs;
    the overlay (`position: fixed`, full-page backdrop) swallows clicks.
- **`index.html`**: the overlay markup (title, bar, spinner, count line,
  footer) and its CSS:
  - rainbow: one element with a six-stop `linear-gradient` (hard stops
    = bands), its width set from progress;
  - cat: an inline SVG pixel cat as a `data:` URI, two frames swapped by
    a `steps(2)` animation; stars: a repeating radial-gradient
    background drifting via `background-position`;
  - spinner: a CSS border spinner;
  - `@media (prefers-reduced-motion: reduce)` turns the animations off
    and swaps the spinner for the static `Working…` text.
  Only transform/background-position animate, and only while the
  overlay is shown.
- **`src/api.js`**: `apiCall` passes `opts.signal` to `fetch`; an
  aborted call rejects with an `AbortError` that callers treat as
  "cancelled", not as an error.
- **Batches** (`src/api.js`, pure, unit-tested): `chunk(ids, 25)` and
  `runBatches(apiCallFn, items, buildBody, verb, { onStep, stopped })`
  → `{ done, failed, cancelled }`. Each batch's response array lists the
  messages that were actually deleted/moved (entries with a message ID,
  spec/11); `failed` = batch size − entries reported.
- **Wiring:** `loadQueues`, `loadMessages`, `loadMoreMessages`
  (abortable, spinner), queue-row Purge / Move all (stoppable — a single
  request, so Esc only changes the footer), single delete/move in the
  detail view, and Delete/Move selected (`runBatches`; >25 counted,
  ≤25 spinner). Cancelled loads show `Loading cancelled — press Refresh
  to retry` / `… press Apply to retry` in the view's info line.
- **`#progress-demo`** (`src/main.js`): when the page opens with that
  hash, run the same counted-then-spinner demo with fake steps, no
  connection needed.

## Testing

- **Rendering:** `SimulationScreen` tests — 256 colors draws `▀` cells
  in band colors and the cat at the head for 0 %, 50 %, 100 %; 16
  colors or `total == 0` draws the spinner (`⠹ <title>…`) and `3 of 10`;
  the footer and detail lines appear.
- **Operation runner:** fake `afterFunc` — hidden before 250 ms, shown
  after; `Finish` at 300 ms keeps it until 650 ms; a superseded
  operation's `Finish` doesn't close the newer one; keys swallowed
  while active, Esc cancels (abortable) or only stops (stoppable),
  Ctrl+C passes.
- **Views:** loads honor cancel and show the retry row; bulk loop stops
  after the current message with the right status; `Step` counts
  include failures; purge/move-all report `(cancelled)`.
- **Proxy:** `inRounds` stops between rounds on cancel without
  cancelling the round in flight (an `httptest` server that blocks
  until the test cancels, then answers).
- **Web:** `node --test` for `createShowTimer`, `chunk`, `runBatches`
  (counts, failures, stop between batches), abort handling in `apiCall`.
  The overlay DOM and CSS animation are checked live (not unit-testable
  under `node --test`).
- **Live:** the `:progress-demo` and `#progress-demo` checks, then real
  slow operations: run mq-proxy and pause/slow it, or use the
  1,600-message queue, against both backends.
- **Smoke test:** the script sends keys while loads run; with keys now
  swallowed during operations, it must wait for each operation to end
  (it already waits for screen text — check and adjust).

## Risks / trade-offs

- Blocking keys means a hung request blocks the UI until it times out
  or the user presses Esc. That's the point of Esc; mutations can't be
  aborted mid-request, only stopped between requests, which is the
  honest limit.
- The cat is an original small pixel drawing in the familiar style (a
  cat with a rainbow trail), not a copy of the plugin's artwork.
