# Plan — FE 52

## Approach

1. **`tui/internal/datadoglogs/facets.go`** (new file, mirrors
   `search.go`'s split): `ListFacetValues(ctx, cfg, facet string, from,
   to time.Time) ([]string, error)` — same access-token guard, same
   `https://api.<site>` base URL construction as `Search`. Split into an
   unexported `listFacetValues(ctx, baseURL, accessToken, facet string,
   from, to time.Time) ([]string, error)` for testing against an
   `httptest.Server`, same pattern as `search`.
   - Request: `POST /api/v2/logs/analytics/aggregate` with
     `filter.query = "*"`, `filter.from`/`filter.to` (RFC3339, same
     formatting as `search`), `compute: [{aggregation: "count"}]`,
     `group_by: [{facet: <facet>, limit: 1000, sort: {aggregation:
     "count", order: "desc"}}]`.
   - Response: `{"data":{"buckets":[{"by":{"<facet>":"<value>"}}, ...]}}`
     — parsed into a struct with `Data.Buckets[].By map[string]string`;
     `listFacetValues` pulls `bucket.By[facet]` for each bucket, skips
     empty/missing values. **Struct shape confirmed/adjusted against a
     real response the first time this runs against a real Datadog
     account** (per spec, not fully pinned from docs alone) — flag this
     explicitly during manual verification.
   - Same error handling as `search`: non-2xx → error with truncated
     body; JSON decode failure → wrapped error.
2. **`tui/internal/datadoglogs/facets_test.go`** (new): mirrors
   `search_test.go` —
   `TestListFacetValuesEmptyAccessTokenErrorsWithoutRequest`,
   `TestListFacetValuesSendsExpectedRequestAndParsesResponse` (asserts
   method/path/auth header/`group_by.facet`/`group_by.limit`/
   `filter.from`/`filter.to`, and that returned values match stubbed
   buckets), `TestListFacetValuesSkipsBucketsWithEmptyValue`,
   `TestListFacetValuesNonOKStatusReturnsError`.
3. **`tui/internal/app/app.go`**: new field `listDatadogFacetValues
   func(ctx context.Context, cfg config.DatadogConfig, facet string,
   from, to time.Time) ([]string, error)` alongside `searchDatadogLogs`
   (same struct, same doc-comment style); wired to
   `datadoglogs.ListFacetValues` in `New()` next to the existing
   `a.searchDatadogLogs = datadoglogs.Search` line.
4. **`tui/internal/app/datadoglogs.go`**:
   - `const facetDiscoveryWindow = 30 * 24 * time.Hour`.
   - Extract `refreshFilterDropdowns()` out of `rebuildFilterOptions()`
     (the two `applyFilterOptions` calls) — `rebuildFilterOptions` keeps
     merging `dv.results` into `knownServices`/`knownEnvs`, then calls
     `refreshFilterDropdowns()`. Purely mechanical extraction, no
     behavior change to the existing post-search path.
   - New `discoverFacetValuesFor(facet string, known map[string]bool)`:
     computes `start`/`end` from `facetDiscoveryWindow`, launches a
     goroutine calling `dv.app.listDatadogFacetValues`, hands the result
     to `handleFacetDiscoveryResult` via `QueueUpdateDraw` — same
     shape as `search()`/`handleSearchResult`.
   - New `handleFacetDiscoveryResult(known map[string]bool, values
     []string, err error)`: on error, `slog.Warn` and return (fails
     soft, per spec decision 5); on success, merges `values` into
     `known` (skipping empty strings), calls `refreshFilterDropdowns()`
     only if something new was actually added (avoids pointlessly
     rebuilding the dropdown/reattaching `SetSelectedFunc` on a no-op
     discovery). Split out from `discoverFacetValuesFor` so it's
     directly unit-testable without a goroutine or a running tview
     event loop, same rationale as `handleSearchResult`.
   - New `discoverFacetValues()`: calls
     `discoverFacetValuesFor("service", dv.knownServices)` and
     `discoverFacetValuesFor("env", dv.knownEnvs)`.
   - `Activate()`: `dv.search(); dv.discoverFacetValues()` — both fire
     immediately; discovery doesn't block or delay the search the user
     actually sees first.
5. **`tui/internal/app/datadoglogs_test.go`**: new tests —
   `TestHandleFacetDiscoveryResultMergesNewValues` (starts with a
   populated `knownServices`, calls with overlapping + new values,
   asserts the dropdown's option list grows to include the new one),
   `TestHandleFacetDiscoveryResultNoopOnError` (asserts `known` and the
   dropdown are unchanged), `TestHandleFacetDiscoveryResultSkipsEmptyValues`,
   `TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive`
   (a selection made before discovery finishes isn't clobbered — exercises
   `applyFilterOptions`'s existing selection-preservation logic through
   the new call path).

## Files touched

- `tui/internal/datadoglogs/facets.go` (new)
- `tui/internal/datadoglogs/facets_test.go` (new)
- `tui/internal/app/app.go`
- `tui/internal/app/datadoglogs.go`
- `tui/internal/app/datadoglogs_test.go`
- `spec/52-fe-datadog-logs-facet-listing/tasks.md` (next gate)

## Key decisions

- **New file (`facets.go`), not added to `search.go`.** Matches this
  package's existing one-concern-per-file layout (`datadoglogs.go` for
  the shared `LogEvent` type, `search.go` for the Search endpoint) and
  `tui/CLAUDE.md`'s one-`_test.go`-per-source-file convention.
- **`handleFacetDiscoveryResult` mutates the `known` map in place**
  rather than returning a new one — matches how `rebuildFilterOptions`
  already mutates `dv.knownServices`/`dv.knownEnvs` directly; keeps the
  two merge paths (post-search, post-discovery) structurally identical.
- **Only refresh the dropdowns when something new was actually
  found.** Cheap to check, avoids visibly resetting the dropdown's
  scroll/highlight state (via the unconditional `SetOptions`/
  `SetCurrentOption` calls inside `applyFilterOptions`) on every
  activation when discovery finds nothing new — most activations after
  the first, in practice.
- **No new config field for the discovery window.** Per spec, out of
  scope; a private constant is enough.

## Manual verification

Per `tui/CLAUDE.md`, this is real external-API-integration behavior
(not queue/broker, but the same "verify against the real thing" spirit
applies) — needs a real Datadog account with `DD_ACCESS_TOKEN`/Settings
configured:

- Confirm the actual `/api/v2/logs/analytics/aggregate` response shape
  matches what `facets.go` expects (adjust the struct if Datadog's real
  response differs from the docs-derived guess) — this is the one part
  of this plan not already confirmed against a real endpoint.
- Reproduce today's original report: open Datadog Logs, confirm a
  service that doesn't show up via search within the current time range
  now appears in the Service dropdown shortly after activation (once
  the background discovery call completes).
- Confirm a manually-picked Service/Env filter selected before discovery
  finishes isn't reset/clobbered when discovery's result lands.
- Confirm an invalid/expired token still shows the existing
  search-error path (unaffected) while discovery fails silently (check
  the app log, not a user-facing error banner) — per fail-soft.
