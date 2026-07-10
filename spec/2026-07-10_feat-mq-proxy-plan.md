# Plan — mq-proxy service

Spec: [2026-07-10_feat-mq-proxy.md](2026-07-10_feat-mq-proxy.md)

This is the largest feature in the repo so far. The plan below fixes the
architecture and file layout; a few Java-library specifics (exact
ActiveMQ client calls, exact oapi-codegen config flags) are deliberately
left to be confirmed against the actual libraries during implementation
rather than guessed here — same "verify, then decide" approach already
used for `yaml.v3`'s map-merge behavior earlier in this project.

## Approach

### `mq-proxy/` Spring Boot module

- Maven, wrapper committed (`mvnw`/`mvnw.cmd`/`.mvn/wrapper/`), Java 21
  (matches the JDK already required by `task doctor`). Group/base package
  `dev.cloudtui.mqproxy`.
- Dependencies: `spring-boot-starter-web`, `spring-boot-starter-security`
  (HTTP Basic Auth), `spring-boot-starter-jms`, `org.apache.activemq:activemq-client`
  (JMS/OpenWire), `org.apache.activemq:activemq-broker` (embedded
  broker, `local` profile only), `openapi-generator-maven-plugin`
  (build-time codegen).
- `application.yaml`: `mqproxy.broker.url` (defaults to the local
  embedded broker's `tcp://localhost:61616`, overridden via env var for
  a real Amazon MQ endpoint), broker JMS credentials, and
  `spring.security.user.name`/`password` for the HTTP Basic Auth layer —
  **two separate credential pairs**: one for callers of the proxy's
  HTTP API, one for the proxy's own JMS connection to the broker.
  Conflating them would be a real design smell.
- `application-local.yaml` (`local` profile): embedded `BrokerService`
  (in-memory, non-persistent, one TCP connector) started as a Spring
  bean (`initMethod = "start", destroyMethod = "stop"`); local HTTP
  Basic Auth credentials default to a fixed dev value overridable by env
  var, per CLAUDE.md's "no credentials in config files" (defaults are
  not secrets, just dev convenience).

### `api/openapi.yaml` — the contract

- `GET /queues` → `QueueSummary[]` (`name`, `pendingCount`,
  `consumerCount`).
- `GET /queues/{name}/messages?limit=` → `Message[]` (`id`, `body`,
  `properties`).
- `POST /queues/{name}/messages` (`SendMessageRequest`: `body`,
  `properties`) → 201.
- `POST /queues/{name}/purge` → 204.
- `POST /queues/{name}/move` (`MoveMessagesRequest`: `targetQueue`,
  `maxMessages?`) → 204.
- `security: [basicAuth: []]` on every operation.
- Error responses use a shared `ErrorResponse` schema (`message`).

### Codegen — different generators per side, on purpose

- **Java** (server): `openapi-generator-maven-plugin`, Spring
  interface-only + delegate pattern — generates `QueuesApi` (interface)
  + DTOs into `target/generated-sources/` (already covered by the
  existing `target/` `.gitignore` rule); `QueuesController` implements
  the generated delegate with the real JMS logic.
- **Go** (client): `oapi-codegen` — idiomatic, no JVM dependency for the
  Go build even though a JDK is already required for `mq-proxy` itself.
  Generated into `tui/internal/mqproxyclient/generated/`, deliberately
  under a directory literally named `generated` so it's covered by the
  existing `**/generated/` `.gitignore` rule rather than needing a new
  pattern. A committed `tui/internal/mqproxyclient/oapi-codegen-config.yaml`
  drives it.
- New Taskfile targets: `generate:mqproxy-client` (runs `oapi-codegen`
  against `api/openapi.yaml`) and a top-level `generate` aggregating it
  (the Java side's codegen runs automatically as part of `mvnw`'s own
  build lifecycle, no separate Task step needed). `build:tui`/`test:tui`/
  `run:tui` gain `generate:mqproxy-client` as a dependency, since the Go
  compiler needs the generated package to physically exist first.
- `build`/`test` (top-level) gain `build:mq-proxy`/`test:mq-proxy` as a
  second dependency — the gap already flagged in the original Taskfile
  spec.

### Queue listing without JMX or a broker-side plugin

