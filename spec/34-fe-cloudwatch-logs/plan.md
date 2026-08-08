# Plan — FE 34: CloudWatch Logs investigation

## Verified against the real SDK (not assumed)

Temporarily fetched `aws-sdk-go-v2/service/cloudwatchlogs` and inspected
it via `go doc` before committing to this plan (reverted after —
nothing lands in `go.mod` until this plan is approved):

- `DescribeLogGroups(ctx, &DescribeLogGroupsInput{NextToken,
  LogGroupNamePattern})` → `[]types.LogGroup{LogGroupName, CreationTime
  (*int64, ms since epoch), RetentionInDays (*int32, nil = never
  expire), StoredBytes, ...}`, paginated via `NextToken`. Confirms the
  list side is exactly as simple as the other AWS list views.
- `FilterLogEvents(ctx, &FilterLogEventsInput{LogGroupName, StartTime,
  EndTime (*int64, ms since epoch), FilterPattern, Limit,
  StartFromHead, NextToken})` → `[]types.FilteredLogEvent{Timestamp,
  Message, LogStreamName, EventId}`, paginated via `NextToken`
  (expires after 24h — irrelevant here since this slice never
  auto-paginates, per spec.md decision 5). `StartFromHead: false`
  (newest first) is explicitly supported and is what this slice always
  passes — its one documented restriction (`startTime` must be on/after
  2024-01-01 UTC) is a non-issue since every relative preset is recent.
  `FilterPattern` is passed straight through as AWS's own filter-pattern
  string; empty means "match everything in range."
- Both calls need only a log group *name* (not ARN) for a same-account,
  non-cross-account setup, matching this slice's scope.

## `tui/internal/awslogs/`

```go
type LogGroup struct {
    Name          string
    RetentionDays int32     // 0 = never expire
    CreatedAt     time.Time
}

type LogEvent struct {
    Timestamp time.Time
    LogStream string
    Message   string
}

// ListLogGroups fetches every log group's metadata for profile,
// paginating through all results (there's no per-search cost concern
// here — it's metadata, same shape as awsssm.List/awssecrets.List).
func ListLogGroups(ctx context.Context, profile string) ([]LogGroup, error)

// FilterEvents runs a single FilterLogEvents call (profile's log group
// logGroupName, time window [start, end), optional pattern) and returns
// at most one page of results, newest first. hasMore reports whether
// AWS indicated more results exist (NextToken present) — this package
// never auto-paginates (spec.md decision 5); the caller decides what
// to do with hasMore (show it, don't silently fetch more).
func FilterEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern string) (events []LogEvent, hasMore bool, err error)
```

Same construction pattern as `awsssm`/`awssecrets`: `newClient(ctx,
profile)` errors on an empty profile, else `cloudwatchlogs.
NewFromConfig(config.LoadDefaultConfig(ctx,
config.WithSharedConfigProfile(profile)))`.

`FilterEvents` requests `Limit: 1000` (spec.md decision 5's "fixed
page size" — generous enough for one investigation pass, small enough
to render and scroll comfortably) and `StartFromHead: false`.

Pure, unit-testable helpers factored out of the AWS-calling functions
(mirrors `awsssm.buildParameters`/`awssecrets.buildSecrets`):

- `buildLogGroups(raw []types.LogGroup) []LogGroup` — nil-date/nil-int
  handling (`RetentionInDays == nil` → `0`), sorts by name.
- `buildLogEvents(raw []types.FilteredLogEvent) []LogEvent` — nil-safe
  field unwrapping, preserves AWS's own order (already newest-first via
  `StartFromHead: false`, so no re-sort needed).

## New view: `tui/internal/app/logs.go`

Same shape as `ssmParamsView`/`secretsView`: a `tview.Table` (columns
NAME/RETENTION/CREATED) + filter input (`/`-filter convention,
substring on name, client-side — this is metadata, not a search),
registered as `ui.View` + `ui.Shortcuttable`, added to `a.views` and
Home's "Apps" section. Filtered title uses `"(text)"` and the repaint
includes the `Select(1,0)`/`SetOffset(0,0)` scroll-to-top pair from the
start (both established the hard way in FE 32/33 — see
`spec/11-bugfix-queues-scroll-to-top`). `Enter` opens the search view
for the selected log group.

## New view: `tui/internal/app/logsearch.go`

The new shape this feature introduces — a *search* screen, not a
static list or detail:

- A results `tview.Table` (TIMESTAMP/STREAM/MESSAGE, message truncated
  to a preview like `messagesView`'s `Preview` column — full text is
  in the detail view) + a pattern `tview.InputField`.
