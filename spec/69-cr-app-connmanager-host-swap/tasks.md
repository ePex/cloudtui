# Tasks — CR 69: swap `connManager`/`connEditor` to `host ui.Host`

1. [x] Update `connections.go` per plan.md: `connManager` struct gains
   `host ui.Host`, `confirm *confirmDialog`, `editor *connEditor`
   (replacing `app *App`); `connEditor` struct gains `host ui.Host`,
   `manager *connManager` (replacing `app *App`). Update both
   constructors' signatures and every method substitution listed in
   plan.md (`show`/`close`/`populate`/`delete` on `connManager`;
   `show`/`rebuildTail`/`close`/`save` on `connEditor`). This task
   alone won't build yet — `app.go`'s construction calls still pass the
   old signature — that's expected, resolved in task 2.

2. [x] Update `app.go`: `newConnManager(a)` → `newConnManager(a,
   a.confirm)`, `newConnEditor(a)` → `newConnEditor(a, a.connManager)`,
   plus one new line `a.connManager.editor = a.connEditor` right after
   the `connEditor` construction. `gofmt -l`, `go vet ./...`,
   `go build ./...`, `go test ./...` all clean.

3. [x] Live-verify (`verify-live` skill) the full connection-management
   flow in one pass: open the connection manager (`:aq`), create a new
   connection, edit an existing one (toggle Backend jolokia↔proxy to
   exercise `rebuildTail`), duplicate one, delete a non-active one,
   delete the active one (confirm it switches away correctly). Record
   what was checked here. Clean up any test connections created, and
   restore `config.yaml` to its prior state afterward.

   Verified live via tmux (`config.yaml` backed up and restored
   afterward): opened the manager (list populated correctly); created
   `CR69-test` (jolokia) via `n`, toggling Backend to `proxy` and back
   to `jolokia` mid-edit to exercise `rebuildTail` — Name preserved
   across both rebuilds, Broker Name field correctly disappeared/
   reappeared; saved — `connManager`'s list refreshed via the
   `ce.manager.populate()` sibling reference; duplicated it via `d`
   (correctly pre-filled `CR69-test-copy`, cancelled — not saved);
   edited it via `e` (correctly pre-filled, cancelled); deleted it via
   `x` (non-active — confirm dialog via `cm.confirm.show()` sibling
   reference, list stayed open and repainted); deleted `default` (the
   active connection) via `x` — switched to `local-mq-proxy` (first
   remaining) and navigated to "queues". All 6 steps correct, no
   regressions.

4. [x] Final verification pass: `grep -n '\.app\.' tui/internal/app/connections.go`
   returns nothing; `gofmt -l tui/` and `go vet ./...` clean repo-wide;
   `go build ./...` and `go test ./...` pass repo-wide. No commit
   needed unless this surfaces something to fix.

   Confirmed: zero remaining `.app.` access, all checks clean.
