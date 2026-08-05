# CR 06 — Queue list: extended columns, colors, sorting, and navigation

Date: 2026-08-05

## What and why

The initial queue list (feature 04) shows only Name, Pending, and Consumers
with no visual distinction between healthy and unhealthy states. This change
extends it to match the richer view shown in the reference screenshot:

- **Two new columns** — EnqueueCount and DequeueCount (throughput indicators).
- **Color-coded metrics** — Pending > 0 is highlighted in accent color (warning:
  messages are stuck); Consumers = 0 is highlighted in accent color (warning:
  nobody is consuming); all other values are shown in the normal text color.
- **Styled header row** — distinct background (label/accent color) with dark
  foreground text and a sort indicator on the active column.
- **Selectable rows** — cursor navigation with ↑/↓ (and j/k); selected row
  shown in the palette selection colors.
- **Default sort** — queues sorted ascending by name on load; sort indicator
  (▲/▼) shown in the NAME header.

## Columns

| # | Header    | Source field     | Color rule |
|---|-----------|------------------|------------|
| 0 | NAME ▲    | Summary.Name     | label color |
| 1 | PENDING   | Summary.PendingCount  | accent if > 0, else text |
| 2 | CONSUMERS | Summary.ConsumerCount | accent if = 0, else text |
| 3 | ENQUEUED  | Summary.EnqueueCount  | text color |
| 4 | DEQUEUED  | Summary.DequeueCount  | text color |

## Scope

**In scope:**
- Extend `queue.Summary` with `EnqueueCount int64` and `DequeueCount int64`.
- Extend Jolokia client bulk read to fetch `EnqueueCount` and `DequeueCount`.
- Rewrite `queuesView` repaint: 5-column table, styled header row, per-cell
  color logic, selectable rows, default sort by name.
- Update all tests.

**Out of scope:**
- Interactive sort (clicking headers to change sort column/direction).
- Filtering.
- Actions on a selected queue (purge, move, etc.).
- Pagination.
