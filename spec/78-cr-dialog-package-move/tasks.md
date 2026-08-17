# Tasks — CR 78: physical move to `internal/dialog`

1. [x] Moved all 9 production files and 5 test files from
   `internal/app/` to `internal/dialog/` via `git mv`; changed
   `package app` → `package dialog` at the top of all 14. `go build
   ./...` confirmed failures confined to `internal/app/app.go`
   (undefined type names) and `internal/dialog` (undefined
   `renderedScreenText`/`fakeQueueBackend`, not yet added) as
   expected; the 9 production files alone (`go build
   ./internal/dialog/...`) compiled with zero content changes needed.

2. [x] Created `internal/dialog/dialogtest_test.go` with the
   duplicated `renderedScreenText` and `fakeQueueBackend`, per
   plan.md's template.

   **Correction during implementation**: `datadogsettings_test.go`'s
   `TestSaveDatadogEditorRoundTrip` and `awsprofiles_test.go`'s
   `TestActivateAWSProfilePersistsAndUpdatesUI` — the two true
   `App`/`Host` integration tests CR 76/77 deliberately left calling
   unexported overlay methods (`.save()`, `.activate()`) directly —
   don't compile once moved: those methods are unexported in the new
   `dialog` package, unreachable from a hypothetical `internal/app`
   copy, and the tests can't stay in `internal/dialog` either since
   they need `app.New()` for real `App`/`Host` wiring (info panel,
   settings list, disk persistence), which `internal/dialog` can't
   import. Neither spec.md nor plan.md anticipated this — both assumed
   the 5 test files moved as complete units. Fixed by splitting each:
   - `internal/dialog/datadogsettings_test.go`: replaced
     `TestSaveDatadogEditorRoundTrip` with
     `TestDatadogEditorSaveCallsHostAndCloses` (pure overlay test via
     `testHost`, asserting `host.savedDatadogConfig` was recorded).
   - `internal/dialog/awsprofiles_test.go`: replaced
     `TestActivateAWSProfilePersistsAndUpdatesUI` with
     `TestAWSProfilesActivateCallsHostAndCloses` (pure overlay test via
     `testHost`, asserting `host.activeAWSProfile`/`host.status`).
   - New `internal/app/host_test.go` (didn't exist before): added
     `TestSaveDatadogConfigPersists` and
     `TestSetActiveAWSProfilePersistsAndUpdatesUI`, testing `App`'s
     real `SaveDatadogConfig`/`SetActiveAWSProfile` (`ui.Host` methods)
     directly — the actual disk-persistence/info-panel/settings-list
     behavior, now correctly decoupled from the overlay's own tests.

   `gofmt -l`, `go build ./internal/dialog/...`,
   `go vet ./internal/dialog/...` clean; `go test ./internal/dialog/...`
   passes (all 4 moved test files + `movepicker_test.go`, added below).

3. [x] Updated `internal/app/app.go`: added the `internal/dialog`
   import, qualified the 10 field-declaration types and 10 constructor
   calls with `dialog.`.

   **Correction during implementation**: `a.connManager.editor =
   a.connEditor` (wiring the one sibling back-reference — `ConnEditor`
   doesn't exist yet when `NewConnManager` runs) reaches into
   `ConnManager`'s unexported `editor` field directly; this compiled
   before the move (same package) but not after. Neither spec.md nor
   plan.md caught this — the earlier "zero type-name references
   outside `app.go`" audit checked for type names, not field access on
   an exported type from another package. Fixed by adding
   `func (cm *ConnManager) SetEditor(ce *ConnEditor)` to
   `connections.go` and changing the call site to
   `a.connManager.SetEditor(a.connEditor)`. `gofmt -l`, `go build
   ./...` passes repo-wide after this fix.

4. [x] `go vet ./...` / `go test ./...` repo-wide surfaced a third,
   larger correction: 4 more `internal/app` test files reach into now
   cross-package unexported overlay state, none of them anticipated by
   spec.md/plan.md (which assumed only `app.go` needed touching
   outside the moving set):
   - `app_test.go`: 7 `.visible` reads → `.Visible()`
     (`TestOnPromptDoneAqOpensConnectionManager` ×1,
     `TestOnPromptDoneConnectionsOpensConnectionManager`,
     `TestOnPromptDoneAqWorksFromAnyView`,
     `TestOnPromptDoneApOpensAWSProfiles`,
     `TestOnPromptDoneAwsprofilesOpensAWSProfiles`,
     `TestOnPromptDoneApWorksFromAnyView`); 2 sub-widget focus
     assertions (`a.connManager.list`, `a.awsProfiles.table` — no
     exported accessor exists for either, and none is needed elsewhere,
     so per plan.md's anticipated fallback these were dropped rather
     than adding accessors solely for this) simplified to just the
     `Visible()` check; and one fully misplaced test,
     `TestSortPickerQueues` (tests `movepicker.go`'s own
     `sortPickerQueues` helper, not anything about `App`) moved
     verbatim to new `internal/dialog/movepicker_test.go`.
   - `messages_test.go`: 5 `.visible` reads → `.Visible()`; one
     `a.confirm.text.GetText(true)` (unexported field) → rendered via
     the already-shared `renderedScreenText(t, a.confirm.Primitive(),
     ...)` (same package, already used elsewhere in `internal/app`)
     with a `strings.Contains` check instead of exact equality.
   - `logsearch_test.go` / `datadoglogs_test.go`: each had one test
     (`TestLogSearchViewTKeyOpensTimeRangeModal`,
     `TestDatadogLogsViewTKeyOpensTimeRangeModal`) reaching into
     `.visible`, `.relativeList.GetCurrentItem()`, and calling
     `.onApply(...)` directly — none reachable post-move. Rewrote both
     to drive the real UI path instead: assert the focused primitive
     (`a.tv.GetFocus()`) type-asserts to `*tview.List` (a public
     `tview` type, not a `dialog`-package field) and read/set its
     current item via that type's own exported API, then simulate a
     real Enter keypress via `list.InputHandler()` to trigger the
     production `applyRelative` → `onApply` path — arguably a more
     realistic test than the direct-call version it replaced, and adds
     an implicit focus-correctness check the original didn't have.

   `gofmt -l`, `go build ./...`, `go vet ./...`, `go test ./...` all
   pass repo-wide (all packages `ok`) after these fixes.

5. [x] Added the `internal/dialog/` bullet to `tui/CLAUDE.md`'s
   "Package layout" section, per plan.md's wording.

6. [x] Live verification via the `verify-live` skill, against the
   real configured broker/AWS profile/Datadog config (not a fresh
   fixture — the dev machine's real, non-disposable `config.yaml`).
   Built `/tmp/cloudtui_verify78`, drove it via tmux. Checked, in
   order: ThemePicker (Settings → Theme, renders the embedded theme
   list with the active one starred) → ConnManager (Settings →
   Connection, lists all 5 real connections) → ConnEditor (`n` from
   the manager, renders the new-connection form, Esc returns cleanly
   to the manager) → AWSProfilesPicker (Settings → AWS Profile, lists
   all 69 real discovered profiles, active one starred) →
   DatadogEditor (Settings → Datadog, prefilled from real config) →
   ConfirmDialog (`:queues` → filter `orders` → `p` purge → renders
   "Purge \"orders\"?", selected "No" to cancel — 0 pending, but not
   touched regardless) → SendMessageOverlay (`c` on `orders`, renders
   text area + Submit/Cancel, cancelled via Esc) → seeded one real
   message into `orders` (`task seed:queue -- orders 1`) to exercise
   the ones needing message content → MovePicker (`m` on the seeded
   message, renders the real queue list loaded via the async
   goroutine+`QueueUpdateDraw` path, cancelled via Esc) →
   ConfirmDialog again (`d` to delete the seeded message, renders
   "Delete message from \"orders\"?", confirmed "Yes" this time to
   clean up — queue back to 0 pending, matching pre-seed state) →
   MessageFilter (`f` on the empty `orders` messages view, renders
   JMS Type/From/To/Max Count + Apply/Clear/Cancel) → TimeRangeModal
   (`t` in Datadog Logs, both Relative and Absolute tabs render
   correctly). All 10 overlays rendered and closed correctly, zero
   visual or behavioral regressions. Quit cleanly with `q`; tmux
   session exited with the app (no leftover session to kill). No
   connections added/removed, no queues purged, only the one seeded
   scratch message (created and deleted within the session, net
   zero change to `orders`).

7. [x] Final verification pass: `gofmt -l tui/` clean; `go vet ./...`
   clean; `go build ./...` and `go test ./...` pass repo-wide (all
   packages `ok`). `git status --short` confirms the actual diff scope
   (larger than plan.md anticipated, per tasks 2–4's corrections
   above): the 14 moved files, `internal/app/app.go` +
   `tui/CLAUDE.md` (as planned), plus `internal/app/app_test.go`,
   `internal/app/datadoglogs_test.go`, `internal/app/logsearch_test.go`,
   `internal/app/messages_test.go` (fixed cross-package access), plus
   3 new files: `internal/app/host_test.go`,
   `internal/dialog/dialogtest_test.go`,
   `internal/dialog/movepicker_test.go`.
