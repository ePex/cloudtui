# Spec — FE 20: MQ Proxy Service

Date: 2026-08-07

## Problem

The cloudtui TUI communicates with Apache ActiveMQ via the Jolokia HTTP/JMX
API. AWS Amazon MQ does not expose Jolokia, making cloudtui incompatible with
managed AWS AMQ brokers.

## Solution

Introduce an `mq-proxy/` service at repo root: a Kotlin + Spring Boot
application that exposes the same HTTP/JSON API the TUI already calls, and
translates those calls to native JMS operations against an ActiveMQ broker via
the `activemq-client` (OpenWire) library.

The TUI connects to the proxy instead of directly to Jolokia. The proxy is
broker-agnostic from the TUI's perspective — the same proxy can front a
local/self-hosted ActiveMQ or an AWS AMQ instance.

## Scope

### In scope

- `mq-proxy/` directory at repo root containing a standalone Spring Boot
  application (Kotlin, Gradle + Kotlin DSL).
- REST endpoints covering all operations the TUI currently performs via
  Jolokia:
  - **List queues** — returns queue names, pending message count, consumers
    count, enqueue/dequeue counts.
  - **Browse messages** — returns messages in a queue (non-destructive),
    including ID, body, timestamp, headers, and string properties.
  - **Get message detail** — returns a single message by ID.
  - **Delete message** — removes a single message by JMS message ID.
  - **Move message** — moves a single message to a named destination queue
    (atomic, within a JMS session transaction).
  - **Move all messages** — moves all messages from one queue to another.
  - **Send message** — publishes a new text message to a named queue.
  - **Purge queue** — removes all messages from a queue.
- JSON response shape compatible with what `mq-proxy/` client code in the TUI
  expects (new client, not Jolokia-shaped — the TUI's broker client will grow
  a second backend alongside the existing Jolokia client).
- Configuration via `application.yml`: broker URL, credentials, and HTTP port.
- Basic HTTP authentication on all proxy endpoints (configurable username/password).
- Swagger UI (`/swagger-ui.html`) for interactive API exploration — publicly
  accessible (no auth), with an **Authorize** button that accepts Basic
  credentials once and applies them to all subsequent "Try it out" requests;
  generated automatically from controller annotations via
  `springdoc-openapi-starter-webmvc-ui` + an `OpenApiConfig` bean that
  declares the HTTP Basic security scheme globally.
- Dockerfile for containerised deployment (built and run via podman).
- `mq-proxy/README.md` — build, run, and configuration instructions.

### Out of scope

- Switching the TUI to use the proxy by default (separate feature).
- Quarkus/native compilation (can be revisited later).
- Message selectors / filtering server-side beyond what JMS supports natively.
- Support for non-text (binary/object) message bodies.
- Multi-broker / multi-tenant support.
- Admin operations (create queue, delete queue).
- AMQP 1.0 or STOMP backends (OpenWire only for now).
- UI changes to select backend type (Jolokia vs proxy).

## Why Kotlin + Spring Boot

- Spring Boot has first-class Kotlin support (data classes, null safety,
  idiomatic REST controllers).
- `activemq-client` is a Java library with seamless Kotlin interop.
- Gradle with Kotlin DSL (`build.gradle.kts`) keeps the build consistent with
  the Kotlin source.
- Spring Boot's embedded Tomcat means the proxy runs as a single JAR with no
  external server.
- Quarkus + GraalVM native can be explored later if startup time or image size
  become a concern.

## JMS operations

| TUI operation     | JMS API used                                                      |
|-------------------|-------------------------------------------------------------------|
| List queues       | `ActiveMQSession.getDestinations()` or advisory topics            |
| Browse messages   | `QueueBrowser` (non-destructive enumeration)                      |
| Delete message    | `MessageConsumer` with selector `JMSMessageID = '<id>'` + ACK    |
| Move message      | JMS transaction: consume (selector) from src + send to dst        |
| Move all          | JMS transaction: loop consume + send until queue empty            |
| Send message      | `MessageProducer.send(TextMessage)`                               |
| Purge queue       | `MessageConsumer` loop consume + ACK until queue empty            |

## REST API shape (proposed)

```
GET  /api/queues                          → list of queue summaries
GET  /api/queues/{name}/messages          → paginated message list
GET  /api/queues/{name}/messages/{id}     → single message detail
DELETE /api/queues/{name}/messages/{id}   → delete message
POST /api/queues/{name}/messages/{id}/move?to={dest}  → move message
POST /api/queues/{name}/move?to={dest}    → move all messages
POST /api/queues/{name}/messages          → send new message (body in request body)
DELETE /api/queues/{name}/messages        → purge queue
```

All responses are JSON. Errors return `{ "error": "<message>" }` with an
appropriate HTTP status code.

## Deployment

- Runs as a sidecar alongside the TUI, or as a shared service on the network.
- Default port: `8080` (configurable).
- Broker URL and credentials set in `application.yml` or via environment
  variables (`BROKER_URL`, `BROKER_USERNAME`, `BROKER_PASSWORD`).
- Dockerfile produces a JVM image (`eclipse-temurin:21-jre` base); built and
  run with podman.

## Files introduced

| Path | Description |
|------|-------------|
| `mq-proxy/` | New module root |
| `mq-proxy/build.gradle.kts` | Gradle build |
| `mq-proxy/settings.gradle.kts` | Gradle settings |
| `mq-proxy/src/main/kotlin/…/MqProxyApplication.kt` | Spring Boot entry point |
| `mq-proxy/src/main/kotlin/…/QueueController.kt` | REST endpoints |
| `mq-proxy/src/main/kotlin/…/BrokerService.kt` | JMS operations |
| `mq-proxy/src/main/resources/application.yml` | Default configuration |
| `mq-proxy/Dockerfile` | Container image |
| `mq-proxy/README.md` | Build and run instructions |
| `mq-proxy/src/test/kotlin/…/BrokerServiceTest.kt` | Unit tests (mocked JMS) |
| `mq-proxy/src/test/kotlin/…/QueueControllerTest.kt` | Controller slice tests |

## Definition of done

1. `./gradlew build` in `mq-proxy/` passes with all tests green.
2. Proxy starts and connects to a local ActiveMQ broker.
3. TUI (pointed at proxy URL) can list queues, browse, send, delete, move, purge.
4. Podman image builds (`task build:proxy`) and proxy runs in container.
5. Swagger UI is accessible at `http://localhost:8080/swagger-ui.html`.
6. `mq-proxy/README.md` documents setup and configuration.
