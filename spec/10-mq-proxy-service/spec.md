# mq-proxy: standalone ActiveMQ REST proxy service

_Condensed from spec/20, spec/38 — see those folders for the incremental history. The exact REST routes/DTOs sketched in the original FE 20 spec were later reworked (CR 44/45/49/51, spec/11) to match a reference API; this document covers the service's architecture and purpose, not the wire contract — see spec/11 for that._

## Purpose

AWS Amazon MQ (managed ActiveMQ) does not expose Jolokia, so cloudtui's
Jolokia-based queue backend (spec/07) cannot talk to it. `mq-proxy/`
is a standalone service, independent of the TUI, that exposes a REST/JSON
API over the same broker operations and translates them to native JMS
(OpenWire) calls via `activemq-client`. The TUI's `proxy.Client` backend
(spec/11) talks to this service instead of Jolokia; the proxy is
broker-agnostic from the TUI's perspective — the same proxy can front a
local/self-hosted ActiveMQ or an AWS AMQ instance.

## Architecture

- **Kotlin + Spring Boot**, Gradle with Kotlin DSL (`build.gradle.kts`),
  living in `mq-proxy/` at the repo root — a fully separate module from
  `tui/`, no shared build.
- Spring Boot's embedded Tomcat means the proxy runs as a single JAR, no
  external app server.
- `activemq-client` (OpenWire) does the actual broker communication —
  `QueueBrowser` for non-destructive browsing, transacted JMS sessions for
  move operations, `MessageConsumer`/`MessageProducer` for
  delete/send/purge.
- Package layout: `config/` (`BrokerProperties`, `JmsConfig`,
  `SecurityConfig`), `api/` (`QueueController` + response/request data
  classes), `service/` (`BrokerService`, the JMS operations layer).
- Configuration via `application.yml`, overridable by environment variables
  (`BROKER_URL`, `BROKER_USERNAME`, `BROKER_PASSWORD`, `SERVER_PORT`).
  Broker connection (`ActiveMQConnectionFactory`) is constructed lazily —
  no socket opened at application startup, so the app boots cleanly even
  with no broker reachable yet.
- HTTP Basic auth on all `/api/**` endpoints (`proxy.auth.username`/
  `proxy.auth.password`), CSRF disabled (stateless REST API).
- Dockerfile (`eclipse-temurin:21-jre` base) for containerized deployment,
  built/run via podman; runs as a sidecar alongside the TUI or as a shared
  network service.

## Behavior covered

The service exposes broker operations equivalent to everything the TUI
needs: list queues (with pending/consumer/enqueue/dequeue counts), browse
messages (with filtering — see spec/11), get a single message,
delete a message, move a single message, move all messages between queues,
send a new message, and purge a queue. See spec/11 for the exact
route shapes, request/response DTOs, and the filter query-param
conventions as they exist today.

## API documentation

- `springdoc-openapi-starter-webmvc-ui` serves a live, dynamically
  generated OpenAPI document at `/v3/api-docs` (JSON) and
  `/v3/api-docs.yaml` (YAML), plus interactive Swagger UI at
  `/swagger-ui.html`. Both the docs endpoints and Swagger UI are
  `permitAll` (no auth) so the spec/UI are reachable without pre-supplying
  credentials; Swagger UI's own **Authorize** button accepts Basic
  credentials once and applies them to "Try it out" requests.
- A **static, committed export** lives at `mq-proxy/openapi.yaml` —
  generated *from* the live springdoc output (never hand-maintained, to
  avoid drift) via the `springdoc-openapi-gradle-plugin`, which forks a
  real Spring Boot run of the app, fetches `/v3/api-docs.yaml`, writes it
  to the file, then stops. Regenerated on demand via `task openapi:proxy`
  (`./gradlew generateOpenApiDocs` under the hood) — a deliberate step run
  after an API-visible change, not part of every build or CI.
- Gradle's own daemon JVM is pinned to 21 via `updateDaemonJvm`
  (`gradle/gradle-daemon-jvm.properties`, committed) plus the Foojay
  toolchain resolver plugin in `settings.gradle.kts` — required because the
  plugin's forked Spring Boot run has no toolchain hook of its own and
  otherwise inherits whatever JVM is running Gradle, which can mismatch the
  project's Java 21 compile target and fail with
  `UnsupportedClassVersionError`.

## Testing

- `BrokerServiceTest` — unit tests with mocked JMS (`Connection`,
  `Session`, `QueueBrowser`, `MessageConsumer`, `MessageProducer` via
  MockK).
- `QueueControllerTest` — `@WebMvcTest` slice tests with `BrokerService`
  mocked, covering happy paths, error-status mapping, and unauthenticated
  401s.
- `./gradlew test` from `mq-proxy/`.

## Out of scope (deliberate)

- Making the proxy the TUI's default backend (separate concern — see
  spec/11 and spec/12 for connection-level backend
  selection).
- Quarkus/native compilation.
- Non-text (binary/object) message bodies.
- Multi-broker/multi-tenant support within one proxy instance.
- Admin operations (create/delete queue).
- AMQP 1.0 or STOMP backends — OpenWire only.
- CI enforcement that the committed `openapi.yaml` matches a fresh
  generation.
- Client SDK/codegen from the spec.
