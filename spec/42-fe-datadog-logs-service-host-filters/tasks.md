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
3. [ ] Manual verification (per `tui/CLAUDE.md`): run a broad search,
   confirm both dropdowns populate with real distinct services/hosts;
   pick a Service, confirm results narrow and the Host dropdown
   re-populates to just that service's hosts; pick `(any)` to clear a
   filter; confirm the combined query works for a host value containing
   a hyphen.
