# Spec — FE 22: Named connections

Date: 2026-08-07

## Background

The TUI currently has a single, hard-coded backend connection defined by
top-level `backend`, `queue`, and `proxy` fields in `config.yaml`. There is
no way to manage connections from within the app — the user must edit the
YAML file and restart.

## Problem

Users who work against multiple brokers (e.g. a local dev ActiveMQ and an
AWS Amazon MQ in staging) have no in-app way to define, switch, or manage
those connections.

## Solution

Introduce named connections: a list of `Connection` entries in `config.yaml`,
each with a full name, a short alias, a backend type, and backend-specific
credentials. One connection is active at a time. The top-left info panel shows
the active connection alias. The Settings view gains a "Connection" entry that
opens a connection manager where the user can choose, add, edit, duplicate, and
delete connections — without leaving the app or editing YAML by hand.

## Config shape

```yaml
activeConnection: local       # name of the active connection
connections:
  - name: local
    backend: jolokia
    queue:
      brokerName: localhost
      url: http://localhost:8161/api/jolokia
      username: admin
      password: ""
  - name: aws-staging
    backend: proxy
    proxy:
      url: http://localhost:8080
      username: cloudtui
      password: changeme
```

`name` is the human-readable identifier used in the manager list, the
top-left info panel, and as the `activeConnection` key.

> **Update (CR 55):** the shape above originally also had a short `alias`
> field (e.g. 3–6 chars) shown in the info panel where space was tight,
> separate from `name`. It was removed as redundant — see
> [spec/55-cr-amq-connection-remove-alias](../55-cr-amq-connection-remove-alias/spec.md).
> `name` alone is now used everywhere.

### Backwards compatibility

If `connections` is absent from the file, `Load()` synthesises a single
connection named `"default"` (CR 55: originally also with alias `"def"`)
from the legacy top-level `backend`, `queue`, and `proxy` fields. The legacy
fields remain in the struct
for this migration only and are not written back by `Save()`. A file
round-tripped through `Save()` will always use the new `connections` shape.

## Scope

### In scope

- New `Connection` struct (`Name`, `Alias`, `Backend`, `Queue`, `Proxy` — CR 55:
  `Alias` later removed) and `Connections []Connection` /
  `ActiveConnection string` fields on `Config`.
- `Load()` migration: legacy top-level fields → single `"default"` (CR 55:
  originally `"default"` / `"def"`) connection when `connections` is absent.
- Top-left info panel: second line `Connection: <alias>` (CR 55: `<name>`).
- Settings view: replaced `tview.Form` with a `tview.List` — item 0 is
  "Theme: &lt;name&gt;" (opens a theme-picker overlay), item 1 is
  "Connection: &lt;alias&gt;" (CR 55: `&lt;name&gt;`) (opens the connection
  manager overlay). Both items
  reflect the current live values and refresh automatically after changes.
  The theme picker lists all available themes with the active one marked ⭐.
  (A third, unrelated item — "AWS Profiles" — was added later; see
  `spec/28-fe-aws-profile-discovery/`.)
- **Connection manager overlay**:
  - Lists all connections as `<name>  (<backend>)` (CR 55: originally
    `<alias>  <name>  (<backend>)`); active one marked ⭐.
  - Keyboard: `Enter` = activate, `n` = new, `e` = edit, `d` = duplicate,
    `Del`/`x` = delete (with confirmation; cannot delete the last connection).
  - Activating a connection hot-swaps the backend immediately, updates the info
    panel, closes any open messages or message-detail view (returning to the
    queues list), and reloads the queue list.
- **Connection editor overlay**: a form with Name, Backend (dropdown), and
  the relevant backend fields (jolokia: BrokerName, URL, Username, Password;
  proxy: URL, Username, Password). Shared by Add and Edit. `Esc` cancels
  (same as the Cancel button) without saving — added 2026-08-08 after the
  initial implementation shipped without it, the only way to cancel was
  tabbing all the way to the Cancel button. (CR 55: the form originally also
  had an Alias field between Name and Backend; removed.)
- Duplicate creates a copy named `"<original>-copy"` and opens the editor on
  it. (CR 55: originally also derived an alias `"<alias>2"`; removed along
  with the field.)
- Changes are persisted to `config.yaml` immediately.
- `config.example.yaml` updated to show the new `connections` shape.

### Out of scope

- Per-connection theme.
- Importing/exporting connections.
- UI validation beyond "name must not be empty" (CR 55: originally also
  "alias must not be empty") and "name must be unique".
- Reordering connections.

## UI flow

```
Settings view (tview.List, j/k to navigate, Enter to open)
  ├─ "Theme: <name>"       →  Theme picker overlay (list of themes, ⭐ = active,
  │                           Enter = apply, Esc = cancel)
  └─ "Connection: <name>"  →  Connection manager overlay (CR 55: originally
                              "Connection: <alias>")
       ├─ Enter / activate  →  hot-swap backend; close messages/detail;
       │                       reload queues; close overlay
       ├─ n / new          →  Connection editor overlay (empty form)
       ├─ e / edit         →  Connection editor overlay (prefilled)
       ├─ d / duplicate    →  Connection editor overlay (copy, renamed)
       └─ Del / x / delete →  Confirm dialog → delete; if active, activate
                               first remaining; reload queues
```

## Files touched

| File | Change |
|---|---|
| `tui/internal/config/config.go` | `Connection` struct; `Connections` / `ActiveConnection` on `Config`; update `Default()`; migration in `Load()`; `Save()` writes new shape only |
| `tui/internal/config/config_test.go` | tests for new fields, migration, round-trip |
| `tui/internal/app/app.go` | `activeConn()` / `switchConnection()` helpers; backend hot-swap; close messages/detail on switch |
| `tui/internal/app/topbar.go` | `infoPanelText()` adds `Connection: <alias>` line |
| `tui/internal/app/settings.go` | replace Form with List; add `refreshSettingsList()`, `showThemePicker()`, `closeThemePicker()` |
| `tui/internal/app/connections.go` | new — connection manager + editor overlays |
| `tui/config.example.yaml` | updated to show `connections` / `activeConnection` |

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. A config with no `connections` field loads without error; a `"default"` /
   `"def"` connection is synthesised from the legacy fields.
3. Multiple connections can be activated from the manager; the alias appears in
   the info panel; messages/detail views are closed; the queue list reloads.
4. Add, edit, duplicate, and delete work as described; changes persist to
   `config.yaml`.
5. Deleting the active connection activates the first remaining one.
6. The last connection cannot be deleted.
