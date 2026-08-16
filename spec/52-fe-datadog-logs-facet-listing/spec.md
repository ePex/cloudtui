# Spec — FE 52: List all Service/Env values via Datadog's facet API

Date: 2026-08-16

## What

The Service/Env filter dropdowns in Datadog Logs (`spec/42-fe-datadog-logs-service-host-filters`) currently only ever offer values seen in this session's search *results* (`knownServices`/`knownEnvs`, accumulated across searches). Add a real "list distinct values" call against Datadog's Logs Aggregate API (`POST /api/v2/logs/analytics/aggregate`, grouping by the `service`/`env` facet) so the dropdowns can offer services/envs that exist in Datadog but haven't shown up in anything searched yet this session.

## Why

Reported live: a known-real service didn't appear in the Service dropdown even after widening the search time range to the widest preset (24h) and re-searching. Per spec 42 decision 2, that dropdown is (deliberately) populated only from what's actually been fetched — a service that hasn't logged within whatever window(s) were searched this session simply never gets discovered, no matter how the accumulation mechanism itself works. Confirmed live it's not a bug in that mechanism — 24h is just the widest window `logSearchView`'s presets (`15m/1h/3h/24h`) go up to, and the service in question apparently logs less often than that.

Datadog's own Aggregate API can answer "what distinct service values exist" directly via a facet `group_by`, decoupled from whatever time range the user has selected for their actual search — this is the same mechanism Datadog's own Log Explorer facet panel uses.

## Decisions (proposed — confirm before plan)

1. **New discovery time window, independent of the search time range.**
   The facet-listing call uses its own fixed, wide window (proposed: last
   **30 days** — long enough to catch infrequent loggers, short enough to
   stay within typical Datadog log-retention windows so the call doesn't
   waste effort on data that's already been deleted) rather than
   `dv.presetIdx`'s current selection. **Confirm 30 days is the right
   default**, or state a different one.
2. **Both Service and Env**, not just Service. Same mechanism, same
   endpoint (two calls, one per facet, since Datadog's `group_by` groups
   by one facet per aggregate call) — doing both keeps the two dropdowns
   consistent rather than Service getting real listing and Env staying
   accumulate-only. **Confirm, or scope down to Service only** if Env's
   existing behavior is fine as-is (few distinct values, always
   observed quickly).
3. **Merges with, does not replace, the existing accumulate-from-results
   mechanism.** `knownServices`/`knownEnvs` still grow from live search
   results too — the facet call seeds a broad baseline (capped at some
   limit, sorted by count), live accumulation still catches anything
   outside that cap or that starts logging mid-session.
4. **Runs automatically once per view activation** (`Activate()`), in
   its own goroutine alongside the initial `search()` call — not a
   separate manual keybinding. Consistent with "browse then narrow"
   (spec 42) and this view's existing pattern of always searching on
   activate.
5. **Fails soft.** If the facet-listing call errors (network blip,
   token issue already surfaced elsewhere, etc.), log it and fall back
   to whatever's already accumulated — do not show an error banner or
   block the view for what's fundamentally a convenience/completeness
   enhancement, not the primary search functionality.
6. **Capped, like the existing Search call.** `group_by.limit` set to
   the same 1000 used for Search's page limit (spec 39 decision 2) — an
   org with more than 1000 distinct services in 30 days is out of scope
   for a dropdown UI regardless.

## Scope

- `internal/datadoglogs`: new `ListFacetValues(ctx, cfg, facet string,
  from, to time.Time) ([]string, error)` (or two thin wrappers
  `ListServices`/`ListEnvs`) calling `POST
  /api/v2/logs/analytics/aggregate` with `group_by: [{facet, limit:
  1000, sort: {aggregation: "count", order: "desc"}}]`, `compute:
  [{aggregation: "count"}]`, and a `filter.query` of `"*"` (matches
  everything) over the fixed discovery window. Reuses the same
  auth/base-URL/error-handling conventions as `Search`/`search`.
  **Response struct shape (`buckets[].by.<facet>`) confirmed against a
  real call during implementation**, not fully pinned down from docs
  alone (Datadog's aggregate response format wasn't fully verifiable
  from the public API reference alone).
- `internal/app/datadoglogs.go`: `Activate()` also kicks off the
  facet-listing goroutine(s); on success, merge results into
  `knownServices`/`knownEnvs` and call `rebuildFilterOptions`-equivalent
  refresh of the dropdowns (without needing a full re-search).

## Out of scope

- Making the discovery window user-configurable (cycling presets, a
  settings field, etc.) — a fixed constant is enough for now.
- Applying the same treatment to any other facet (status, host, etc.)
  — matches spec 42's existing "service/env only" scope.
- A manual "refresh facet list" keybinding — automatic-on-activate only,
  per decision 4.
