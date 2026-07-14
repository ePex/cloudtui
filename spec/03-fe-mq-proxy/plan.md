# Plan — mq-proxy service

Spec: [spec.md](spec.md)

This was the largest feature in the repo at the time. A few Java-library
specifics (exact ActiveMQ client calls, exact oapi-codegen config flags)
were deliberately left to be confirmed against the actual libraries
during implementation rather than guessed in advance.

## Approach

### `mq-proxy/` Spring Boot module

Maven, wrapper committed, Java 21, group/base package
`dev.cloudtui.mqproxy`. Dependencies: `spring-boot-starter-web`,
`spring-boot-starter-security` (HTTP Basic Auth),
`spring-boot-starter-jms`, `activemq-client` (JMS/OpenWire),
`activemq-broker` (embedded broker, `local` profile only),
`openapi-generator-maven-plugin`. `application.yaml` carries **two
separate credential pairs** — one for HTTP callers of the proxy, one for
the proxy's own JMS connection to the broker — conflating them would be
a real design smell. `application-local.yaml` starts an embedded
`BrokerService` as a Spring bean; local Basic Auth credentials default
to a fixed dev value, overridable by env var.

### `api/openapi.yaml` — the contract

`QueueSummary`/`Message`/`SendMessageRequest`/`MoveMessagesRequest`/
`ErrorResponse` schemas; Basic Auth security scheme on every operation.

### Codegen — different generators per language, deliberately

- **Java** (server): `openapi-generator-maven-plugin`, Spring
  interface-only + delegate pattern.
- **Go** (client): `oapi-codegen`, generated into
  `tui/internal/mqproxyclient/generated/` — deliberately under a
  directory literally named `generated` so the existing `**/generated/`
  `.gitignore` rule covers it without a new pattern.
- New Taskfile targets (`generate:mqproxy-client`, top-level
  `generate`); `build:tui`/`test:tui`/`run:tui` depend on it since the
  Go compiler needs the generated package to physically exist first.
  `build`/`test` (top-level) gain `build:mq-proxy`/`test:mq-proxy` as a
  second dependency.

### Queue listing without JMX or a broker-side plugin

Amazon MQ's JMX is unavailable and the managed broker's XML config isn't
ours to edit, so `<statisticsBrokerPlugin/>` isn't an option to declare —
but Amazon MQ enables it by default, and `ActiveMQConnection
.getDestinationSource()` (a standard client API) gives a live queue view
with no broker-side configuration required either way. Per-queue
pending/consumer counts come from ActiveMQ's built-in statistics-query
mechanism (an empty message to `ActiveMQ.Statistics.Destination.<name>`
with a temporary reply queue, reading the `MapMessage` reply's `size`/
`consumerCount` fields) — confirmed against a real broker via a
throwaway spike test during implementation, since the plugin
requirement wasn't certain from documentation alone.

### The five operations (`QueueService`)

List as above; **Browse** via `Session.createBrowser`, `limit` applied
client-side (JMS has no server-side pagination); **Send** via
`MessageProducer.send` with a `TextMessage`; **Purge**/**Move** via a
transacted session (`receive(timeout)` looped to empty then `commit()`
for purge; consume-then-send then a single `commit()` for move, making
it atomic). `QueueService` connects to the broker lazily, on first real
call — an eager connection would make the whole service fail to boot
under the default profile if no broker is reachable at startup.
`QueueNotFoundException` covers browse/purge/move (JMS auto-creates
queues on reference, so "not found" means "not currently in
`destinationSource.getQueues()`"); **send** deliberately skips that
check so posting to a brand-new queue still works.

### tui integration

`tui/internal/queue`: a `Backend` interface (`List`/`Browse`/`Send`/
`Purge`/`Move`, all `context`-aware) and domain types independent of the
generated client's types. `tui/internal/queue/proxy`: the real
implementation, using `oapi-codegen`'s `ClientWithResponses` +
`WithRequestEditorFn` for Basic Auth. `internal/config` gains a `Queue`
section (`ProxyURL`/`Username`/`Password`, the latter overridable via
`MQPROXY_CLIENT_PASSWORD`) — the "Queue Connection" setting deferred
from `02-fe-tui-shell-and-starting-features`. The `settings` view gains a second, read-only row for
it (no picker — a free-text URL isn't a discoverable list like AWS
profiles). `queues` moves into `internal/app` (mirroring `settings`):
list, browse/detail pane, send/purge/move actions, all non-blocking
(goroutine + `tv.QueueUpdateDraw`) — the first real content the bottom
status bar ever showed ("Loading queues…" etc.).

## Files touched

**`mq-proxy/` (new module):** `pom.xml`, Maven wrapper,
`MqProxyApplication.java`, `config/{SecurityConfig,BrokerConfig,
LocalBrokerConfig}.java`, `queues/{QueuesController,QueueService,
QueueNotFoundException}.java`, `error/ApiExceptionHandler.java`,
`application{,-local}.yaml`, plus tests for each.

**`api/` (new):** `api/openapi.yaml`.

**`tui/`/root:** `Taskfile.yml`; `tui/internal/mqproxyclient/
oapi-codegen-config.yaml`; `tui/internal/queue/{backend.go,
proxy/proxy.go}` (+ tests); `internal/config` (`Queue` section);
`internal/app/{queues.go,settings.go,app.go}` (+ tests);
`internal/ui/views/queues.go` deleted; `config.example.yaml`.

## Key decisions / trade-offs

- Different codegen tool per language — no technical requirement that
  both sides use the same generator, and each is the better-fit tool for
  its own ecosystem.
- Generated Go client lives under a `generated/` directory specifically
  so the existing gitignore pattern covers it.
- Two separate credential pairs (HTTP Basic Auth vs. JMS broker
  credentials) — never conflated.
- Queue listing/stats via ActiveMQ client APIs, not JMX or a
  broker-side plugin — the only option that works against both the
  local embedded broker and a real, unconfigurable Amazon MQ broker.
- Queue Connection settings are read-only this pass — hand-edited in
  `config.yaml`; an interactive editor is a reasonable later increment.
- `queues` moves into `internal/app`, same reasoning as `settings`.
- All backend calls from the UI are goroutine + `QueueUpdateDraw` — non-
  negotiable per the non-blocking-UI requirement.

## Testing strategy

- **Java**: `QueueService` against the real local embedded broker for
  all five operations; `QueuesController` with MockMvc against a mocked
  `QueueService`; a security test for missing/bad/valid Basic Auth.
- **Go (`queue/proxy`)**: against an `httptest.Server` standing in for
  `mq-proxy`, covering all five methods and error propagation.
- **Go (`config`)**: the `Queue` section's defaults and save/load
  round-trip.
- **tui (`app/queues_test.go`)**: a fake `queue.Backend` verifying list
  rendering, browse/send/purge/move wiring, and that backend calls go
  through the goroutine + `QueueUpdateDraw` path rather than blocking.
