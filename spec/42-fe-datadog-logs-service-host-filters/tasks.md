# Tasks — FE 42

1. [x] `internal/app/datadoglogs.go`: add `serviceFilterDD`/`hostFilterDD`
   fields + layout row, `applyFilterOptions`/`rebuildFilterOptions`
   (with the no-recursive-search guard), `effectiveQuery`, wire
   `search()` to use it, `Shortcuts()` + `S`/`H` key handling. Unit
   tests for `applyFilterOptions`/`rebuildFilterOptions` (including the
   no-recursion assertion, tested deterministically — see plan.md's
   updated Testing section) and `effectiveQuery`.
2. [x] `internal/app/app.go`: `onGlobalKey` exemptions for both new
   dropdowns. Unit tests mirroring the existing `queryInput` exemption
   test.
3. [x] Pivot: replace the Host filter with an Env filter (requested
   after tasks 1-2 shipped, before manual verification completed) — see
   spec.md's "Pivoted mid-implementation" note. `internal/datadoglogs`:
   `LogEvent.Env` + `extractEnv` (env is a tag, not a top-level
   attribute, unlike service/status/host). `internal/app/datadoglogs.go`:
   `hostFilterDD`/`hostFilter`/`knownHosts` → `envFilterDD`/`envFilter`/
   `knownEnvs`, label "Host:"→"Env:", shortcut `H`→`E`, query key
   `host:`→`env:`. `internal/app/datadoglogdetail.go`: detail view
   gains an Env line (Host stays — still real, informational data, just
   no longer filterable). All affected tests renamed/updated to match.
4. [x] Manual verification (per `tui/CLAUDE.md`): run a broad search,
   confirm both dropdowns populate with real distinct services/envs;
   pick a Service, confirm results narrow and the Env dropdown
   re-populates to just that service's envs; pick `(any)` to clear a
   filter; confirm the combined query works correctly. Confirmed
   working.

   **Follow-up requested**: after picking a filter value, focus stayed
   on the dropdown rather than returning to the results table. Fixed by
   having the `onSelect` closures (wired in `rebuildFilterOptions`) call
   `dv.app.tv.SetFocus(dv.table)` after `search()` — safe since these
   closures only ever fire for a genuine user-driven selection, never
   during the reconciliation rebuild (see `applyFilterOptions`'s
   nil-callback guard). Covered by
   `TestRebuildFilterOptionsSelectingAnOptionRefocusesTable`.

   **Found during verification (1)**: unselected items in both
   dropdowns' popup lists were unreadable — the same `tview.DropDown`
   gotcha already hit for the theme/connection-editor dropdowns (needs
   `SetListStyles`, wired via this codebase's existing `styleDropDown`
   helper). Neither new dropdown had it applied. Fixed by calling
   `styleDropDown(dd, p)` right after constructing each. Genuinely
   untestable — `tview.DropDown` exposes no getter for list styles, and
   no such test exists for the original settings dropdowns either;
   confirmed by inspection and will be re-verified live.

   **Found during verification (2)**: after picking a Service (e.g.
   "activemq"), every other option became unselectable until first
   resetting to "(any)". Cause: `rebuildFilterOptions` rebuilt the
   dropdown purely from the latest (now filter-narrowed) result set, so
   the option list shrank to just the current selection on every
   search. Fixed by accumulating distinct values across searches
   (`knownServices`/`knownHosts`) instead of replacing them each time —
   see the updated decision in spec.md and the doc comment on
   `rebuildFilterOptions`. Covered by
   `TestRebuildFilterOptionsAccumulatesAcrossNarrowedSearches`.
