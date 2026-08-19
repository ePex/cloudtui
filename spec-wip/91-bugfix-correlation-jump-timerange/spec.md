# CorrelationID jump: carry the event's timestamp into an absolute time range

Date: 2026-08-19

## What

Fix the Datadog → CloudWatch CorrelationID jump (spec/19-log-investigation-
crosslinks, `g` on `datadogLogDetailView`) so the matching CloudWatch
event is actually found, regardless of when the jump is performed
relative to the original Datadog log's timestamp.

Today, `g` queues only the CorrelationID string
(`App.pendingCloudWatchPattern`). When the user then picks a log group,
`LogSearchView.Open()` unconditionally resets the search's time range to
the relative default ("1h" from *now*) — the Datadog event's own
timestamp is discarded entirely. If the Datadog log is older than the
current relative window (e.g. investigating something from this morning,
in the afternoon), the jump lands on a CloudWatch search that structurally
cannot contain the event, even with CR 90's pagination fix — it's not
a page-cap problem, it's outside the queried window altogether.

## Why

The jump's entire purpose is "take me to this event's CloudWatch
counterpart." Silently searching the wrong time window defeats that,
and the failure mode is invisible — the user just sees zero/irrelevant
results and has to notice, then manually reconstruct the right absolute
window from the Datadog event's timestamp (which requires backtracking
to the Datadog detail view to read it again).

## Proposed behavior

- `g` queues the Datadog event's **timestamp** alongside its
  CorrelationID (still one-shot, still cleared on navigating anywhere
  other than `cloudwatch-logs` — same rules as today's pattern-only
  queue, spec/19).
- When the queued jump is consumed (picking a log group), the search
  opens with an **absolute** time range centered on that timestamp
  (`[timestamp - buffer, timestamp + buffer]`) instead of resetting to
  the relative default — see "Open question" below for the buffer size.
- A normal (non-jump) `Open()` — i.e. no queued CorrelationID — keeps
  today's behavior exactly: reset to the relative default preset ("1h"
  from now).
- Once landed, the time range behaves completely normally afterward —
  `t` still opens the shared modal, prefilled from this computed
  absolute window like any other absolute range; the user can widen/
  narrow/switch to relative from there same as always.

## Scope

- `internal/view/datadoglogdetail.go`'s `g` handler.
- `internal/app`'s pending-jump plumbing (`pendingCloudWatchPattern` and
  its `SetPendingCloudWatchPattern`/`OpenLogSearch` methods) — needs to
  also carry a timestamp.
- `internal/ui.ViewHost`'s `SetPendingCloudWatchPattern` method
  signature (interface change, mirrors CR 90's pattern of touching
  `ViewHost` + `App` + the test fake together).
- `internal/view/logsearch.go`'s `LogSearchView.Open()` — needs a way to
  receive an optional initial time range override.
- Unit tests covering the new plumbing and `Open()`'s branch.

## Out of scope

- Anything about CR 90's pagination (`fetchPages`, `n` load-more) —
  unrelated, already shipped.
- The reverse jump direction (CloudWatch → Datadog) — still not
  requested, per spec/19.
- Changing the shared time-range modal itself.

## Decisions

- Buffer size around the Datadog event's timestamp: **±15 minutes**
  (30 minutes total) — tight enough to avoid reintroducing CR 90's
  volume problem in most cases, generous enough for normal cross-system
  delay/clock skew.
