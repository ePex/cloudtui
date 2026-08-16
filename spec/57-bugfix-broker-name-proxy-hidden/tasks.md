# Tasks — Bugfix 57: hide Broker Name when Backend is proxy

1. [x] Add `rebuildConnEditorTail(backend string)` to
       `tui/internal/app/connections.go`: capture current Broker Name/URL/
       Username/Password-Source-selection/Password-or-Secret-text via
       `GetFormItemByLabel` (nil-safe); remove every item from index 2
       onward; re-add Broker Name (only if `backend != "proxy"`), URL,
       Username, Password Source, and Password/Password Secret, each
       prefilled from the captured values; re-style and re-wire the fresh
       Password Source dropdown's `SetSelectedFunc`. Fix
       `setConnEditorPasswordField`'s hardcoded `RemoveFormItem(6)` →
       `RemoveFormItem(f.GetFormItemCount() - 1)` (the latent panic bug —
       see plan.md).
2. [x] Wire the Backend dropdown's `SetSelectedFunc` in
       `tui/internal/app/app.go` (currently `nil`) to call
       `a.rebuildConnEditorTail(backends[idx])`, added after the full
       initial form chain is built (same reasoning as the Password Source
       dropdown's wiring).
3. [x] Update `showConnEditor` in `connections.go`: switch Broker Name/URL/
       Username/Password Source/Password-or-Secret prefill from
       `GetFormItem(2..6)` to `GetFormItemByLabel(...)`; only prefill
       Broker Name when `conn.Backend != "proxy"`.
4. [x] Update `saveConnEditor` in `connections.go`: same
       `GetFormItem(2..6)` → `GetFormItemByLabel(...)` swap; Broker Name
       read is nil-safe (empty when the field isn't present).
5. [x] Verify: `go build ./...` and `go test ./...` pass in `tui/`; manual
       live verification (`verify-live` skill, per `tui/CLAUDE.md`):
       jolokia connection shows Broker Name prefilled; toggling to proxy
       hides it without clearing URL/Username/Password Source/Password;
       toggling back to jolokia restores the original Broker Name value;
       with Backend set to proxy, switching Password Source to "AWS
       Secret" does not panic (the scenario that used to break before the
       task 1 fix); saving a proxy connection produces no `brokerName` in
       `config.yaml`.

       `gofmt`, `go vet`, `go build ./...`, `go test ./...` all clean.

       Manually verified 2026-08-16 via `verify-live` (tmux-driven real
       binary) against the real local connections. Confirmed: Broker Name
       shows prefilled (`localhost`) for the `default` jolokia connection;
       toggling Backend to proxy hides it while URL/Username/Password
       Source/Password stay untouched; with Backend=proxy, switching
       Password Source to "AWS Secret" no longer panics (the exact latent
       bug from task 1). A new `zzverify57` proxy connection was created,
       saved, and confirmed in `config.yaml` to have `brokerName: ""` (no
       Broker Name persisted) — then deleted via the manager to leave
       `config.yaml` as found.

       **A second, real bug was found and fixed during this pass**, not
       anticipated in the original plan: modifying Broker Name, toggling to
       proxy, then back to jolokia came back with Broker Name silently
       reset to `""` instead of the modified value — the
       capture-before-remove preservation trick has nothing to read Broker
       Name back from while it's hidden. Fixed with a new `connEditorBrokerName`
       shadow field on `App`; see the "Update, found during live
       verification" note in `plan.md`. Re-verified live after the fix:
       Broker Name modified to `localhostMODIFIED`, round-tripped through
       proxy and back to jolokia, came back exactly as `localhostMODIFIED`.
