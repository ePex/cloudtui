# Tasks — CR 06: Queue list extended columns, colors, sorting, navigation

Plan: [plan.md](plan.md)

1. [x] **Extend `Summary`** — add `EnqueueCount int64` and `DequeueCount int64`
   to `queue.Summary` in `backend.go`.

2. [x] **Extend Jolokia client** — fetch `EnqueueCount` and `DequeueCount` in
   the bulk read (4 attributes per queue instead of 2); decode the larger
   response slice; update `jolokia_test.go` happy-path to supply 4 values per
   queue and assert the new fields.

3. [x] **Rewrite queues view repaint** — 5-column table; styled non-selectable
   header row (label-color background, background-color foreground); data rows
   selectable and sorted ascending by name; per-cell color logic (Pending accent
   if > 0; Consumers accent if = 0; Name in value color; Enqueued/Dequeued in
   text color); j/k forwarded to ↑/↓ via `SetInputCapture`.

4. [x] **Update queues tests** — update `TestQueuesViewHeaderLabels` (5 headers,
   NAME has ▲), `TestQueuesViewColumnCount` (5); add
   `TestQueuesViewRepaintSortsAlphabetically`,
   `TestQueuesViewPendingAccentWhenNonZero`,
   `TestQueuesViewConsumerAccentWhenZero`.

5. [x] **Manual verification** — run broker + TUI; confirm 5 columns with styled
   header, correct warning colors, alphabetical sort, cursor navigation with
   ↑/↓/j/k, and correct repaint on theme switch.
