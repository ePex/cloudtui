# Loading indicator for the queue list

Date: 2026-08-28

## What

`QueuesView.Load()` fetches the queue list from the active backend in a
goroutine and only touches the screen once it resolves — until then, the
table keeps showing whatever was last loaded (a *different* connection's
queues, or nothing) with no indication a fetch is even in progress. This
fix:

- Shows a status-bar message ("Loading queues…") the moment a fetch
  starts, cleared once it resolves (success or error) — same convention
  already used for JMS type scanning, message sends, etc. (`host.SetStatus`).
- Clears the table immediately and shows a single "Loading…" placeholder
  row (same inline-row mechanism `showError` already uses for its error
  row), so the previous connection's queues can't be mistaken for the
  new connection's current state while a fetch is in flight.
- Guards against a slow, stale response landing after a newer one: if
  the user triggers another `Load()` (switches connections again, or
  presses `r`) before the first resolves, the first's eventual result is
  discarded rather than clobbering whatever the second, newer request
  already rendered.

## Why

Reported directly: switching AMQ connections — especially to a
proxy-backed one that might need a moment to warm up (see the existing
GET-retry-once fix in `tui/internal/queue/proxy/proxy.go`), have wrong
credentials, or simply be unreachable (bad URL, network down) — can take
several seconds up to the proxy client's full ~30s timeout (up to ~60s
with the one retry). For that whole window today, the queue list quietly
keeps showing the *previous* connection's queues with nothing on screen
suggesting a fetch is even happening, which reads as "the app is showing
me this connection's queues" when it's actually stale data from a
connection that's no longer active. This isn't unique to connection
switching — pressing `r` to refresh, or navigating back to the queues
view, hits the exact same `Load()` path — but connection switching is
where the delay is most likely to be long enough to notice and be
actually confusing (mis-attributing stale data to the wrong connection),
which is why it's what got reported.

## Scope

- `QueuesView.Load()`: status-bar "Loading…" message + table placeholder
  row on start, cleared/replaced on completion; a generation counter (or
  equivalent) so only the most recent in-flight `Load()`'s result is
  ever applied.
- Covers every path that calls `Load()`: connection switch
  (`switchConnection` → `Activate()`), navigating to the queues view,
  and the `r` refresh shortcut — fixing it once at the `Load()` level
  rather than per call site.

## Out of scope

- Applying the same loading-indicator treatment to other resource views
  (SSM Parameters, Secrets Manager, CloudWatch/Datadog Logs,
  CodePipeline) — they have the exact same gap (confirmed: no view in
  the app shows any loading indicator today), but the report was
  specifically about queues/connection-switching; worth a follow-up if
  wanted, not bundled in here.
- A cancel affordance or visible countdown/elapsed-time while waiting —
  the fix makes it clear *that* something is loading, not how long it
  might take or a way to abort it early.
- Changing the proxy backend's actual timeout/retry behavior (already
  fixed separately, see `tui/internal/queue/proxy/proxy.go`'s GET-retry
  and spec/11) — this is purely about the UI not going silent while that
  existing behavior plays out.
