# Spec — FE 34: CloudWatch Logs investigation

Date: 2026-08-08

## Background

FE 28/29/32/33 built up a small suite of read-only AWS browsers
(profiles, Parameter Store, Secrets Manager) all sharing the same
shape: list → filter → detail, using `cfg.ActiveAWSProfile` for
credentials. CloudWatch Logs is a different shape — not a small set of
named entries to browse, but a high-volume, time-ordered event stream
to *search* — so this is the first feature in that family that's a
query tool rather than a browser.

## Problem

No way to search CloudWatch Logs from cloudtui — useful for the same
"investigate without leaving the app" motivation as FE 32/33 (e.g.
checking a Lambda/ECS service's recent errors without reaching for the
AWS Console).

## Decisions (confirmed)

1. **New top-level app**, `cloudwatch-logs`, same pattern as
   `queues`/`ssm-parameters`/`secrets-manager`: a Home "Apps" entry,
   its own `ui.View`. Uses `cfg.ActiveAWSProfile` for credentials,
   erroring clearly if none is selected.
2. **Two screens**: a log-group list (browse, filterable by name — same
   convention as the other AWS list views), and a search screen entered
   per log group (filter pattern + time range → matching events).
3. **Search uses `FilterLogEvents`, not CloudWatch Logs Insights.**
   Insights (the SQL-like query language) is async (`StartQuery` →
   poll `GetQueryResults`), costs per GB scanned, and is substantially
   more implementation work — deliberately deferred to a later feature,
   the same way Secrets Manager was deferred out of FE 32. This slice
   uses `FilterLogEvents`'s filter-pattern syntax passed straight
   through to AWS (no client-side reinterpretation) against a single
   log group at a time.
4. **Time range is a relative preset, not free-form timestamps**: 15m /
   1h / 3h / 24h, ending now, cycled with a key rather than typed.
   Simpler UI, covers the common "what just happened" case; explicit
   start/end timestamps are a future enhancement if wanted.
5. **Results are capped to a single page** (a fixed `Limit`, not
   auto-paginated) — if AWS reports more results are available
   (`NextToken` present), that's surfaced in the title/status rather
   than silently fetching more. Keeps a search fast and predictable
   rather than potentially pulling thousands of events across many
   pages. Narrowing the filter pattern or time range is the way to see
   fewer, more relevant results — consistent with this being an
   "investigate," not "export," tool.
6. **Newest events first** (`StartFromHead: false`) — investigation
   typically starts from "what's happening now" and works backward,
   unlike a log file naturally read top-to-bottom.
7. **A result opens a detail view** showing the full message
   unwrapped/unmasked (log lines are frequently long or multi-line),
   mirroring the message/parameter/secret detail view precedent. `c`
   copies the message to the clipboard (reusing `App.copyToClipboard`,
   no reveal-gating needed here since nothing is masked).
8. **Read-only, browse/search-scoped**: no log group creation/deletion,
   no metric filters, no subscription filters, no writing.

## Proposed scope for this slice

- `tui/internal/awslogs` (or similar): thin wrapper over
  `cloudwatchlogs.Client` — `ListLogGroups(ctx, profile) ([]LogGroup,
  error)` (paginated `DescribeLogGroups`) and `FilterEvents(ctx,
  profile, logGroupName string, start, end time.Time, pattern string)
  ([]LogEvent, hasMore bool, error)` (single-page `FilterLogEvents`).
- New view (table, same shape as the other AWS list views): lists log
  groups (name/retention/created), filterable by substring on name.
- New search view per log group: filter-pattern input (server
  round-trip on Enter, not live-filtered like the local-data views),
  time-range preset cycled by a key, results table
  (timestamp/stream/message preview). `Enter` opens the detail view for
  a result.
- Registered as a real `ui.View`/`ui.Shortcuttable`, listed under
  Home's "Apps" section next to the other AWS apps.

## Out of scope (this slice)

- CloudWatch Logs Insights (query language, aggregations) — a likely
  later feature, deliberately deferred as noted in decision 3.
- Live tailing / follow mode (this is historical search, not
  monitoring).
- Multiple log groups per search.
- Free-form/absolute time ranges — only the relative presets.
- Any region other than the active profile's configured region.
- Log group management (create/delete/retention/metric filters/
  subscription filters).
