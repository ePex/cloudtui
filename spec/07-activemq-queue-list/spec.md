# ActiveMQ queue list

_Condensed from spec/04, spec/06, spec/09, spec/10, spec/11 — see those
folders for the incremental history. This document describes only the
final, current behavior of the Queues view._

## Purpose

A read-only-by-default **Queues** view listing every queue on the active
ActiveMQ broker, reachable via the Home dashboard (spec/05) and via
`:queues`. This is the tool's primary screen — the entry point into browsing
(spec/08) and acting on (spec/09) queue messages.

## Behavior

- **Columns** (5, in order): **NAME**, **PENDING**, **CONSUMERS**,
  **ENQUEUED**, **DEQUEUED** (enqueue/dequeue counts are cumulative
  throughput indicators, not current state).
- **Color coding**: PENDING > 0 and CONSUMERS = 0 are both rendered in the
  palette's accent/warning color (stuck messages / nobody consuming);
  every other cell uses the normal text color. The header row has a
  distinct background (label/accent color) with dark foreground text.
- **Row selection**: cursor navigation with ↑/↓ and j/k; selected row shown
  in the palette's selection colors. Enter on a row opens the Messages view
  for that queue (spec/08).
- **Sorting**: `o` cycles the active sort column through all 5 columns in
  order (NAME → PENDING → CONSUMERS → ENQUEUED → DEQUEUED → NAME → …);
  `O` (Shift+o) toggles ascending/descending. The active column's header
  shows a `▲`/`▼` marker. Default: NAME ascending. Sort state
  (`sortCol int`, `sortAsc bool`) persists across navigating away and back,
  reapplied on every repaint.
- **Filtering**: `/` opens an inline `tview.InputField` labeled
  `" / filter: "`; typing filters rows by case-insensitive substring match
  on queue name, live on every keystroke. Enter/Esc closes the input and
  keeps the filter applied. The table title reflects the active filter
  (`" Queues "` → `" Queues (foo) "`); an empty confirmed filter clears it
  and restores the plain title. The filter string persists across
  navigation and is reapplied after every data reload — same persistence
  model as sort.
- **Scroll-to-top on repaint**: every repaint (initial load, filter change,
  sort change, or re-entering the view) resets the table selection to the
  first data row. Scroll position is deliberately *not* preserved across
  navigations — this is intentional UX, not a gap.
- **Reload**: `r` reloads the queue list from the active backend
  (Jolokia or mq-proxy, spec/11).
- **Auto-refresh on activate**: entering the view (via Home, `:queues`, or
  Backspace/Esc back from a child view) triggers an automatic reload.

## Data & config

- `queue.Summary`: `Name string`, `PendingCount int64`, `ConsumerCount
  int64`, `EnqueueCount int64`, `DequeueCount int64`.
- `queue.Backend` interface: `List(ctx context.Context) ([]Summary, error)`
  — implemented by both the Jolokia client and the mq-proxy client
  (spec/11); this view is backend-agnostic.
- Jolokia backend connection config lives under `queue:` in `config.yaml`
  (`brokerName`, `url`, `username`, `password`); `Default()` points at
  `http://localhost:8161/api/jolokia` with `admin`/`` credentials matching
  the local dev broker. `password` is empty in the file; `Load()` injects
  `MQPROXY_CLIENT_PASSWORD` from the environment when the field is blank,
  so credentials never need to live in the file.
- Local dev broker: `infra/compose.yaml` (Podman/Docker Compose) runs
  `apache/activemq-classic`, exposing `8161` (web console / Jolokia) and
  `61616` (OpenWire). `podman compose -f infra/compose.yaml up -d` (or
  `docker compose`) gives a ready broker matching `Default()`'s
  credentials.

## Implementation notes

- Current location: `tui/internal/view/queues.go` (`QueuesView`) — moved
  here from `internal/app/queues.go` by the later package split; see
  spec/03 for the current `internal/view` package layout and the
  `ui.ViewHost` interface this view depends on.
- The Jolokia client (`tui/internal/queue/jolokia/`) reads queue summaries,
  including Enqueue/DequeueCount, from the broker MBean over Jolokia HTTP
  using only `net/http` + `encoding/json` — no JMX library dependency.

## Notable gotchas worth preserving

- A `tview.Table` cell swallows a leading `[x]`-shaped substring as a color
  tag. The filter string is escaped/wrapped before being embedded in the
  table title for this reason (same issue independently hit and fixed in
  the connection manager and move-picker filters — see spec/12 and
  spec/09).
- Every Jolokia request sets an explicit `Origin` header to the broker
  URL. ActiveMQ's default `jolokia-access.xml` rejects origin-less
  requests under strict origin checking — without this header, calls fail
  against a stock deployment, not just a hardened one.
- Resetting the table selection to row 1 on repaint (`Select(1, 0)`) is
  not enough by itself to visually scroll to the top: `tview.Table` tracks
  an internal "stick to bottom" flag that `Select` alone doesn't clear, so
  a repaint after the table was scrolled to the bottom can still render
  latched there. `SetOffset(0, 0)` must be called alongside `Select(1,
  0)` to actually reset the scroll position.
