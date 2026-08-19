# Tasks

1. [x] Update `awslogs.FilterEvents` (`tui/internal/awslogs/filter.go`):
   add a `nextToken string` input param (`"" ` → nil
   `FilterLogEventsInput.NextToken`), and return `(events []LogEvent,
   next string, err error)` instead of `(events []LogEvent, hasMore
   bool, err error)`. Update the doc comment.

2. [x] Update `ui.ViewHost.FilterLogEvents` (`tui/internal/ui/viewhost.go`)
   and its implementation (`tui/internal/app/viewhost.go`'s
   `App.FilterLogEvents`, `tui/internal/app/app.go`'s
   `filterLogEvents` field type) to the same new signature. Thin
   passthrough, no logic change.

3. [ ] Add the `fetchPages` pagination helper and
   `maxAutoContinuePages = 10` constant to
   `tui/internal/view/logsearch.go` (see plan.md for signature). Unit
   tests: exhausts before the cap, hits the cap with more remaining,
   errors on a later page (discards partial results, matches today's
   all-or-nothing error handling), single page (`maxPages=1`).

4. [ ] Rewire `LogSearchView`: rename the `hasMore bool` field to
   `nextToken string`; update `search()` to call `fetchPages`
   (`maxPages` = 10 if `pattern != ""`, else 1); update
   `handleSearchResult` to the new `next string` signature; update
   `Open()`'s reset (`sv.hasMore = false` → `sv.nextToken = ""`).
   Update the existing tests that reference `hasMore`/the old
   `filterLogEventsFn` signature (including `fakeViewHost`).

5. [ ] Add manual "load more": `loadMore()` method (no-op if
   `sv.nextToken == ""`) + `handleLoadMoreResult` (appends to
   `sv.results` rather than replacing, updates `sv.nextToken`, an
   error leaves existing results in place). Wire the `n` keybinding
   into `table`'s `SetInputCapture` switch and add it to
   `Shortcuts()`. Unit tests: appends correctly, updates `nextToken`,
   no-op (no fetch) when `nextToken == ""`, error preserves existing
   results.

6. [ ] Update `updateTitle()`'s hint text (`"(more available — narrow
   your search)"` → `"(more available — press n to load more, or
   narrow your search)"`), shown whenever `sv.nextToken != ""`. Update/
   add the corresponding test(s).

7. [ ] Manual verification against a real AWS CloudWatch log group
   (the 3 scenarios in plan.md's "Manual verification" section — no
   pattern still single-page, pattern auto-continues past 1000,
   previously-hidden event is now found without narrowing the time
   range). Record what was actually checked here once done.

8. [ ] Merge-back: update `spec/17-aws-cloudwatch-logs/spec.md` to
   describe the new end-state pagination behavior (replacing the
   "single page, not auto-paginated" language); delete
   `spec-wip/90-cr-log-search-pagination/`.
