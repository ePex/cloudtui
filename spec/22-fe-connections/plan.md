# Plan — FE 22: Named connections

## 1. Config changes

### New `Connection` struct

```go
type Connection struct {
    Name    string      `yaml:"name"`
    Alias   string      `yaml:"alias"`
    Backend string      `yaml:"backend"` // "jolokia" | "proxy"
    Queue   QueueConfig `yaml:"queue"`
    Proxy   ProxyConfig `yaml:"proxy"`
}
```

### Updated `Config` struct

The legacy top-level `Backend`, `Queue`, `Proxy` fields are removed. A
private `legacyConfig` struct handles migration during `Load()` only and is
never written.

```go
type Config struct {
    ActiveConnection string       `yaml:"activeConnection"`
    Connections      []Connection `yaml:"connections"`
    Theme            string       `yaml:"theme"`
    Logo             []string     `yaml:"logo"`
    Colors           Palette      `yaml:"colors"`
}

// legacyConfig is used inside Load() to read pre-FE22 files; never written.
type legacyConfig struct {
    Backend string      `yaml:"backend"`
    Queue   QueueConfig `yaml:"queue"`
    Proxy   ProxyConfig `yaml:"proxy"`
}
```

### `Default()`

Returns a single `"default"` / `"def"` jolokia connection:

```go
Connections: []Connection{{
    Name: "default", Alias: "def", Backend: "jolokia",
    Queue: QueueConfig{BrokerName: "localhost",
        URL: "http://localhost:8161/api/jolokia", Username: "admin"},
}},
ActiveConnection: "default",
```

### Migration in `Load()`

After unmarshalling, if `cfg.Connections` is empty, unmarshal again into
`legacyConfig` and synthesise a `"default"` / `"def"` connection from those
fields. This is transparent — the next `Save()` writes the new shape.

### `ActiveConn()` helper on `Config`

```go
func (c Config) ActiveConn() Connection {
    for _, conn := range c.Connections {
        if conn.Name == c.ActiveConnection {
            return conn
        }
    }
    if len(c.Connections) > 0 {
        return c.Connections[0]
    }
    return Connection{}
}
```

### Env-var password injection

Currently `MQPROXY_CLIENT_PASSWORD` injects into `cfg.Queue.Password`. With
the new shape it injects into the active connection's `Queue.Password` (for
jolokia) or `Proxy.Password` (for proxy) when that field is empty. The
injection is applied to all connections, not just the active one, so that
switching connections in the manager also picks up the env var.

## 2. Backend construction helper

Package-level function in `app.go`, shared by initial construction and
hot-swap:

```go
func newBackendForConn(conn config.Connection) queue.Backend {
    if conn.Backend == "proxy" {
        return proxy.NewClient(conn.Proxy)
    }
    return jolokia.NewClient(conn.Queue)
}
```

## 3. Hot-swap: `switchConnection(name string)`

Method on `*App`. Called from the connection manager when a connection is
activated:

1. Find the connection by name in `a.cfg.Connections`; no-op if not found.
2. Set `a.cfg.ActiveConnection = name`.
3. Reinitialise `a.backend = newBackendForConn(conn)`.
4. Update `a.queuesV.backend = a.backend` (the only view with its own backend
   reference; `messagesView` and `messageDetailView` call `a.backend` directly).
5. Update the info panel: `a.infoPanel.SetText(infoPanelText(a.cfg))`.
6. Close messages/detail views: switch pages to `"queues"`, set focus to
   `a.queuesV.flex`, trigger `a.queuesV.load()`.
7. Persist: `config.SaveDefault(a.cfg)`.

## 4. Top bar

`infoPanelText` gains a second line:

```
Theme:      dark
Connection: def
```

The info panel is a `*tview.TextView` already sized to `shortcutPanelRows`
rows, so the second line fits without a height change.

## 5. Settings view

