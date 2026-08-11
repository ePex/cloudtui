# Spec — FE 42: Service/Env filters for Datadog Logs

Date: 2026-08-10

## Background

FE 39's Datadog Logs search view has a single free-text query field.
Datadog's own query syntax already supports `service:<value>` and
`env:<value>`, but typing that by hand every time is friction the user
doesn't want — they asked for dedicated filtering on at least service
and (originally) host.

**Pivoted mid-implementation**: the second filter facet was changed
from Host to Env after the Service filter shipped — Env (deployment
environment, e.g. `env:prod`/`env:testt`) is the more useful pairing
with Service in practice. Unlike `service`/`status`/`host`, Datadog has
no top-level `env` attribute — it's conventionally an `env:<value>` tag,
so `LogEvent` gained an `Env` field extracted from the tags array
(`internal/datadoglogs.extractEnv`). `Host` remains on `LogEvent` and in
the detail view (still real, informational data) — only the *filter*
moved.

## Decisions (confirmed)

1. **Server-side, not client-side.** New "Service" and "Env" dropdown
   fields are combined into the actual Datadog query sent on search
   (`service:"<val>" env:"<val>" <free-text query>`), not a local
   narrowing of already-fetched rows — keeps searching the full
   selected time range rather than being limited to whatever's already
   on screen (capped at 1000 events per FE 39 decision 2).
2. **Dropdowns populated from results seen so far**, not free text —
   both dropdowns offer every distinct `Service`/`Env` value seen
   across searches, prefixed with an `(any)` option that clears that
   facet. **Accumulated, not replaced per-search** (found live): once a
   facet filter is active, every subsequent search response only
   contains events matching it, so rebuilding purely from the latest
   result set shrank the option list to just the current selection —
   every other previously-seen value became unselectable without
   resetting to `(any)` first. Values are merged into a running set
   (`knownServices`/`knownEnvs`) instead. First search (before any
   filter is picked) still has to run unfiltered to discover what
   values exist at all — expected and fine, matches "browse then
   narrow."
3. **Selecting a value re-searches immediately** — no separate confirm
   step, consistent with this app's other immediate-apply pickers (e.g.
   the theme picker).
4. **Filters reconcile against new results after every search**: if the
   currently-selected Service/Env is no longer present among the new
   result set's distinct values, it's reset to `(any)` rather than
   silently continuing to claim a value that isn't actually being
   applied to what's displayed.
5. **Quoted the same way FE 41's CorrelationID fix was**: service/env
   values are double-quoted in the constructed query (values can
   contain punctuation Datadog's — like CloudWatch's — query tokenizer
   would otherwise split on unquoted).
6. **Unselected dropdown items need `styleDropDown`** (found live): same
   `tview.DropDown` gotcha already hit for the theme/connection-editor
   dropdowns — `SetListStyles` isn't automatic, or unselected popup-list
   items are unreadable against the theme background.

## Scope

- `internal/datadoglogs`: `LogEvent.Env` field, `extractEnv(tags
  []string) string` (first `"env:<value>"` tag, empty if none) used by
  `buildLogEvents`.
- `internal/app/datadoglogs.go`: two `tview.DropDown` fields
  (`serviceFilterDD`, `envFilterDD`) in a new row between the results
  table and the query input, both styled via `styleDropDown`;
  `serviceFilter`/`envFilter string` state, accumulated via
  `knownServices`/`knownEnvs`; combined into the query string
  `search()` sends. New shortcuts `S` (focus Service) / `E` (focus Env)
  — uppercase specifically to avoid colliding with the lowercase global
  hotkeys (`s`=settings, `l`=log — `e` itself isn't a global hotkey, but
  `S`/`E` keeps the pairing visually consistent).
- `internal/app/datadoglogdetail.go`: detail view gains an `Env` line
  alongside the existing Service/Status/Host/Tags lines.
- `onGlobalKey`: exempt both dropdowns while focused (same pattern as
  `queryInput`), so typing to jump to an option inside an open dropdown
  doesn't fire a global hotkey.

## Out of scope

- Filtering by anything other than service/env (status, host, tags,
  etc.) — can follow the same pattern later if wanted.
- Persisting the last-used filter values across searches/sessions.
- Autocomplete/search-as-you-type within the dropdown beyond whatever
  tview's own `DropDown` provides natively.
