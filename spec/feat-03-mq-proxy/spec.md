# mq-proxy service

Date: 2026-07-10.

## Feature

Build the `mq-proxy` Spring Boot service `CLAUDE.md`'s architecture calls
for — the full set of queue operations (list/browse/send/purge/move)
over JMS/OpenWire, a local embedded-broker dev stack, an OpenAPI-first
contract generating both the Spring controller interfaces and the tui's
Go client, and wiring the tui `queues` view to use it as a real,
interactive view instead of a placeholder.

## What

- **`mq-proxy/` Spring Boot module** (Maven, wrapper committed) exposing:
  - `GET /queues` — list queues with pending/consumer counts (via
    ActiveMQ's statistics-query mechanism).
  - `GET /queues/{name}/messages` — browse messages (via `QueueBrowser`),
    paginated client-side.
  - `POST /queues/{name}/messages` — send a message.
  - `POST /queues/{name}/purge` — purge all messages (transacted
    consume).
  - `POST /queues/{name}/move` — move messages from one queue to
    another (transacted consume + send, single commit).
- **HTTP Basic Auth** on every endpoint, both locally and once deployed
  — credentials from Secrets Manager in AWS, from local config for the
  `local` profile.
- **`local` Spring profile**: an embedded ActiveMQ `BrokerService` (TCP
  connector) started in the same JVM — no Docker required for local dev.
- **`api/` OpenAPI spec** as the single source of truth, with build-time
  codegen for the Spring controller interfaces and the tui's Go client.
- **Taskfile wiring**: `build:mq-proxy`/`run:mq-proxy`/`test:mq-proxy`
  targets, `generate:mqproxy-client`; top-level `build`/`test`/`generate`
  gain them as dependencies.
- **tui integration**: a `QueueBackend` Go interface with a `proxy`
  implementation (using the generated Go client) that the `queues` view
  uses — becoming a real, interactive view (queue list, browse/detail
  pane, send/purge/move actions) instead of a placeholder, the same
  progression `settings` already went through in
  `feat-02-tui-shell-and-starting-features`.

## Why

Amazon MQ's Jolokia API is read-only and remote JMX access isn't
available, so a companion proxy is the only way for the tui to actually
operate on queues — locally or in AWS. This was the last major piece
missing before cloudtui could do anything beyond browsing AWS profiles.

## Scope

- The full `mq-proxy` service: all five endpoints, HTTP Basic Auth, the
  `local` embedded-broker profile.
- The OpenAPI contract and its build-time codegen (both languages).
- Taskfile updates for the new module.
- The tui-side `QueueBackend` interface, `proxy` implementation, and a
  real `queues` view built on it.

## Out of scope

- Actual AWS deployment (VPC wiring, hosting, real Secrets Manager
  plumbing) — this is the service, the local dev stack, and the tui
  integration; deploying it is separate future work.
- Amazon MQ's RabbitMQ engine — ActiveMQ/OpenWire only.
- A direct `jolokia` read-only backend — a possible later, optional
  addition, not this pass.
- Message schema/content validation, dead-letter handling, or any
  broker-side business logic beyond the five generic operations.

## A note on size

This was, by a wide margin, the largest single feature built in the repo
at the time — a new Java module, a cross-language generated contract,
JMS wiring, and a new interactive tui view — implemented across 23
individually-approved tasks per the standing per-task-approval rule.