Add a `tview.Button` labelled `"Connection"` below the Theme dropdown.
Pressing it calls `a.showConnectionManager()`.

## 6. Connection manager overlay (`connections.go`)

Reuses the same `centered()` + `rootPages` overlay pattern as the existing
move-picker and confirm overlays.

Layout:
```
┌─ Connections ────────────────────────────────────────┐
│ ⭐ def   default         (jolokia)                   │
│    stg   aws-staging     (proxy)                     │
│                                                       │
│ [Enter] activate  [n] new  [e] edit  [d] dup         │
│                            [Del/x] delete             │
└──────────────────────────────────────────────────────┘
```

A `tview.List` occupies the upper portion; a `tview.TextView` below it shows
the shortcut hints. `SetInputCapture` on the list handles `n`, `e`, `d`,
`Del`, `x`, and `Escape`.

**Delete flow:** show the existing `showConfirm` dialog. On confirm:
- Remove the connection from `a.cfg.Connections`.
- If it was active, set `ActiveConnection` to `a.cfg.Connections[0].Name` and
  call `switchConnection`.
- Persist and re-open the manager.
- Blocked if `len(a.cfg.Connections) == 1`.

## 7. Connection editor overlay (`connections.go`)

A `tview.Form` + submit/cancel `tview.List` in a `tview.Flex`, opened on top
of the manager (or directly from the manager).

**Fields (always shown):**

| Field | Notes |
|---|---|
| Name | `tview.InputField` |
| Alias | `tview.InputField` |
| Backend | `tview.DropDown`: jolokia / proxy |
| BrokerName | `tview.InputField`; jolokia only (ignored on save if backend=proxy) |
| URL | `tview.InputField` |
| Username | `tview.InputField` |
| Password | `tview.InputField`, masked |

All fields are always visible. On save, only the fields relevant to the chosen
backend are written; the other backend's sub-config is left at its zero value.

**Save logic:**
- Validate name non-empty and unique (among other connections, excluding the
  one being edited).
- Build a `config.Connection` and either append (new/dup) or replace (edit) in
  `a.cfg.Connections`.
- Persist, refresh the manager list, and close the editor.

**Duplicate:** copies the selected connection, sets name to `"<name>-copy"` and
alias to `"<alias>2"` (truncated to keep it short), opens the editor prefilled.

## 8. Overlay structure additions in `app.go`

New fields on `App`:

```go
connManagerFlex    *tview.Flex
connManagerList    *tview.List
connManagerHints   *tview.TextView
connManagerVisible bool

connEditorFlex    *tview.Flex
connEditorForm    *tview.Form
connEditorList    *tview.List  // Submit / Cancel
connEditorVisible bool
```

Both are added to `rootPages` at construction time (same as move-picker etc.).
`showConnectionManager()` and `showConnectionEditor()` are the entry points.

## 9. Testing

- `config_test.go`: migration from legacy fields, `ActiveConn()` fallback,
  round-trip with `connections` shape, `Default()` fields.
- `topbar_test.go`: `infoPanelText` with alias line.
- Connection manager and editor overlays are wired to tview so they're tested
  manually (declared untestable in the unit-test sense).

## Files touched

| File | Change |
|---|---|
| `tui/internal/config/config.go` | `Connection` struct; remove legacy top-level fields; `Default()`; migration; `ActiveConn()` |
| `tui/internal/config/config_test.go` | migration, `ActiveConn`, round-trip, `Default()` |
| `tui/internal/app/app.go` | `newBackendForConn`; `switchConnection`; conn overlay fields; update backend init; `rootPages` wiring |
| `tui/internal/app/topbar.go` | `infoPanelText` with Connection alias line |
| `tui/internal/app/topbar_test.go` | update / add tests for new info panel format |
| `tui/internal/app/settings.go` | Connection button |
| `tui/internal/app/connections.go` | new — manager + editor overlays |
| `tui/config.example.yaml` | new shape with `activeConnection` / `connections` |
