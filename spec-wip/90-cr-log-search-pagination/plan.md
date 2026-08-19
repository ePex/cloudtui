# Implementation plan

## Approach

Both "manual load more" and "auto-continue when a pattern is set" are
the same primitive — fetch another `FilterLogEvents` page via
`NextToken` and either replace or append to the results — so the
implementation is one small pagination helper used two ways:

- `search()` (fresh query) fetches page 1, then keeps calling for more
  pages automatically **only if a pattern is set**, up to a fixed cap
  of **10 pages** (~10k events). Without a pattern, it fetches exactly
  1 page, same as today.
- A new `n` keybinding manually fetches *one more* page on top of
  whatever's currently shown (available any time `NextToken` is
  non-empty, pattern or not) — this is what lets you get past the
  10-page auto-continue cap too, or paginate at all when no pattern is
  set.

## Signature changes

`hasMore bool` is replaced by the actual `NextToken` string
(`"" `= no more) everywhere it's threaded through, since the caller
now needs the token value, not just a flag:

- `awslogs.FilterEvents(ctx, profile, logGroupName string, start, end
  time.Time, pattern, nextToken string) (events []LogEvent, next
  string, err error)` — adds an input `nextToken` (pass through to
  `FilterLogEventsInput.NextToken`, `""` → nil) and returns the
  outgoing token instead of a bool.
- `ui.ViewHost.FilterLogEvents` — same shape change, plus the new
  `nextToken` input param.
- `app.App.FilterLogEvents` / `a.filterLogEvents` field — same shape
  change (thin passthrough, no logic).
- `view.fakeViewHost.filterLogEventsFn` (test double) — same shape
  change.

## New pagination helper (`internal/view/logsearch.go`)

```go
const maxAutoContinuePages = 10

// fetchPages calls fetch (one FilterLogEvents page per call, chained
// by nextToken, starting from "") up to maxPages times, accumulating
// events. Stops early if a page's nextToken is "" (no more results)
// or fetch errors. Returns the accumulated events, the final
// nextToken ("" if exhausted, non-"" if maxPages was hit with more
// remaining), and the first error.
//
// No goroutine/UI dependency — takes fetch as a plain closure so it's
// unit-testable with a call-counting stub, same spirit as
// buildLogEvents/handleSearchResult being split out from their
// network-calling callers.
func fetchPages(fetch func(nextToken string) ([]awslogs.LogEvent, string, error), maxPages int) ([]awslogs.LogEvent, string, error)
```

`search()` calls `fetchPages(pageFetchFn, maxPages)` where
`pageFetchFn` closes over `host.FilterLogEvents(ctx, profile,
logGroupName, start, end, pattern, nextToken)`, and `maxPages` is
`maxAutoContinuePages` if `pattern != ""`, else `1`. Runs inside the
existing background goroutine; result handed to (an updated)
`handleSearchResult(events []awslogs.LogEvent, next string, err
error)` via `QueueUpdateDraw`, same as today — it *replaces*
`sv.results`.

## Manual "load more" (`n` key)

- New method `loadMore()`: guarded by `sv.nextToken != ""` (no-op
  otherwise — nothing to fetch). Spawns a goroutine calling
  `host.FilterLogEvents` once with `sv.nextToken`, handing the result
  to a new `handleLoadMoreResult(events []awslogs.LogEvent, next
  string, err error)` via `QueueUpdateDraw` — *appends* to
  `sv.results` (unlike `handleSearchResult`, which replaces) and
  updates `sv.nextToken`.
- Wired into `table.SetInputCapture`'s switch alongside the existing
  `r`/`t`/`/`/`j`/`k` cases.
- Added to `Shortcuts()`'s list (`{Key: "n", Description: "load
  more"}`), conditionally worded or always-shown — simplest is
  always-shown, matching how `r`/`t` are always shown regardless of
  state.

## State field rename

`LogSearchView.hasMore bool` → `LogSearchView.nextToken string`.
`Open()` resets it to `""` (was `false`) alongside the other reset
state.

## Title/UX

`updateTitle()`'s hint changes from:

    (more available — narrow your search)

to:

    (more available — press n to load more, or narrow your search)

only shown when `sv.nextToken != ""`, same as today's `sv.hasMore`
check.

## Out of scope (confirmed from spec.md)

- `internal/datadoglogs` — same "single page, no auto-paginate"
  pattern exists there (`search.go`'s own comment references
  `awslogs.FilterEvents`'s behavior explicitly), but the user's report
  and this CR are CloudWatch-specific. Not touched.

## Tests

- `awslogs`: extend existing `filter_test.go` coverage of
  `buildLogEvents` if needed; `FilterEvents` itself still isn't
  independently unit-tested (real AWS call) — unchanged from today.
- `internal/view/logsearch_test.go`:
  - `fetchPages`: table-driven — exhausts before cap, hits cap with
    more remaining, errors on page 2 (partial results discarded or
    kept? — decision: discard, treat as a full search error, simplest
    and matches today's all-or-nothing error handling in
    `handleSearchResult`), single page (maxPages=1, no-pattern case).
  - `handleSearchResult`: update existing tests for the `next
    string`-based signature; add a case asserting the new title
    wording when `next != ""`.
  - `handleLoadMoreResult`: new tests — appends rather than replaces,
    updates `nextToken`, error case leaves existing results alone
    (doesn't clear the table like a fresh search error would).
  - `n` keybinding: input-capture test mirroring the existing `t`-key
    test — asserts `loadMore()` is a no-op (no fetch call) when
    `nextToken == ""`.
  - `Open()` reset test: update the `sv.hasMore = true` seed to
    `sv.nextToken = "some-token"`, assert it resets to `""`.

## Manual verification (spec/CLAUDE.md: AWS-integration behavior can't be fully unit tested)

This needs a real AWS profile against a log group with enough volume
to exceed 1000 events in a chosen window (not achievable in CI):

1. Search a high-volume log group with no pattern, a window known to
   exceed 1000 events → confirm still exactly 1 page (today's
   behavior unchanged), `n` fetches a second page and appends.
2. Same log group, with a pattern matching >1000 events across the
   window → confirm the view fetches multiple pages automatically
   (visible via elapsed time / event count > 1000) up to the 10-page
   cap.
3. A pattern matching a known event previously hidden by the 1000-cap
   (per this CR's motivating case) → confirm it's now found without
   manually narrowing the time range.

Record what was actually checked in `tasks.md` once done.
