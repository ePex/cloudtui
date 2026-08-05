# Spec 04 — AMQ queue list via Jolokia

Date: 2026-08-05

## What

Add a read-only **Queues** view that lists all queues on a local ActiveMQ
broker by talking directly to the broker's built-in Jolokia HTTP endpoint.
A `compose.yaml` makes it trivial to spin up the broker with Podman.

### Queue list

A new `Queues` view shows a table with three columns: **Name**, **Pending**
(messages waiting), **Consumers** (active consumers). The view is reachable
via the home dashboard and via `:queues`. Pressing `r` reloads the list from
the broker. The view uses the `activatable` interface — entering it triggers
an automatic reload.

No filtering, sorting, or action modals in this iteration.

### Connection configuration

A `queue:` section in `config.yaml` holds the broker connection settings:

```yaml
queue:
  brokerName: localhost
  url: http://localhost:8161/api/jolokia
  username: admin
  password: ""
```

`Default()` populates this with the localhost values above. `Load()` injects
`MQPROXY_CLIENT_PASSWORD` from the environment into `Password` when the field
is empty, so credentials stay out of the config file. The queue section is
documented in `config.example.yaml`.

### Local infrastructure

`infra/compose.yaml` (Podman/Docker Compose) defines an `activemq` service
using the `apache/activemq-classic` image, exposing:
- `8161` — web console / Jolokia
- `61616` — OpenWire broker port

Running `podman compose -f infra/compose.yaml up -d` (or
`docker compose -f infra/compose.yaml up -d`) gives a ready broker with the
default `admin`/`admin` credentials that match `Default()`.

## Why

The tool's primary purpose is queue management. This feature wires in the
first real backend so that the queues view shows live data rather than being
a placeholder. Keeping this iteration read-only and filter-free means the
plumbing (Jolokia client, config, view) can be verified end-to-end before
adding actions and UX refinements.

Podman Compose support means any developer can run a local broker with one
command, matching the cross-platform requirement in the root CLAUDE.md.

## Scope

- `tui/internal/config/config.go` — `QueueConfig` struct (`BrokerName`,
  `URL`, `Username`, `Password`); add `Queue QueueConfig` field to `Config`;
  `Default()` sets localhost jolokia defaults; `Load()` injects
  `MQPROXY_CLIENT_PASSWORD` when `Password` is empty.
- `tui/internal/config/config_test.go` — defaults, env-var injection,
  round-trip save/load.
- `tui/internal/queue/backend.go` — `Summary` struct (`Name string`,
  `PendingCount int64`, `ConsumerCount int64`) and `Backend` interface
  with a single method `List(ctx context.Context) ([]Summary, error)`.
- `tui/internal/queue/jolokia/jolokia.go` — `Client` implementing
  `queue.Backend`; `List()` reads queue summaries from the broker MBean
  over Jolokia HTTP using stdlib `net/http` + `encoding/json` only.
- `tui/internal/queue/jolokia/jolokia_test.go` — `List()` happy path and
  error cases against an `httptest.Server`.
- `tui/internal/app/queues.go` — `queuesView` implementing `ui.View`,
  `ui.Shortcuttable`, and `activatable`; `tview.Table` with Name/Pending/
  Consumers columns; `Activate()` and `r` reload from the backend.
- `tui/internal/app/queues_test.go` — repaint, shortcut labels, reload.
- `tui/internal/app/app.go` — construct jolokia backend from config;
  register queues view; add queues entry to home dashboard.
- `tui/internal/app/theme.go` — repaint queues table on theme switch; add
  `"queues"` to both palette `Views` maps.
- `tui/config.example.yaml` — document the `queue:` section.
- `infra/compose.yaml` — Podman/Docker Compose file for local ActiveMQ.
- Unit tests throughout.

## Out of scope

- Browse, send, purge, move — future features.
- Named connection profiles / connection CRUD — future feature.
- Queue filtering or sorting — future feature.
- In-app Podman control — users run compose manually.
