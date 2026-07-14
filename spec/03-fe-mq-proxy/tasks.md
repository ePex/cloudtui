# Tasks — mq-proxy service

Plan: [plan.md](plan.md)

Condensed from the original 23-item task file (the largest in the repo);
every item below was already implemented before this condensation, so
all are checked. Notable implementation findings are kept as notes since
they're genuinely useful, not just status.

1. [x] **`mq-proxy/` Maven scaffold + Taskfile wiring** — `pom.xml`,
   Maven wrapper, `MqProxyApplication.java`, base `application.yaml`;
   `build:mq-proxy`/`run:mq-proxy`/`test:mq-proxy` Taskfile targets.
   Spring Boot 4.1.0 / Java 21. Windows needed `cmd.exe /c
   .\\mvnw.cmd ...` (Task's `mvdan/sh` shell treats a bare `./`/`.\`
   prefix as POSIX escaping; `.\\` survives as a literal `.\`).
2. [x] **`api/openapi.yaml`** — the full contract: all five operations,
   schemas, Basic Auth security scheme on every operation.
3. [x] **Codegen wiring, both languages** — `openapi-generator-maven-
   plugin` (Java, interface-only + delegate: with `interfaceOnly=true`
   the "delegate" methods live directly on the generated `QueuesApi`
   interface, no separate delegate bean); `oapi-codegen/oapi-codegen/v2`
   v2.7.2 (Go, pinned via `go run`, generated code under
   `tui/internal/mqproxyclient/generated/`, covered by the existing
   `**/generated/` gitignore rule).
4. [x] **Local embedded broker + JMS connection factory** —
   `LocalBrokerConfig` (`@Profile("local")`, in-memory `BrokerService`),
   `BrokerConfig` (profile-aware `ActiveMQConnectionFactory`). ActiveMQ
   classic 6.2.7.
5. [x] **HTTP Basic Auth** — `SecurityConfig` (stateless, CSRF
   disabled); base profile relies on Spring Boot's generated-and-logged
   random password, `local` profile sets a fixed dev user/password.
6. [x] **The five endpoints** (`QueueService` + `QueuesController`) —
   list (statistics-query mechanism, confirmed via a spike test that the
   plugin *is* enabled by default on Amazon MQ), browse (`QueueBrowser`,
   client-side limit), send, purge (transacted consume), move
   (transacted consume + send, single commit). `QueueService` connects
   lazily, on first call, so a default-profile boot never requires a
   reachable broker.
7. [x] **`tui/internal/queue` + `tui/internal/queue/proxy`** — the
   `Backend` interface/domain types and the real client implementation
   (Basic Auth via `WithRequestEditorFn`), tested against
   `httptest.Server`.
8. [x] **`tui/internal/config`: `Queue` section** + **Settings view's
   read-only "Queue Connection" row**.
9. [x] **tui Queues view** (list, browse/detail pane, send/purge/move
   actions) **+ `app.go` wiring** — built together since the pieces
   share too much state to sequence separately. Added the `activatable`
   interface so `switchTo` reloads the queue list each time the view
   opens. Required real test infrastructure: `runApp`/`waitFor` helpers
   using a headless `tcell.SimulationScreen`, since `QueueUpdateDraw`
   blocks forever without a running event loop.
10. [x] **Verify** — `task build`/`task test` (Java + Go) clean;
    `gofmt`/`go vet` clean; a real smoke test (started `mq-proxy` via
    `task run:mq-proxy`, hit it with `curl` — `200 []` with valid Basic
    Auth, `401` without, then cleanly stopped). The tui's queues view UI
    itself (list navigation, detail pane, action modals) was not
    exercised interactively from the sandboxed shell that built it —
    still recommended to try end-to-end via `task run:mq-proxy` +
    `task run:tui` in separate terminals.
