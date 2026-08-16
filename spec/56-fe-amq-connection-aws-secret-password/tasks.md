# Tasks — FE 56: AWS-Secrets-Manager-backed connection passwords

1. [x] Add `PasswordSecret string` (`yaml:"passwordSecret,omitempty"`) to
       both `QueueConfig` and `ProxyConfig` in
       `tui/internal/config/config.go`. Add a round-trip test in
       `tui/internal/config/config_test.go` covering `passwordSecret` on
       both `queue` and `proxy` blocks.
2. [x] Create `tui/internal/app/connectionsecrets.go`: `secretCache`
       (in-memory, mutex-guarded, keyed by `profile + "\x00" + secretName`);
       `(a *App) resolvePassword(ctx, profile, secretName)` (immediate error
       on empty profile, cache hit, or `a.revealSecret` + cache + binary-value
       rejection); `passwordSecretName`/`connWithPassword` helpers;
       `buildBackend` (renamed from the current `newBackendForConn` body);
       `newBackendForConn(a *App, conn config.Connection) queue.Backend`
       (delegates to `buildBackend` when no secret is configured, else
       returns a `*secretBackend`); `secretBackend` implementing all 9
       `queue.Backend` methods — `current`/`refresh`, read methods
       (`List`, `BrowseMessages`) retry once on failure, write methods
       (`PurgeQueue`, `RemoveMessage`, `MoveMessage`, `MoveAllMessages`,
       `SendMessage`, `DeleteMessages`, `MoveMessages`) invalidate on
       failure without retrying.
3. [x] Update `tui/internal/app/app.go`: both existing `newBackendForConn(conn)`
       call sites → `newBackendForConn(a, conn)`; initialize `a.secretCache`
       in `New()` before the init-time backend construction; add the
       "Password Secret (AWS)" field to the connection editor form after
       Password; update the overlay height comment/literal for 7 items
       (18 → 20).
4. [x] Update `tui/internal/app/connections.go`: `newBackendForConn(conn)`
       call in `saveConnEditor` → `newBackendForConn(a, conn)`;
       `showConnEditor` prefills the new field from
       `conn.Queue.PasswordSecret`/`conn.Proxy.PasswordSecret` per backend;
       `saveConnEditor` reads it and sets the corresponding field on the
       saved `config.Connection`.
5. [x] Add `tui/internal/app/connectionsecrets_test.go`: cache-hit-once
       (two `current()`/activations issue one `revealSecret` call);
       no-profile-selected fails without calling `revealSecret`; read call
       (`List`) retries once on failure and returns the retry's result;
       read call still failing after retry surfaces the error (no infinite
       loop); write call (`RemoveMessage`) does not retry on failure but
       does invalidate the cache for the next call. All against fakes
       (`app.revealSecret` and `secretBackend.build` swapped out) — no real
       AWS or HTTP calls.
6. [x] Document `passwordSecret` in `tui/config.example.yaml` (comment on
       the example connection block: precedence over `password`/
       `MQPROXY_CLIENT_PASSWORD`, requires an AWS profile selected in
       Settings).
7. [x] Verify: `go build ./...` and `go test ./...` pass in `tui/`; manual
       live verification (`verify-live` skill, per `tui/CLAUDE.md` — this is
       connection behavior) against a real AWS Secrets Manager secret with
       an AWS profile selected: connection authenticates and lists queues;
       a wrong/nonexistent secret name surfaces a clear `Error: …` row
       instead of failing silently; a connection with no `passwordSecret`
       set is completely unaffected (regression check).

       `gofmt`, `go vet`, `go build ./...`, `go test ./...` all clean.

       Manually verified 2026-08-16 via `verify-live` (tmux-driven real
       binary): the real-AWS-authentication path (a `passwordSecret`
       actually resolving against Secrets Manager and connecting) was
       **deliberately skipped** at the user's choice — this machine's AWS
       profiles are the user's employer's real org (`mlf-*` accounts), and
       creating even a throwaway secret there for this needs explicit
       sign-off I didn't have. That path's logic (cache hit/miss,
       no-profile error, read-retry, write-no-retry+invalidate) is covered
       by `connectionsecrets_test.go` against fakes instead — not
       exercised against real AWS.

       What *was* verified live: regression check — activated the
       `default` connection (jolokia, no `passwordSecret`) and it listed
       real queues exactly as before CR 56, confirming secret-less
       connections are completely unaffected by the new `secretBackend`
       wrapper. Connection editor: opened `e` on `default` and confirmed
       the new "Password Secret (AWS)" field renders (empty, correctly
       prefilled from the unset field) between Password and the Save/Cancel
       buttons, with the overlay not clipped at its new (taller) height.
       Cancelled out without saving; local `config.yaml` left untouched.

8. [x] Refine: replace the always-visible Password + Password Secret (AWS)
       fields (task 3/4's original design — both shown together, ambiguous
       about which wins) with a "Password Source" dropdown (`Plain` / `AWS
       Secret`) that dynamically swaps a single field between the two. New
       `setConnEditorPasswordField(sourceIdx int)` in `connections.go`
       (`RemoveFormItem(6)` + re-add the right field type — always lands
       back at index 6 since `AddButton` items aren't counted by
       `GetFormItem`). `app.go`: dropdown added as item 5, `Password` as
       the default item 6; the dropdown's callback is wired via
       `SetSelectedFunc` after the form is fully built (not passed to
       `AddDropDown` itself, which would fire it before item 6 exists).
       `showConnEditor`/`saveConnEditor` updated to read/set item 5's
       selected source before reading/prefilling item 6. Saved connections
       now only ever have one of `password`/`passwordSecret` set, by
       construction. See `spec.md`/`plan.md` for the updated design.

       `gofmt`, `go vet`, `go build ./...`, `go test ./...` all clean (no
       test changes needed — `connectionsecrets_test.go` exercises
       `secretBackend`/`resolvePassword` directly, not the editor form).

       Manually verified 2026-08-16 via `verify-live`: opened the "new
       connection" editor — default state shows "Password Source: Plain"
       and a "Password" field, no Password Secret field present. Selected
       "AWS Secret" from the dropdown → "Password" swapped to "Password
       Secret (AWS)" with no duplicate/stray field and the overlay still
       rendered correctly (label column just widened to fit the longer
       label). Switched back to "Plain" → swapped back to "Password"
       cleanly. Tabbed off the dropdown and typed a throwaway character —
       landed correctly in the Password field (masked `*` appeared),
       confirming focus tracking survives the `RemoveFormItem`/`Add*`
       swap. Cancelled out without saving.
