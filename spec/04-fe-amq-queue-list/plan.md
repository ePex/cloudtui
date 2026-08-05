# Plan — AMQ queue list via Jolokia

Spec: [spec.md](spec.md)

## Approach

### Config (`tui/internal/config/config.go`)

Add a `QueueConfig` struct and a `Queue QueueConfig` field to `Config`:

```go
type QueueConfig struct {
    BrokerName string `yaml:"brokerName"`
    URL        string `yaml:"url"`
    Username   string `yaml:"username"`
    Password   string `yaml:"password"`
}
```

`Default()` sets:
```go
Queue: QueueConfig{
    BrokerName: "localhost",
    URL:        "http://localhost:8161/api/jolokia",
    Username:   "admin",
    Password:   "",
},
```

`Load()` checks `MQPROXY_CLIENT_PASSWORD` after unmarshalling: if the env
var is set and `cfg.Queue.Password` is empty, the env var value is written
into `cfg.Queue.Password`. This keeps credentials out of `config.yaml`.

### Backend interface (`tui/internal/queue/backend.go`)

Minimal interface scoped to this feature only:

```go
package queue

import "context"

type Summary struct {
    Name          string
    PendingCount  int64
    ConsumerCount int64
}

type Backend interface {
    List(ctx context.Context) ([]Summary, error)
}
```

### Jolokia client (`tui/internal/queue/jolokia/jolokia.go`)

`Client` holds the connection parameters and implements `queue.Backend`.

`List()` sends a Jolokia `search` request to find all queue MBeans, then a
bulk `read` request to fetch `QueueSize` and `ConsumerCount` attributes for
each. Both requests are plain HTTP POST with JSON bodies.

Two things to handle carefully:
- **Origin header**: ActiveMQ's default `jolokia-access.xml` rejects requests
  without a matching `Origin` header under strict-checking. We set
  `Origin: <URL>` on every request.
- **HTTP auth**: Basic Auth via `req.SetBasicAuth`.
- **Error detection**: A non-200 Jolokia status inside the JSON envelope is
  treated as an error even if the HTTP status is 200.

No new dependencies — `net/http` + `encoding/json` from stdlib.

### Queues view (`tui/internal/app/queues.go`)

`queuesView` implements `ui.View`, `ui.Shortcuttable`, and the `activatable`
interface already in `app.go`.

Internally it holds:
- `backend queue.Backend` — injected at construction
- `table *tview.Table` — the display primitive wrapped in a bordered Flex
- `queues []queue.Summary` — last fetched data

`Activate()` fires a goroutine that calls `backend.List()` and calls
`tv.QueueUpdateDraw` to repaint — keeping the event loop unblocked.

`Shortcuts()` returns `[{Key: "r", Description: "refresh"}]`.

`repaint()` writes the header row (Name / Pending / Consumers) and one data
row per summary. Cells use palette `Label` / `Value` colors.

### App wiring (`tui/internal/app/app.go`)

- Add `backend queue.Backend` field.
- In `New()`: construct `jolokia.NewClient(cfg.Queue)` and store it.
- Add a `queuesView` and register it alongside home/settings/log.
- Add a `"Queues"` entry to the home dashboard.

### Theme (`tui/internal/app/theme.go`)

Add a `"queues"` repaint block in `reapplyTheme` that updates the queues
table background and border/title colors. Add `"queues"` to both
`DarkPalette()` and `CyberpunkPalette()` `Views` maps.

### Infrastructure (`infra/compose.yaml`)

```yaml
services:
  activemq:
    image: apache/activemq-classic:latest
    ports:
      - "8161:8161"   # web console + Jolokia
      - "61616:61616" # OpenWire
    environment:
      ACTIVEMQ_CONNECTION_USER: admin
      ACTIVEMQ_CONNECTION_PASSWORD: admin
```

Podman and Docker Compose both work with this file.

## Files touched

- `tui/internal/config/config.go` (+ `config_test.go`)
- `tui/internal/queue/backend.go` (new)
- `tui/internal/queue/jolokia/jolokia.go` (new)
- `tui/internal/queue/jolokia/jolokia_test.go` (new)
- `tui/internal/app/queues.go` (new)
- `tui/internal/app/queues_test.go` (new)
- `tui/internal/app/app.go` (+ `app_test.go`)
- `tui/internal/app/theme.go`
- `tui/config.example.yaml`
- `infra/compose.yaml` (new)

## Key decisions / trade-offs

- **Synchronous Jolokia calls in a goroutine** — `List()` is called from a
  goroutine so the tview event loop is never blocked. `QueueUpdateDraw`
  ensures the repaint happens on the tview goroutine.
- **`activatable` already exists** — `log.go` introduced it; `queuesView`
  just implements the same interface.
- **Single `Queue QueueConfig` field** — simpler than named profiles for
  this iteration. A future feature will replace it with `Connections
  []Connection`.
- **`search` + bulk `read`** — two Jolokia HTTP round-trips per refresh.
  This is fine for a local broker; a remote broker with many queues may
  need batching, deferred to a later iteration.
- **No new Go module dependencies** — stdlib only, consistent with the
  existing module.

## Testing

- `internal/config`: `Default()` has `Queue` populated; env-var injection
  sets `Password` when field is empty; env-var does not override an
  explicit password; round-trip save/load preserves `Queue`.
- `internal/queue/jolokia`: `List()` happy path returns correct summaries;
  HTTP error returns error; Jolokia error status returns error — all against
  an `httptest.Server`.
- `internal/app/queues`: `repaint()` renders correct column count and
  header labels; `Shortcuts()` includes `r`; `Activate()` triggers a
  backend call.
