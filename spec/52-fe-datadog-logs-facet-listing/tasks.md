# Tasks — FE 52

1. [x] `internal/datadoglogs`: new `facets.go` (`ListFacetValues`/
   `listFacetValues` against `POST /api/v2/logs/analytics/aggregate`)
   and `facets_test.go`.
2. [x] `internal/app`: `App.listDatadogFacetValues` field + wiring
   (`app.go`); `facetDiscoveryWindow`, `refreshFilterDropdowns`
   extraction, `discoverFacetValues`/`discoverFacetValuesFor`/
   `handleFacetDiscoveryResult`, `Activate()` wiring (`datadoglogs.go`);
   new tests (`datadoglogs_test.go`).

## Manual verification

- Confirm the real `/api/v2/logs/analytics/aggregate` response shape
  against a live Datadog account; adjust `facets.go`'s response struct
  if it differs from the docs-derived guess.
- Reproduce the original report: a service that doesn't show up via a
  normal search within the current time range should now appear in the
  Service dropdown shortly after the view activates.
- A Service/Env selection made before discovery finishes isn't
  reset/clobbered when discovery's result lands.
- An invalid/expired token: existing search-error path unaffected;
  discovery fails silently (visible in the app log only, no user-facing
  error banner).

**Verified 2026-08-16**, against the user's real Datadog account. First
attempt failed live: `group_by[0].sort` (`{aggregation:"count",
order:"desc"}`, no `type`) got rejected with `400
input_validation_error(Field 'aggregation' is invalid: Unrecognized
parameter)` — Datadog's sort defaults to `type=alphabetical`, under
which `aggregation` isn't a recognized field at all (only applies to
`type=measure`). Diagnosed directly from `~/.cloudtui/cloudtui.log`
(readable locally, no need to ask the user to paste it). Fixed by
dropping `sort` entirely from the `group_by` request — alphabetical
default is fine, since `internal/app`'s `sortedKeys` already re-sorts
the returned values for the dropdown regardless of what order Datadog
returns them in. Re-verified after the fix: previously-missing services
now appear in the Service dropdown.
