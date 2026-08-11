# Spec — FE 42: Service/Host filters for Datadog Logs

Date: 2026-08-10

## Background

FE 39's Datadog Logs search view has a single free-text query field.
Datadog's own query syntax already supports `service:<value>` and
`host:<value>`, but typing that by hand every time is friction the user
doesn't want — they asked for dedicated filtering on at least service
and host.

## Decisions (confirmed)

1. **Server-side, not client-side.** New "Service" and "Host" dropdown
   fields are combined into the actual Datadog query sent on search
   (`service:"<val>" host:"<val>" <free-text query>`), not a local
   narrowing of already-fetched rows — keeps searching the full
   selected time range rather than being limited to whatever's already
   on screen (capped at 1000 events per FE 39 decision 2).
2. **Dropdowns populated from the current results**, not free text —
   after every search, both dropdowns are rebuilt from the distinct
   `Service`/`Host` values actually present in `dv.results`, prefixed
   with an `(any)` option that clears that facet. This means the first
   search (before any filter is picked) has to run unfiltered to
   discover what values exist — expected and fine, matches "browse then
   narrow."
3. **Selecting a value re-searches immediately** — no separate confirm
   step, consistent with this app's other immediate-apply pickers (e.g.
   the theme picker).
4. **Filters reconcile against new results after every search**: if the
   currently-selected Service/Host is no longer present among the new
   result set's distinct values (e.g. after picking a Host that no
   events from the previously-selected Service produced), it's reset to
   `(any)` rather than silently continuing to claim a value that
   isn't actually being applied to what's displayed.
5. **Quoted the same way FE 41's CorrelationID fix was**: service/host
   values are double-quoted in the constructed query (hostnames can
   contain hyphens, e.g. `ip-10-0-1-23`, which Datadog's — like
   CloudWatch's — query tokenizer can split on unquoted).

## Scope

- `internal/app/datadoglogs.go`: two `tview.DropDown` fields
  (`serviceFilterDD`, `hostFilterDD`) in a new row between the results
  table and the query input; `serviceFilter`/`hostFilter string` state;
  rebuilt after every `handleSearchResult`; combined into the query
  string `search()` sends. New shortcuts `S` (focus Service) / `H`
  (focus Host) — uppercase specifically to avoid colliding with the
  lowercase global hotkeys (`s`=settings, `h`=home).
- `onGlobalKey`: exempt both dropdowns while focused (same pattern as
  `queryInput`), so typing to jump to an option inside an open dropdown
  doesn't fire a global hotkey.

## Out of scope

- Filtering by anything other than service/host (status, tags, etc.) —
  can follow the same pattern later if wanted.
- Persisting the last-used filter values across searches/sessions.
- Autocomplete/search-as-you-type within the dropdown beyond whatever
  tview's own `DropDown` provides natively.
