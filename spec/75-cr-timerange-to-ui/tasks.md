# Tasks — CR 75: promote `timeRange` group to `internal/ui`

1. [x] Create `internal/ui/timerange.go` with the 7 promoted/exported
   identifiers (`TimeRangePreset`, `TimeRangePresets`,
   `DefaultPresetIdx`, `TimeRangeMode`, `TimeRangeRelative`,
   `TimeRangeAbsolute`, `TimeRange` with `Bounds`/`Label` methods),
   moved verbatim from `logsearch.go` per plan.md's template, doc
   comments updated for the new names. `gofmt -l`, `go build
   ./internal/ui/...` clean.

2. [x] Create `internal/ui/timerange_test.go`: moved
   `TestTimeRangeBounds`/`TestTimeRangeLabel` from `logsearch_test.go`
   verbatim, updated to `ui.TimeRange{...}`/`.Bounds`/`.Label`.
   `go test ./internal/ui/...` passes.

3. [x] Removed the 7 declarations from `logsearch.go`; updated every
   call site to the `ui.`-qualified exported names. Removed
   `TestTimeRangeBounds`/`TestTimeRangeLabel` from `logsearch_test.go`
   and updated its remaining `timeRange`/`timeRangeMode`/etc.
   references, plus added the `ui` import (wasn't previously imported
   by the test file). Confirmed remaining build errors were only in
   `datadoglogs.go`/`timerangemodal.go` and their test files.

4. [x] Updated `datadoglogs.go`/`datadoglogs_test.go`: field type,
   `Show(...)` callback parameter, `.Bounds`/`.Label` call sites, and
   the `ui.TimeRange{...}` construction; added the `ui` import to the
   test file (wasn't previously imported). Confirmed remaining build
   errors were only in `timerangemodal.go`/`timerangemodal_test.go`.

5. [x] Updated `timerangemodal.go`/`timerangemodal_test.go`: renamed
   every occurrence to the `ui.`-qualified exported names.

   Note: also found (and fixed) one call site plan.md's template
   didn't call out explicitly — the constructor's
   `for i, p := range ui.TimeRangePresets { tm.relativeList.AddItem(p.label, ...) }`
   uses `TimeRangePreset`'s own `label` field, which is exported to
   `Label` alongside the type — `go build` caught this immediately as
   `p.label undefined`. `timerangemodal_test.go` was rewritten in full
   (via `Write`, not per-line `Edit`) given the volume of mechanical
   `timeRange{mode: ...}` → `ui.TimeRange{Mode: ...}` /
   `func(timeRange)` → `func(ui.TimeRange)` substitutions across the
   file — sed was avoided per plan.md's stated reasoning (ambiguous
   matches against local `from`/`to` variables sharing names with the
   struct's field labels). `gofmt -l`, `go build ./...` pass
   repo-wide.

6. [x] Final verification pass: grep confirms zero remaining
   `timeRangePreset`/`timeRangePresets`/`defaultPresetIdx`/
   `timeRangeMode`/`timeRangeRelative`/`timeRangeAbsolute`/bare
   `timeRange` identifiers anywhere in `internal/app` — the only
   remaining hits are inside test-failure message string literals
   (e.g. `"timeRangeModal.activeTab = %v, want timeRangeRelative"`),
   which are descriptive text, not code. `gofmt -l tui/` clean; `go
   vet ./...` clean; `go build ./...` and `go test ./...` pass
   repo-wide (all packages `ok`). No live verification needed — pure
   promotion + rename, per spec.md.
