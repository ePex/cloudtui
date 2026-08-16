# Tasks — CR 55: Drop the connection `alias` field

1. [x] Remove `Alias` from `tui/internal/config/config.go` (`Connection`
       struct, `Default()`). Update `tui/internal/config/config_test.go`:
       delete `TestDefaultConnectionAlias`; drop `alias:`/`.Alias` from the
       YAML round-trip test fixture and assertions; add a test that a
       fixture with a stale `alias:` key still loads cleanly and produces
       the right `Name`.
2. [x] Update `tui/internal/app/topbar.go` (`infoPanelText`: `.Alias` →
       `.Name`) and `tui/internal/app/topbar_test.go` (rename the two
       alias-checking tests, assert against `Name`).
3. [x] Update `tui/internal/app/settings.go` (`refreshSettingsList` label
       and doc comment: `conn.Alias` → `conn.Name`).
4. [x] Update `tui/internal/app/connections.go`: manager list label format
       drops the alias column; `showConnEditor`/`saveConnEditor` drop the
       Alias form field entirely and reindex the shifted `GetFormItem()`
       calls; drop the "alias required" validation and `Alias:` from the
       saved `config.Connection{}` literal.
5. [x] Update `tui/internal/app/app.go`: remove the Alias `AddInputField`
       from the editor form; fix the shifted `GetFormItem(2)` → `(1)` dropdown
       styling lookup; remove the alias-suffix logic from the duplicate
       (`'d'`) handler; recompute and update the overlay height
       comment/literal for one fewer form item.
6. [x] Update `tui/internal/devtool/config.go` (`AddProxyConnection` drops
       the `alias` param) and `tui/internal/devtool/config_test.go`
       (fixtures and assertions); update `tui/cmd/devtool/main.go`'s
       `add-proxy-conn` usage string, arg count, arg indices, and success
       message.
7. [x] Update `tui/config.example.yaml` to drop `alias:` from example
       connections.
8. [x] Update `spec/22-fe-connections/spec.md`: drop `alias:` from the
       config-shape example, note the field's removal (linking to this CR)
       in the explanatory paragraph, and update the manager/editor bullets
       that describe the old alias-inclusive shape.
9. [x] Verify: `go build ./...` and `go test ./...` pass in `tui/`; manually
       run the app and confirm the info panel, Settings → AMQ Connection
       list, connection manager, editor form (no Alias field, correct tab
       order), and duplicate action all show/use `name` with no trace of
       `alias`, and the editor overlay isn't clipped at its new height.

       Manually verified 2026-08-16 via `verify-live` (tmux-driven real
       binary, no broker calls needed — UI-only change): info panel showed
       `AMQ Connection: local-other-proxy` (full name); Settings list showed
       the same; connection manager rows read `default (jolokia)` /
       `local-mq-proxy (proxy)` / `⭐ local-other-proxy (proxy)` — no alias
       column; `e` on `default` opened the editor with fields Name → Backend
       → Broker Name → URL → Username → Password, no Alias field, overlay
       rendered with a little spare room below the buttons (not clipped);
       `d` on `default` opened "New AMQ Connection" prefilled
       `Name: default-copy`, no alias suffix. Cancelled out without saving;
       local `config.yaml` left untouched.
