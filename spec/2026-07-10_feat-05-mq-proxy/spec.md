# 2026-07-10 — mq-proxy service

## Feature

Build the `mq-proxy` Spring Boot service CLAUDE.md's architecture calls
for — the full set of queue operations (list/browse/send/purge/move)
over JMS/OpenWire, a local embedded-broker dev stack, an OpenAPI-first
contract generating both the Spring controller interfaces and the tui's
Go client, and wiring the tui `queues` view to use it as a real,
interactive view instead of a placeholder.

## What

- **`mq-proxy/` Spring Boot module** (Maven, wrapper committed) exposing:
  - `GET /queues` — list queues with pending/consumer counts (via
    ActiveMQ's statistics plugin).
  - `GET /queues/{name}/messages` — browse messages (via `QueueBrowser`),
    paginated.
  - `POST /queues/{name}/messages` — send a message.
  - `POST /queues/{name}/purge` — purge all messages (transacted
    consume).
  - `POST /queues/{name}/move` — move messages from one queue to
    another (transacted consume + send).
- **HTTP Basic Auth** on every endpoint, both locally and once deployed —
  credentials come from Secrets Manager in AWS, from local config for
  the `local` profile. This replaces the bearer-token approach
  previously documented in `CLAUDE.md` (now updated).
- **`local` Spring profile**: an embedded ActiveMQ `BrokerService` (TCP
  connector) started in the same JVM — no Docker required for local dev,
  per the project's cross-platform requirement.
- **`api/` OpenAPI spec** as the single source of truth for the above
  contract, with build-time codegen for the Spring controller interfaces
  and the tui's Go client.
- **Taskfile wiring**: `build:mq-proxy`/`run:mq-proxy`/`test:mq-proxy`
  targets, with the top-level `build`/`test` targets gaining mq-proxy as
  a second dependency (a gap already flagged in the original Taskfile
  spec).
- **tui integration**: a `QueueBackend` Go interface with a `proxy`
  implementation (using the generated Go client) that the `queues` view
  uses — becoming a real, interactive view (queue list, browse/detail
  pane, send/purge/move actions) instead of a placeholder, the same
  progression the `settings` view already went through.

## Why

Amazon MQ's Jolokia API is read-only and remote JMX access isn't
available, so a companion proxy is the only way for the tui to actually
operate on queues — locally or in AWS. This is the last major piece
missing before cloudtui can do anything beyond browsing AWS profiles.

## Scope

- The full `mq-proxy` service: all five endpoints, HTTP Basic Auth, the
  `local` embedded-broker profile.
- The OpenAPI contract and its build-time codegen (both languages).
- Taskfile updates for the new module.
- The tui-side `QueueBackend` interface, `proxy` implementation, and a
  real `queues` view built on it.
- `CLAUDE.md`'s architecture section updated to reflect Basic Auth
  (already done, ahead of this spec, since it's a standing document
  correction rather than part of the feature itself).

## Out of scope

- Actual AWS deployment (VPC wiring, hosting, real Secrets Manager
  plumbing for deployed credentials) — this pass is the service, the
  local dev stack, and the tui integration; deploying it is a separate
  future feature.
- Amazon MQ's RabbitMQ engine — ActiveMQ/OpenWire only.
- A direct `jolokia` read-only backend — CLAUDE.md already calls this out
  as a possible *later*, optional addition, not this pass.
- Message schema/content validation, dead-letter handling, or any
  broker-side business logic beyond the five generic operations.
- Search/filter UX beyond what the existing generic `/`-filter
  scaffolding (`ui.Filterable`) already provides, if the queues view ends
  up implementing it.

## A note on size

Given the chosen scope, this is by far the largest feature built in this
repo so far — a new Java module, a cross-language generated contract,
JMS wiring, and a new interactive tui view. Even as one spec, the
implementation plan's task breakdown will necessarily be long, and per
the standing workflow, each task still gets approved individually before
being implemented — this will take many rounds, not one.
