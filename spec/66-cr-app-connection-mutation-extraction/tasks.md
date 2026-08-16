# Tasks — CR 66: extract config-mutating logic into named `App` methods

1. [x] Add `(a *App) SaveConnection(conn config.Connection, origName string, isNew bool)`
   to `connections.go` per plan.md, and update `connEditor.save()` to
   call it instead of inlining the append-or-replace/backend-rebuild
   block. `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...`
   all clean.

2. [x] Add `(a *App) DeleteConnection(name string) (wasActive bool)` to
   `connections.go` per plan.md, and update `connManager.delete()`'s
   confirm callback to call it and branch on the return value instead
   of inlining the removal/reactivation logic. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

3. [x] Add `(a *App) SaveDatadogConfig(cfg config.DatadogConfig)` to
   `datadogsettings.go` per plan.md, and update `datadogEditor.save()`
   to call it instead of inlining the set+persist+refresh. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

4. [x] Add `(a *App) SetActiveAWSProfile(name string)` to
   `awsprofiles.go` per plan.md (including the deliberate persist-order
   change noted there), and update `awsProfilesPicker.activate()` to
   call it instead of inlining the set+refresh+persist. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

5. [x] Live-verify all four (`verify-live` skill), since all have real
   broker/AWS-file/Datadog interaction not covered by `go test`:
   - Edit the active connection's settings — queues view reflects the
     new backend immediately.
   - Delete the active connection — switches to another, navigates to
     "queues".
   - Delete a non-active connection — manager's list updates in place,
     no navigation.
   - Save Datadog settings — settings screen shows the new site.
   - Activate a different AWS profile — info panel and settings screen
     update.
   Record what was checked here. Restore any local `config.yaml`/AWS
   profile state changed during verification back to what it was
   before, per `verify-live`'s cleanup checklist.

   Verified live via tmux, backing up `config.yaml` first and restoring
   it afterward:
   - Activated `local-mq-proxy`, edited its URL while active
     (`SaveConnection`'s active-connection branch), confirmed the queues
     view immediately used the new URL (error message referenced the
     exact new, malformed port — proving the backend was rebuilt from
     the saved value, not a stale one).
   - Deleted a non-active connection (`other-proxy-dev-aws-secret`) —
     removed from the list, manager stayed open and repainted, active
     connection unaffected.
   - Deleted the active connection (`local-mq-proxy`) — switched to
     `default` (first remaining), backend rebuilt, navigated to
     "queues" (real broker data shown), manager closed.
   - Saved Datadog settings (changed Site) — persisted to `config.yaml`,
     settings screen updated immediately.
   - Activated a different AWS profile — info panel and settings screen
     both updated, persisted to `config.yaml`.

   Note: hit the documented `tview.Form` focus-memory gotcha (from
   `verify-live`'s "Known tview gotchas") reopening the connection
   editor — focus doesn't reset to item 0 on reopen, so blind Tab
   counts landed in the wrong field twice before switching to the
   recommended technique (probe with a throwaway character, or
   Shift+Tab repeatedly to guarantee landing on the first field).

6. [x] Final verification pass: `gofmt -l tui/` and `go vet ./...` clean
   repo-wide, `go build ./...` and `go test ./...` pass repo-wide. No
   commit needed unless this surfaces something to fix.
