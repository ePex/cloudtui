# Tasks — AMQ queue list via Jolokia

Plan: [plan.md](plan.md)

1. [ ] **`infra/compose.yaml`** — create `infra/` directory with a
   Podman/Docker Compose file running `apache/activemq-classic:latest`,
   exposing ports 8161 (web console / Jolokia) and 61616 (OpenWire);
   environment sets `admin`/`admin` credentials matching `Default()`.

2. [x] **`QueueConfig` in config** — add `QueueConfig` struct (`BrokerName`,
   `URL`, `Username`, `Password`) and a `Queue QueueConfig` field to
   `Config`; `Default()` sets `BrokerName:"localhost"`,
   `URL:"http://localhost:8161/api/jolokia"`, `Username:"admin"`,
   `Password:""`. `Load()` injects `MQPROXY_CLIENT_PASSWORD` env var into
   `cfg.Queue.Password` when the field is empty. Tests: defaults populated,
   env-var injection, env-var does not override explicit password,
   round-trip save/load.

3. [x] **`queue.Backend` interface** — create
   `tui/internal/queue/backend.go` with `Summary` struct (`Name string`,
   `PendingCount int64`, `ConsumerCount int64`) and `Backend` interface
   (`List(ctx context.Context) ([]Summary, error)`).

4. [x] **Jolokia client** — create
   `tui/internal/queue/jolokia/jolokia.go` implementing `queue.Backend`:
   `NewClient(cfg config.QueueConfig) *Client`; `List()` does a Jolokia
   `search` for queue MBeans then a bulk `read` for `QueueSize` and
   `ConsumerCount`; sets `Origin` and `Authorization` headers on every
   request; treats non-200 Jolokia status as an error. Tests in
   `jolokia_test.go` against an `httptest.Server`: happy path, HTTP error,
   Jolokia error status.

5. [x] **Queues view** — create `tui/internal/app/queues.go`:
   `queuesView` implementing `ui.View`, `ui.Shortcuttable`, and
   `activatable`; bordered `tview.Table` with Name / Pending / Consumers
   header row; `Activate()` fires a goroutine calling `backend.List()` +
   `QueueUpdateDraw`; `r` shortcut also reloads; `repaint()` populates rows
   using palette colors; backend errors are logged via `slog.Error` and
   displayed in the table. Tests in `queues_test.go`: header labels, column
   count, shortcut `r` present.

6. [x] **App wiring** — in `app.go`: add `backend queue.Backend` field;
   construct `jolokia.NewClient(cfg.Queue)` in `New()`; create and register
   `queuesView`; add `"Queues"` entry to home dashboard. Tests: queues view
   registered, `:queues` command switches to it.

7. [x] **`reapplyTheme` for queues** — in `theme.go`: repaint queues table
   background and border/title colors; add `"queues"` key to both
   `DarkPalette()` and `CyberpunkPalette()` `Views` maps.

8. [x] **Docs** — add `queue:` section to `tui/config.example.yaml`;
   update `tui/CLAUDE.md` architecture section to mention the `queue`
   packages.

9. [ ] **Manual verification** — run `podman compose -f infra/compose.yaml
   up -d`; run `task run:tui`; confirm Queues view appears on home
   dashboard; confirm queue list loads from the broker; confirm `r`
   refreshes; confirm theme switch repaints the view correctly.
