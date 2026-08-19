# AWS CloudWatch Logs search

_Condensed from spec/34 — see that folder for the incremental history. Time-range UI superseded by spec/53 — see spec-origin/19-log-investigation-crosslinks for the current shared time-range modal. Pagination behavior updated by spec-origin/90-cr-log-search-pagination._

## Purpose

Search CloudWatch Logs from inside cloudtui — e.g. checking a Lambda/ECS
service's recent errors without reaching for the AWS Console. Unlike the
Parameter Store / Secrets Manager browsers (a small set of named entries),
this is a query tool over a high-volume, time-ordered event stream.

## Behavior / user flow

- A top-level app (`cloudwatch-logs`), its own `ui.View`, listed in Home's
  "Apps" section. Uses `cfg.ActiveAWSProfile` for credentials; errors
  clearly if none is selected.
- **Two screens**:
  1. Log-group list — browse, filterable by name (`/`, same convention as
     the other AWS list views).
  2. Search screen, entered per log group — filter pattern + time range →
     matching events.
- Search uses `FilterLogEvents` (not CloudWatch Logs Insights — Insights
  is async, costs per GB scanned, and is substantially more work;
  deliberately deferred). The filter-pattern string is passed straight
  through to AWS with no client-side reinterpretation, against a single
  log group at a time. The query round-trips to AWS on submit — not
  live-filtered like the local-data views.
- Time range: see spec-origin/19 for the shared time-range modal (`t` key)
  used by this view — Relative presets (15m/1h/4h/1day/2days/3days/7days/
  15days/1month) or an Absolute From/Until range.
- Each `FilterLogEvents` call is capped to a single page (`Limit: 1000`).
  A **plain browse** (no filter pattern) fetches exactly one page — if
  AWS reports more are available (`NextToken` present), that's surfaced
  in the title (`(more available — press n to load more, or narrow your
  search)`) rather than silently fetching more. A **pattern search**
  auto-continues fetching further pages itself, up to
  `maxAutoContinuePages` (10, ~10k events), before falling back to the
  same title hint — this exists because a specific pattern expresses
  clear intent to find every match, and silently dropping one behind the
  1000-event single-page cap defeated that (see
  spec-origin/90-cr-log-search-pagination for the motivating case: a
  high-volume window with a pattern match older than the newest 1000
  events in range). Either way, pressing `n` any time the title shows
  the hint fetches and appends one more page on top of what's already
  shown; narrowing the pattern or time range is still the other way to
  see fewer, more relevant results.
- Newest events first (`StartFromHead: false`) — investigation starts from
  "what's happening now" and works backward.
- `Enter` on a result opens a detail view with the full, unwrapped message
  (log lines are frequently long/multi-line). `c` copies the message to
  the clipboard (nothing is masked here, so no reveal-gating).
- The search screen's results table supports `w` (spec-origin/92): a
  per-session (not persisted) word-wrap toggle on the Message column,
  off by default. The Message column already shows only the first line
  of a multi-line event, capped at 200 chars (`logEventPreview`) —
  wrapping doesn't reveal more of a multi-line message, only the part
  that would otherwise be clipped by the column's rendered width; the
  detail view is still the only place to see a genuinely multi-line
  event in full. Not on the log-group list screen (no free-text column
  there to wrap).
- Read-only, browse/search-scoped: no log group creation/deletion, no
  metric/subscription filters, no writing, no live tailing/follow mode,
  no multi-log-group search.

## Data & config

- `tui/internal/awslogs/` (or equivalent): `ListLogGroups(ctx, profile)
  ([]LogGroup, error)` (paginated `DescribeLogGroups`) and
  `FilterEvents(ctx, profile, logGroupName string, start, end time.Time,
  pattern, nextToken string) ([]LogEvent, next string, error)` (one
  `FilterLogEvents` page per call; `nextToken`/`next` chain calls
  together, `next == ""` meaning no further pages).
- `logSearchView`'s `fetchPages` helper drives `FilterEvents` in a loop
  (closure-based, no goroutine/UI dependency) to implement both the
  pattern auto-continue and the `n` manual-load-more paths described
  above.
- View type: `logSearchView` (per-log-group search screen) — also the
  jump target for the Datadog correlation-ID lookup, see spec-origin/19.

## Notable gotchas worth preserving

- CloudWatch Logs Insights (the SQL-like query language, async
  `StartQuery`/`GetQueryResults`) is a plausible later feature, not
  implemented — `FilterLogEvents` is what's here today.
- The filter-pattern syntax is CloudWatch's own — notably, a term
  containing hyphens (e.g. a UUID) must be double-quoted or it gets
  tokenized on the internal hyphens and fails to match as a whole term
  (relevant when constructing a pattern programmatically — see
  spec-origin/19's correlation-jump feature).
