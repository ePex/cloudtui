# FE 10 — Queue list column sorting

Date: 2026-08-06

## What and why

The queue list is currently always sorted ascending by name. Users need to
sort by any column (e.g. pending count descending to find the most-loaded
queues) without leaving the view.

## User flow

1. User is in the Queues view. The active sort column is indicated by a
   `▲` (ascending) or `▼` (descending) marker in the header cell.
2. User presses **`o`** — the sort column cycles to the next column
   (NAME → PENDING → CONSUMERS → ENQUEUED → DEQUEUED → NAME → …).
   The table re-sorts immediately.
3. User presses **`O`** (Shift+o) — the sort direction toggles between
   ascending and descending. The table re-sorts immediately.
4. The sort state persists when the user navigates away and returns
   (same as the filter).

## Scope

**In scope:**
- `o` cycles the active sort column through all 5 columns.
- `O` (Shift+o) toggles sort direction.
- Active sort column and direction shown in header via `▲`/`▼` marker.
- Sort state (`sortCol int`, `sortAsc bool`) stored on `queuesView`;
  reapplied on every `repaint`.
- Default: sort by NAME ascending (current behaviour preserved).
- Shortcuts updated: `o/O` shown as combined entry `sort col/dir`.

**Out of scope:**
- Multi-column sorting.
- Sorting in the messages view.
