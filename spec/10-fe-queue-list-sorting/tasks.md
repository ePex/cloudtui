# Tasks — FE 10: Queue list column sorting

Plan: [plan.md](plan.md)

1. [x] **Sort state + dynamic header** — add `sortCol int` and `sortAsc bool`
   fields (defaults 0/true) to `queuesView`; extract `queueColumns []string`
   slice; rewrite `setHeader()` to build labels dynamically with `▲`/`▼`
   on the active column.

2. [x] **Sort logic + hotkeys** — replace hardcoded name sort in `repaint`
   with `sort.SliceStable` driven by `sortCol`/`sortAsc` with name as
   tiebreaker for equal values; call `setHeader()` at end of `repaint`; add
   `o` (cycle column) and `O` (toggle direction) to `SetInputCapture`; update
   `Shortcuts()` with combined `o/O` entry.

3. [x] **Tests** — add to `queues_test.go`: sort by pending descending;
   direction toggle changes row order; header marker on active column only;
   `o/O` combined shortcut present.

4. [x] **Manual verification** — press `o` repeatedly and confirm sort column
   cycles with marker; press `O` to flip direction; navigate away and back;
   confirm sort persists.