Amazon MQ's JMX is unavailable and we don't control the managed broker's
XML config (so the `<statisticsBrokerPlugin/>` some ActiveMQ docs
mention isn't an option). Instead: `ActiveMQConnection.getDestinationSource()`
— a standard ActiveMQ *client* API — gives a live view of the queues a
broker knows about, local or remote, with no broker-side configuration
required. Per-queue pending/consumer counts come from ActiveMQ's
built-in statistics-query mechanism (send an empty message to
`ActiveMQ.Statistics.Destination.<name>` with a temporary reply queue,
read the `MapMessage` reply) — also a client-side technique, not a
plugin.

### The five operations (`QueueService`)

- **List**: as above.
- **Browse**: `Session.createBrowser(queue)`, enumerate up to `limit`
  messages (JMS `QueueBrowser` has no server-side pagination, so
  `limit` is applied client-side in the proxy).
- **Send**: `MessageProducer.send(...)` with a `TextMessage`, properties
  copied onto it.
- **Purge**: a transacted session, `MessageConsumer.receive(timeout)`
  looped until empty, then `commit()` — matches CLAUDE.md's existing
  "transacted consume" description.
- **Move**: a transacted session, consume from the source queue and
  send to the target queue, `commit()` at the end so it's atomic.

### tui integration

- **`tui/internal/queue`** (new): `Backend` interface (`List`, `Browse`,
  `Send`, `Purge`, `Move`, all `context`-aware) and domain types
  (`Summary`, `Message`) independent of the generated client's types —
  same "AWS calls live in internal service wrappers, never in UI code"
  separation already followed for AWS.
- **`tui/internal/queue/proxy`** (new): the real `Backend`
  implementation, wrapping the generated Go client and translating
  between its types and the domain types above.
- **Config**: a new `Queue` section (`ProxyURL`, `Username`, `Password`)
  in `config.Config` — this is the "Queue Connection" setting explicitly
  deferred in the AWS-profile-selection spec. `Password` is overridable
  via an env var for scripted/CI use; `config.yaml` (already gitignored)
  is an acceptable home for it either way, consistent with how
  `AWS.Profile` is already stored there.
- **Settings view** gains a second, **read-only** "Queue Connection" row
  showing the configured proxy URL (or "not set"). Unlike "AWS Profile,"
  there's no discoverable list to pick from for a free-text URL, so no
  picker modal this pass — editing happens by hand in `config.yaml`. An
  interactive editor is a reasonable later increment, not this one.
- **`queues` view moves into `internal/app`** (mirroring `settings`),
  since it needs live backend access: a list of queues
  (name/pending/consumer counts), a browse/detail pane for a selected
  queue's messages, and Send/Purge/Move actions.
- **Non-blocking backend calls.** Every backend call triggered from the
  UI (initial list load, refresh, send, purge, move) runs in a goroutine
  and applies its result via `tv.QueueUpdateDraw(...)` — real network
  I/O must never block tview's render loop, per CLAUDE.md. This is the
  first real use of the bottom status bar (reserved since the
  shell-layout feature for "loading indicators, progress bars") — it
  shows a transient "Loading queues…"-style message while a call is in
  flight.

## Files touched

**`mq-proxy/` (new module):**
- `pom.xml`, `mvnw`, `mvnw.cmd`, `.mvn/wrapper/maven-wrapper.properties`,
  `.mvn/wrapper/maven-wrapper.jar`
- `src/main/java/dev/cloudtui/mqproxy/MqProxyApplication.java`
- `.../config/SecurityConfig.java`, `.../config/BrokerConfig.java`,
  `.../config/LocalBrokerConfig.java`
- `.../queues/QueuesController.java`, `.../queues/QueueService.java`,
  `.../queues/QueueNotFoundException.java`
- `.../error/ApiExceptionHandler.java`
- `src/main/resources/application.yaml`,
  `src/main/resources/application-local.yaml`
- `src/test/java/.../QueueServiceIntegrationTest.java` (against the
  local embedded broker), `.../QueuesControllerTest.java` (MockMvc),
  `.../SecurityConfigTest.java` (auth required/accepted)

**`api/` (new):** `api/openapi.yaml`

**`tui/` / root (modified/new):**
- `Taskfile.yml` — `build:mq-proxy`/`run:mq-proxy`/`test:mq-proxy`,
  `generate:mqproxy-client`, `generate`; dependency wiring as above
- `tui/internal/mqproxyclient/oapi-codegen-config.yaml` (new, committed)
- `tui/internal/mqproxyclient/generated/` (new, gitignored, build-time
  only)
- `tui/internal/queue/backend.go` (new)
- `tui/internal/queue/proxy/proxy.go`, `proxy_test.go` (new)
- `tui/internal/config/config.go`, `config_test.go` (modified) — `Queue`
  section
- `tui/internal/app/queues.go`, `queues_test.go` (new) — real queues view
- `tui/internal/app/settings.go`, `settings_test.go` (modified) — Queue
  Connection row
- `tui/internal/app/app.go`, `app_test.go` (modified) — construct the
  backend, wire the real queues view
- `tui/internal/ui/views/queues.go` (deleted),
  `tui/internal/ui/views/views_test.go` (modified)
- `tui/config.example.yaml` (modified) — document `queue:`

**Already done, ahead of this spec:** `CLAUDE.md`'s architecture section
(Basic Auth).

## Key decisions / trade-offs

- **Different codegen tool per language, deliberately.** `oapi-codegen`
  for Go, `openapi-generator-maven-plugin` for Java — each is the
  better-fit tool for its own ecosystem; there's no technical
  requirement that both sides use the same generator.
- **Generated Go client lives under a `generated/` directory** so it's
  covered by the existing `**/generated/` `.gitignore` rule instead of
  needing a new pattern.
- **tui's build/test/run depend on `generate:mqproxy-client`** — the Go
  compiler needs the generated package to physically exist first. This
  means `go test ./...` run directly (bypassing Task) won't work in a
  clean checkout until codegen has run at least once.
- **Two separate credential pairs** (HTTP Basic Auth vs. JMS broker
  credentials) — never conflated.
- **Queue listing/stats via ActiveMQ client APIs, not JMX or a
  broker-side plugin** — the only option that works against both the
  local embedded broker and a real (unconfigurable) Amazon MQ broker.
- **Queue Connection settings are read-only this pass** — no picker (a
  free-text URL isn't a discoverable list like AWS profiles are) and no
  interactive editor; `config.yaml` is hand-edited for now.
- **`queues` view moves into `internal/app`**, same reasoning as
  `settings`: it needs live backend access the stateless-placeholder
  views intentionally don't have.
- **All backend calls from the UI are goroutine + `QueueUpdateDraw`** —
  non-negotiable per CLAUDE.md's non-blocking UI requirement; this also
  finally gives the bottom status bar real content.

## Testing strategy

- **Java**: `QueueService` tested against the real local embedded broker
  (in-process, cheap, no mocking of ActiveMQ itself) for all five
  operations; `QueuesController` tested with MockMvc against a mocked
  `QueueService` for request/response mapping and error cases; a
  security test confirming missing/bad credentials get 401 and valid
  Basic Auth succeeds.
- **Go (`queue/proxy`)**: tests against an `httptest.Server` standing in
  for `mq-proxy`, covering all five `Backend` methods and error
  propagation (network failure, non-2xx responses).
- **Go (`config`)**: the new `Queue` section's defaults and save/load
  round-trip, matching the existing `AWS` section's test pattern.
- **tui (`app/queues_test.go`)**: a fake `queue.Backend` (mirroring the
  existing `fakeFilterableView` pattern) to verify list rendering,
  browse/send/purge/move action wiring, and that backend calls actually
  go through the goroutine + `QueueUpdateDraw` path rather than blocking
  (observable via a channel/synchronization point in the test, not a
  real network call).

## What gets confirmed at task-breakdown time, not guessed here

- Exact `oapi-codegen` config options/generator package name (the
  ecosystem has shifted between `deepmap/oapi-codegen` and
  `oapi-codegen/oapi-codegen`; the task step will pin the current
  maintained one and its exact flags).
- Exact `openapi-generator-maven-plugin` version/config for the
  Spring "interfaceOnly" + delegate pattern.
- Whether `ActiveMQConnection.getDestinationSource()`'s returned
  `Queue` set needs an explicit `advisorySupport`/`watchTopicAdvisories`
  connection-factory flag enabled to populate promptly — confirmed
  against the actual client library during the listing task, not
  assumed here.
