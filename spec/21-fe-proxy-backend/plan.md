# Plan — FE 21: mq-proxy backend for the TUI

## Approach

Minimal addition: one new package, two modified files, one new config struct.
No changes to existing `jolokia.Client` or the `queue.Backend` interface.

**As delivered:** only the `tui/internal/queue/proxy/` package below was built
to this plan. The config/`app.go` wiring described in this section was
superseded by FE 22 (named connections), which was already underway when this
part was implemented — `ProxyConfig` ended up hanging off
`config.Connection` rather than the top-level `Config`, and backend selection
happens in `newBackendForConn(conn config.Connection)` in `app.go`. See
`spec/22-fe-connections/plan.md`.

## Config changes (`tui/internal/config/config.go`) — superseded, see note above

Add a `Backend` string field to `Config` (default `"jolokia"`). Add a
`ProxyConfig` struct with `URL`, `Username`, `Password` fields. Wire the
same existing env-var pattern for the proxy password under
`MQPROXY_CLIENT_PASSWORD` if `cfg.Proxy.Password` is empty.

```go
type Config struct {
    Backend string      `yaml:"backend"` // "jolokia" (default) or "proxy"
    Theme   string      `yaml:"theme"`
    Queue   QueueConfig `yaml:"queue"`
    Proxy   ProxyConfig `yaml:"proxy"`
    Logo    []string    `yaml:"logo"`
    Colors  Palette     `yaml:"colors"`
}

type ProxyConfig struct {
    URL      string `yaml:"url"`
    Username string `yaml:"username"`
    Password string `yaml:"password"`
}
```

`Default()` sets `Backend: "jolokia"` so existing configs without the field
keep the existing behaviour.

## New package `tui/internal/queue/proxy/`

`proxy.Client` wraps `net/http.Client` (30 s timeout) and calls the
mq-proxy REST endpoints with HTTP Basic auth.

### Request helpers

`doRequest(ctx, method, path, body io.Reader, out interface{}) error`
- Builds the full URL from `baseURL + path`.
- Sets `Authorization: Basic …`.
- Sets `Content-Type: application/json` only when `body != nil`.
- On HTTP 4xx/5xx reads the response body and returns it as an error string.
- When `out != nil` and status is not 204, JSON-decodes the response into `out`.

Path segments are encoded with `url.PathEscape`; query values with
`url.QueryEscape`.

### Response → `queue.Message` mapping

`BrowseMessages` calls `GET /api/queues/{name}/messages` which returns
`[]MessageSummary` (id, timestamp, body?, properties).

```
proxy field      → queue.Message field
id               → ID
timestamp        → Timestamp (time.RFC3339)
body (if set)    → JMSType="text"; Preview=first 80 chars; RawFields["text"]=full body
body (nil)       → JMSType="other"; Preview=""; RawFields["text"]=""
properties       → RawFields["properties"] (map[string]interface{})
```

`message_detail.go` reads `RawFields["text"]` for the body section and
`RawFields["properties"]` for the properties section. The other JMS header
keys (`jMSDeliveryMode`, `jMSDestination`, etc.) are not available from
`MessageSummary`; those fields will render as `<nil>` in the detail view,
which is acceptable — the data simply isn't in the browse payload.

### Operation mapping

| Backend method    | HTTP call |
|---|---|
| `List`            | `GET /api/queues` |
| `BrowseMessages`  | `GET /api/queues/{name}/messages` |
| `PurgeQueue`      | `DELETE /api/queues/{name}/messages` (ignores `{ purged: N }`) |
| `RemoveMessage`   | `DELETE /api/queues/{name}/messages/{id}` |
| `MoveMessage`     | `POST /api/queues/{name}/messages/{id}/move?to={dest}` |
| `MoveAllMessages` | `POST /api/queues/{name}/move?to={dest}` (reads `{ moved: N }`) |
| `SendMessage`     | `POST /api/queues/{name}/messages` (raw body, `Content-Type: application/json`) |

## Backend selection in `app.go` — superseded, see note above

Replace the single `jolokia.NewClient(cfg.Queue)` call with:

```go
if cfg.Backend == "proxy" {
    a.backend = proxy.NewClient(cfg.Proxy)
} else {
    a.backend = jolokia.NewClient(cfg.Queue)
}
```

No other changes in `app.go`; import the `proxy` package.

## Testing

`proxy_test.go` uses `net/http/httptest.NewServer` — no external dependency
needed. One test per Backend method; cover the happy path and at least one
error case (non-2xx response).

`config_test.go` adds:
- `TestDefaultBackend` — `Default().Backend == "jolokia"`
- `TestLoadBackendProxy` — loading `backend: proxy\nproxy:\n  url: …` round-trips
- `TestDefaultProxyConfigEmpty` — `Default().Proxy` fields are all empty

## No new dependencies

`net/http`, `net/url`, `encoding/json`, `io`, `strings`, `time` are all
standard library — no additions to `go.mod`.
