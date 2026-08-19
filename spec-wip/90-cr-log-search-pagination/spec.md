# CloudWatch Logs search: pagination

Date: 2026-08-19

## What

Change how `logSearchView` (spec/17-aws-cloudwatch-logs) handles
`FilterLogEvents` results that don't fit in a single page.

Today, a search is a single `FilterLogEvents` call capped at `Limit:
1000`, newest-first (`StartFromHead: false`), and the view never
auto-paginates — if AWS reports more results exist (`NextToken`
present), that's only surfaced as `(more available — narrow your
search)` in the title. This was a deliberate decision (spec/17).

In practice this means: with a broad time window and enough log
volume, an event you're specifically looking for can be silently
excluded from the page you see — even when you typed a filter pattern
that it matches — because 1000 *newer* matching events exist between
it and the end of the window. The only way to find it today is to
guess-and-narrow the time window until the total drops under 1000,
which is what prompted this change (see conversation this spec
originates from).

## Why

A filter pattern expresses clear intent: "show me events matching
X". When that pattern is set, silently dropping a matching event
because of an internal page-size cap defeats the purpose of the
search — the user has no indication their event exists at all short
of noticing the "(more available)" hint and manually shrinking the
window through trial and error.

## Proposed behavior

1. **Manual "load more" paging**, always available: when the last
   search reported `hasMore`, a keybinding (`n`, mirroring `p` if we
   ever add "previous" — TBD in plan) fetches the next page via
   `NextToken` and appends it to the results table, updating the
   title's event count and `hasMore` state. This applies whether or
   not a pattern is set.
2. **Auto-continuation when a pattern is set**: if `sv.pattern != ""`,
   the search automatically keeps fetching subsequent pages (server
   side, before repainting) until either `hasMore` is false or a
   safety cap is hit (exact cap TBD in plan — e.g. a fixed number of
   pages or a wall-clock budget), rather than stopping at page 1. This
   directly addresses "a filter pattern shouldn't let the 1k limit
   hide a match." If the safety cap is hit, the title still shows
   `(more available — narrow your search)` so the user knows results
   are still incomplete, but they've had a much better chance of
   already seeing their match.
3. When **no** pattern is set (browsing raw events), keep today's
   single-page behavior — auto-continuation without a pattern could
   mean fetching an unbounded, unfiltered event stream, which is a
   different (and much more expensive/slow) thing than what's being
   asked for here. Manual paging (point 1) still applies.

## Scope

- `logSearchView` (`tui/internal/view/logsearch.go`) and
  `awslogs.FilterEvents` (`tui/internal/awslogs/filter.go`) — likely
  needs to expose `NextToken` handling to the caller rather than just
  a `hasMore` bool.
- Unit tests for the new pagination logic (page-append, auto-continue
  cap, hasMore/title state).

## Out of scope

- CloudWatch Logs Insights (still deferred, per spec/17).
- Changing `StartFromHead` (still newest-first).
- Cross-log-group search.
- Live tailing/follow mode.

## Decisions

- Auto-continuation safety cap: **fixed page count** (10 pages, i.e.
  up to ~10,000 events per search when a pattern is set). Exceeding
  it still shows `(more available — narrow your search)` as today.

## Open questions for plan.md

- Should manual "load more" (point 1) replace or append to the
  existing table? (Proposal: append, since narrowing/re-searching
  already resets the table via a fresh `search()`.)
- Keybinding choice (`n` vs. something else — `t`/`r`/`/` are taken).
