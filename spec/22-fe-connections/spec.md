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
    alias: lcl
    backend: jolokia
    queue:
      brokerName: localhost
      url: http://localhost:8161/api/jolokia
      username: admin
      password: ""
  - name: aws-staging
    alias: stg
    backend: proxy
    proxy:
      url: http://localhost:8080
      username: cloudtui
      password: changeme
```

`alias` is a short label (e.g. 3–6 chars) shown in the top-left info panel
where space is tight. `name` is the human-readable identifier used in the
manager list and as the `activeConnection` key.

### Backwards compatibility

If `connections` is absent from the file, `Load()` synthesises a single
connection named `"default"` with alias `"def"` from the legacy top-level
`backend`, `queue`, and `proxy` fields. The legacy fields remain in the struct
for this migration only and are not written back by `Save()`. A file
round-tripped through `Save()` will always use the new `connections` shape.

## Scope

### In scope

- New `Connection` struct (`Name`, `Alias`, `Backend`, `Queue`, `Proxy`) and
  `Connections []Connection` / `ActiveConnection string` fields on `Config`.
- `Load()` migration: legacy top-level fields → single `"default"` / `"def"`
  connection when `connections` is absent.
- Top-left info panel: second line `Connection: <alias>`.
- Settings view: replaced `tview.Form` with a `tview.List` — item 0 is
  "Theme: &lt;name&gt;" (opens a theme-picker overlay), item 1 is
  "Connection: &lt;alias&gt;" (opens the connection manager overlay). Both items
  reflect the current live values and refresh automatically after changes.
  The theme picker lists all available themes with the active one marked ⭐.
- **Connection manager overlay**:
  - Lists all connections as `<alias>  <name>  (<backend>)`; active one marked ⭐.
  - Keyboard: `Enter` = activate, `n` = new, `e` = edit, `d` = duplicate,
    `Del`/`x` = delete (with confirmation; cannot delete the last connection).
  - Activating a connection hot-swaps the backend immediately, updates the info
    panel, closes any open messages or message-detail view (returning to the
    queues list), and reloads the queue list.
- **Connection editor overlay**: a form with Name, Alias, Backend (dropdown),
  and the relevant backend fields (jolokia: BrokerName, URL, Username,
  Password; proxy: URL, Username, Password). Shared by Add and Edit.
- Duplicate creates a copy named `"<original>-copy"` / alias `"<alias>2"` and
  opens the editor on it.
- Changes are persisted to `config.yaml` immediately.
- `config.example.yaml` updated to show the new `connections` shape.

### Out of scope

- Per-connection theme.
- Importing/exporting connections.
- UI validation beyond "name must not be empty", "alias must not be empty", and
  "name must be unique".
- Reordering connections.

## UI flow

```
Settings view (tview.List, j/k to navigate, Enter to open)
  ├─ "Theme: <name>"       →  Theme picker overlay (list of themes, ⭐ = active,
  │                           Enter = apply, Esc = cancel)
  └─ "Connection: <alias>" →  Connection manager overlay
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
