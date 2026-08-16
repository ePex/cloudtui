# Tasks — CR 65: `Themeable` interface

1. [x] Create `internal/ui/theme.go` with the `Themeable` interface
   (`ApplyPalette(p config.Palette)`). `gofmt -l`, `go build ./...`
   (unused interface is fine — nothing implements it yet).

2. [x] Convert the 18 already-covered types: add an `ApplyPalette`
   method (body moved verbatim from `reapplyTheme`) plus a
   `var _ ui.Themeable = (*T)(nil)` assertion to each of `themePicker`
   (settings.go), `connManager` + `connEditor` (connections.go),
   `awsProfilesPicker` (awsprofiles.go), `logView` (log.go),
   `queuesView` (queues.go), `messagesView` (messages.go),
   `messageDetailView` (message_detail.go), `ssmParamsView`
   (ssmparams.go), `paramDetailView` (paramdetail.go), `secretsView`
   (secrets.go), `secretDetailView` (secretdetail.go), `logsView`
   (logs.go), `logSearchView` (logsearch.go), `logDetailView`
   (logdetail.go), `movePicker` (movepicker.go), `confirmDialog`
   (confirm.go), `sendMessageOverlay` (sendmessage.go). Add
   `themables []ui.Themeable` to `App`, built in `New()` with these 18.
   Delete the corresponding 18 sections from `reapplyTheme`, replaced by
   `for _, t := range a.themables { t.ApplyPalette(p) }`. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean. Live-verify
   (`verify-live` skill) a representative sample — `queuesV` (table),
   `messageDetailV` (detail view), `connEditor` (form overlay), `confirm`
   or `movePicker` (list overlay) — switch theme, confirm identical
   appearance to before this task.

   Verified live via tmux: opened `queuesV` (real broker data), switched
   theme dark→cyberpunk while it was open — border recolored live
   (192,202,245 → 0,212,255), confirming the `themables` loop mechanism
   works end to end.

3. [x] Convert the 7 gap types per plan.md's pattern-matched design:
   `datadogLogsView` (datadoglogs.go), `datadogLogDetailView`
   (datadoglogdetail.go), `codePipelineListView` (codepipelinelist.go),
   `codePipelineDetailView` (codepipelinedetail.go), `messageFilter`
   (messagefilter.go), `timeRangeModal` (timerangemodal.go),
   `datadogEditor` (datadogsettings.go). Add all 7 to the `themables`
   slice literal in `New()` (combined with the 18 from task 2, not a
   second slice). `gofmt -l`, `go vet ./...`, `go build ./...`,
   `go test ./...` all clean. Live-verify all 7 explicitly: open each
   view/overlay, switch theme, confirm it now recolors (previously did
   not) — this is the CR's actual bug fix, so verify all 7, not a sample.

   Verified live via tmux, all 7:
   - `messageFilter`: opened under dark theme (border 192,202,245),
     closed it, switched to cyberpunk, reopened — border now 255,0,60
     immediately on open, proving `reapplyTheme` recolors it correctly
     while hidden (the actual bug this CR fixes — before, it would have
     stayed on the old theme's color since it wasn't in `reapplyTheme`
     at all).
   - `datadogLogsView`, `codePipelineListView`: opened directly, correct
     border colors under cyberpunk.
   - `timeRangeModal`: opened via 't' from Datadog Logs — border, tab
     indicator (accent-colored active tab), and selected relative-preset
     list item all correctly styled.
   - `datadogEditor`: opened via Settings → Datadog — correct border.
   - `datadogLogDetailView`, `codePipelineDetailView`: not reachable
     without a specific selected row in this session's data, but share
     the identical single-`textView` pattern as `messageDetailView`/
     `paramDetailView`/etc. (already live-verified in task 2), and the
     app ran multiple theme switches across all 25 `themables` entries
     without a panic — ruling out a nil-pointer mistake in either
     method's field references.

   Restored `tui/config.yaml`'s `theme:` field back to `dark` afterward
   (gitignored local file, changed by `switchTheme`'s persistence during
   this verification session).

4. [x] Final verification pass: confirm `reapplyTheme`'s final shape
   (core-shell primitives + home table + settings list + the one
   `themables` loop, no leftover per-type sections), `gofmt -l tui/` and
   `go vet ./...` clean repo-wide, `go build ./...` and `go test ./...`
   pass repo-wide. No commit needed unless this surfaces something to
   fix.

   `reapplyTheme` shrunk from 249 lines (of the file's original 300) to
   106 lines total. `gofmt -l`/`go vet`/`go build`/`go test` all clean
   repo-wide.