- Unlike every other filter input in the app, this one is **not**
  live-filtered on every keystroke — each keystroke would be a real
  AWS API call. `SetDoneFunc` (Enter) triggers `search()`; typing alone
  does nothing.
- `t` cycles the time-range preset (`15m → 1h → 3h → 24h → 15m`,
  default `1h`) and immediately re-runs `search()` with the new range
  — a preset change is cheap and instant feedback is more useful than
  a second explicit trigger.
- `r` re-runs `search()` unchanged (refresh, matching every other
  view's `r` convention).
- Opens with an immediate `search()` using the default range and an
  empty pattern (matches every other view's "populate on open"
  behavior; a single bounded `FilterLogEvents` call is no more
  expensive than the metadata list calls elsewhere).
- Title shows the log group name, current preset, and result count;
  appends "(more available — narrow your search)" when `hasMore`.
- `Enter` on a result opens the detail view.
- Async (goroutine + `QueueUpdateDraw`), like every other AWS-calling
  view. The fetch-outcome handling is factored into
  `handleSearchResult(events, hasMore, err)`, called from the
  goroutine's `QueueUpdateDraw` callback — mirrors FE 33's
  `handleFetchResult` split, done here from the start rather than
  retrofitted, since it's what makes the async path directly
  unit-testable (no running tview event loop needed).

## New view: `tui/internal/app/logdetail.go`

Simplest of the three — nothing is masked here, so it's closer to
`messageDetailView` than `paramDetailView`/`secretDetailView`:

- Shows Timestamp/Log Stream metadata, then the full message
  unwrapped.
- `c` copies the message via the existing `App.copyToClipboard` — no
  reveal-gating needed, always available.
- Not a registered `ui.View` (opened via `App.openLogEventDetail`,
  returns to the search view on Esc/Backspace) — same reasoning as
  `messageDetailView`/`paramDetailView`/`secretDetailView`.

## `App` wiring

- New fields: `logsV *logsView`, `logSearchV *logSearchView`,
  `logDetailV *logDetailView`, `listLogGroups func(ctx context.Context,
  profile string) ([]awslogs.LogGroup, error)`, `filterLogEvents
  func(ctx context.Context, profile, logGroupName string, start, end
  time.Time, pattern string) ([]awslogs.LogEvent, bool, error)`.
- `a.listLogGroups = awslogs.ListLogGroups`, `a.filterLogEvents =
  awslogs.FilterEvents` in `New()`.
- Home's "Apps" section gains `{Name: "cloudwatch-logs", Description:
  "Search CloudWatch Logs"}`.
- New pages `"cloudwatch-logs"` / `"log-search"` / `"log-event-detail"`;
  `logsV.table.SetSelectedFunc` → `a.openLogSearch(...)`;
  `logSearchV.table.SetSelectedFunc` → `a.openLogEventDetail(...)`.
- `onGlobalKey` needs exemptions for **two** filter inputs this time —
  `a.logsV.filterInput` (client-side, same as every other list) and
  `a.logSearchV.filterInput` (the server-search pattern field) — both
  are plain top-level-view inputs, not overlay-tracked, so FE 32's
  "every view's filter input needs its own explicit exemption" finding
  applies to both.
- `theme.go` gains blocks for the log-group table/filter, the
  log-search table/filter, and the log-detail textview, mirroring FE
  32/33's blocks (`p.ViewColor("cloudwatch-logs")`).

## Testing

`awslogs`: same precedent as `awsssm`/`awssecrets` — `ListLogGroups`/
`FilterEvents` themselves aren't unit tested against a fake endpoint;
`newClient`'s empty-profile guard, `buildLogGroups`, and
`buildLogEvents` are, since that's where the actual logic to get wrong
lives.

`app`: `logsView`/`logSearchView`/`logDetailView` construction, header,
filter, the no-active-profile error path, time-range cycling, and
`handleSearchResult`'s success/error branches — all via injected
`listLogGroups`/`filterLogEvents`, no real AWS calls. The
Enter-triggers-search-but-typing-doesn't distinction gets its own test
(asserting `SetChangedFunc` isn't wired to trigger a search, only
`SetDoneFunc`). Filtered-title render test
(`renderedScreenText`, from FE 32) included for `logs.go` from the
start.

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Manual verification per `verify-live`, against this machine's real,
   already-selected AWS profile: log group list loads, opening one
   defaults to a 1h search with results, `t` cycles ranges and
   re-searches, `/` + Enter narrows by pattern (typing alone doesn't
   trigger a call), a result's detail view shows the full message, `c`
   copies it. No log message content gets pasted into any commit
   message or shown beyond confirming the mechanism works — some log
   lines may contain sensitive request data even though they aren't
   "secrets" in the AWS-service sense.
