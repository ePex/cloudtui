# Spec — FE 21: mq-proxy backend for the TUI

Date: 2026-08-07

## Background

The TUI talks to ActiveMQ via a `queue.Backend` interface
(`tui/internal/queue/backend.go`). The only current implementation is
`jolokia.Client` (`tui/internal/queue/jolokia/`), which calls the Jolokia
HTTP/JMX API directly.

FE 20 introduced `mq-proxy` — a sidecar REST service that exposes the same
broker operations over a clean JSON API without requiring Jolokia. This
feature wires the TUI to that proxy.

## Problem

Users running AWS Amazon MQ (or any broker without Jolokia) cannot use the
TUI because the only backend is Jolokia-specific.

## Solution

Add a second `queue.Backend` implementation — `proxy.Client` in
`tui/internal/queue/proxy/` — that calls the mq-proxy REST API. The active
backend is selected by a new `backend` field in `config.yaml`:

```yaml
backend: jolokia   # or: proxy
```

When `backend: proxy` is set, the TUI constructs a `proxy.Client` instead of
`jolokia.Client`; the rest of the application is unchanged.

## Scope

### In scope

- New package `tui/internal/queue/proxy/` implementing `queue.Backend` by
  calling the mq-proxy REST endpoints.
- New `backend` config field (`jolokia` | `proxy`); defaults to `jolokia`
  for backwards compatibility.
- New `proxy` section in `config.yaml` for the proxy URL and credentials:
  ```yaml
  backend: proxy
  proxy:
    url: http://localhost:8080
    username: cloudtui
    password: changeme
  ```
- Backend selection in `app.New()` based on the config field.
- `config.example.yaml` updated to document the new fields.
- Unit tests for `proxy.Client` (HTTP responses mocked with `httptest`).

### Out of scope

- Runtime backend switching (requires app restart to change backend).
- UI for selecting the backend (settings dropdown etc.).
- Any changes to `jolokia.Client` or its config fields.
- Message detail view field mapping beyond what `queue.Message` already
  holds (the proxy returns all needed fields).

## API mapping

| `queue.Backend` method | mq-proxy endpoint |
|---|---|
| `List` | `GET /api/queues` |
| `BrowseMessages` | `GET /api/queues/{name}/messages` |
| `PurgeQueue` | `DELETE /api/queues/{name}/messages` |
| `RemoveMessage` | `DELETE /api/queues/{name}/messages/{id}` |
| `MoveMessage` | `POST /api/queues/{name}/messages/{id}/move?to={dest}` |
| `MoveAllMessages` | `POST /api/queues/{name}/move?to={dest}` |
| `SendMessage` | `POST /api/queues/{name}/messages` (JSON body) |

## Response mapping

mq-proxy response fields → `queue.Message` fields:

| proxy field | `queue.Message` field |
|---|---|
| `id` | `ID` |
| `timestamp` (ISO-8601) | `Timestamp` |
| `body` | `Preview` (truncated to 80 chars); full body in `RawFields["body"]` |
| `deliveryMode`, `priority`, `correlationId`, `replyTo`, `destination`, `redelivered` | `RawFields` entries |
| `properties` | merged into `RawFields` |

`JMSType` → `"text"` when body is present, `"other"` when nil.

## Files touched

| File | Change |
|---|---|
| `tui/internal/queue/proxy/proxy.go` | new — `proxy.Client` implementing `queue.Backend` |
| `tui/internal/queue/proxy/proxy_test.go` | new — `httptest`-based unit tests |

**Note:** the config wiring and backend selection originally scoped here (top-level
`Config.Backend` / `Config.Proxy` fields, `app.go` selecting on `cfg.Backend`) was
not delivered as its own step. By the time it was implemented, FE 22 (named
connections) was in progress, so the proxy backend was wired directly into
`config.Connection.Backend` / `config.Connection.Proxy` instead of standalone
top-level fields — see `spec/22-fe-connections/spec.md` for that config shape
and `tui/internal/app/app.go`'s `newBackendForConn`. `config.go`,
`config_test.go`, `app.go`, and `config.example.yaml` are therefore tracked
under FE 22, not here.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. With `backend: proxy` and mq-proxy running, the TUI lists queues,
   browses messages, and performs all write operations.
3. With `backend: jolokia` (or field absent), existing Jolokia behaviour
   is unchanged.
4. `config.example.yaml` documents both backends.
