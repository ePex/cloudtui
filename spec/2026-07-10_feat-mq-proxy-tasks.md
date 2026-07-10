# Tasks — mq-proxy service

Plan: [2026-07-10_feat-mq-proxy-plan.md](2026-07-10_feat-mq-proxy-plan.md)

Each task below needs explicit manual approval before it is implemented.
Given the size of this feature, tasks are ordered so each subsequent
step has everything it depends on already in place.

1. **`mq-proxy/` Maven scaffold** — `pom.xml`, Maven wrapper
   (`mvnw`/`mvnw.cmd`/`.mvn/wrapper/`), `MqProxyApplication.java`, base
   `application.yaml`. Boots to an empty Spring context; no endpoints,
   no broker, no security yet.
   Status: done. Spring Boot 4.1.0 / Java 21, group `dev.cloudtui`.
   `./mvnw test` passes (context-load test).

2. **Taskfile: mq-proxy targets** — `build:mq-proxy`, `run:mq-proxy`,
   `test:mq-proxy`; top-level `build`/`test` gain them as a second
   dependency (the gap flagged in the original Taskfile spec).
   Status: done. Windows needed `cmd.exe /c .\\mvnw.cmd ...` (mvdan/sh,
   Task's shell interpreter, treats a bare `./`/`.\` prefix as POSIX
   escaping — `.\\` survives that as a literal `.\`); the Linux/darwin
   variants use `./mvnw` directly. `-q` on the Windows `mvnw.cmd` path
   silently no-ops in this environment (exits 0, builds nothing) for
   reasons not fully root-caused — dropped for the Windows commands
   only. Verified `task build`, `task test`, `task build:mq-proxy`,
   `task test:mq-proxy` all work end-to-end.

3. **`api/openapi.yaml`** — the full contract: all five operations,
   `QueueSummary`/`Message`/`SendMessageRequest`/`MoveMessagesRequest`/
   `ErrorResponse` schemas, Basic Auth security scheme on every
   operation.
   Status: done. YAML validated (parses, 4 paths, 5 schemas); structural
   OpenAPI validation happens implicitly in tasks 4/5 when the codegen
   tools parse it.

4. **Java codegen wiring** — `openapi-generator-maven-plugin` in
   `mq-proxy/pom.xml` (Spring interface-only + delegate); a minimal
   stub `QueuesController` implementing the generated delegate (each
   method unimplemented/501) just to prove the generated code compiles
   and wires into the Spring context.
   Status: done. Notes for later tasks: (1) with `interfaceOnly=true`
   the plugin doesn't emit a separate `XxxApiDelegate` — the
   "delegate" methods live directly on the `QueuesApi` interface
   itself (`@RequestMapping`-annotated `_xxx` wrappers calling
   plain overridable `xxx(...)` methods), so `QueuesController`
   implements `QueuesApi` directly, no delegate bean needed. (2) Needed
   two extra runtime deps the generated code requires but the plugin
   doesn't pull in: `io.swagger.core.v3:swagger-annotations-jakarta`
   and `org.openapitools:jackson-databind-nullable`. (3) Spring Boot 4
   split MockMvc test support out of `spring-boot-starter-test` into
   `spring-boot-starter-webmvc-test`; `AutoConfigureMockMvc` moved to
   `org.springframework.boot.webmvc.test.autoconfigure`; verified with
   a wiring test (`GET /queues` → 501 from the stub) using classic
   `MockMvc`/`MockMvcRequestBuilders`, not the newer `MockMvcTester`
   (no parameter-resolver support for it in this setup).

5. **Go codegen wiring** — `tui/internal/mqproxyclient/oapi-codegen-config.yaml`;
   Taskfile `generate:mqproxy-client` target; `build:tui`/`test:tui`/
   `run:tui` gain it as a dependency. Verify the generated package
   (under `tui/internal/mqproxyclient/generated/`) compiles.
   Status: done. Tool: `oapi-codegen/oapi-codegen/v2` v2.7.2, invoked via
   pinned `go run` (not a go.mod dependency itself). Generated code
   imports `github.com/oapi-codegen/runtime`, added as a real tui
   dependency via `go get`. Confirmed `tui/internal/mqproxyclient/generated/`
   is covered by the existing `**/generated/` `.gitignore` rule.
   Verified end-to-end: deleted the generated dir, ran `task test:tui`,
   it regenerated and passed.

6. **Local embedded broker + JMS connection factory** —
   `LocalBrokerConfig` (`@Profile("local")`, in-memory `BrokerService`),
   `BrokerConfig` (profile-aware `ActiveMQConnectionFactory`),
   `application-local.yaml`. A test confirming the embedded broker
   starts and a JMS connection can be opened against it.
   Status: done. ActiveMQ classic 6.2.7 (`activemq-client` +
   `activemq-broker`). `mqproxy.broker.url/username/password` in
   `application.yaml`, defaulting to the local embedded broker's own
   connector — `application-local.yaml` needs no override.
   `LocalBrokerConfigTest` (`@ActiveProfiles("local")`) opens a real JMS
   connection/session against the embedded broker — passes.

7. **HTTP Basic Auth** — `SecurityConfig` (stateless, CSRF disabled,
   `httpBasic()`); `SecurityConfigTest` confirming missing/bad
   credentials get 401 and valid credentials succeed.
   Status: done. Base profile relies on Spring Boot's own
   generated-and-logged random password (safe default — forces explicit
   config before real deployment); `local` profile sets a fixed dev
   user/password via `spring.security.user.*`. Hit two cross-test
   issues: (1) `QueuesControllerWiringTest` (task 4, default profile)
   started getting 401 once security was active — fixed with
   `@AutoConfigureMockMvc(addFilters = false)`, since that test is about
   routing, not auth. (2) `SecurityConfigTest` and `LocalBrokerConfigTest`
   both activate `local` and therefore both start the embedded broker on
   the same fixed port — as separate Spring test contexts they clashed
   on the bind; fixed by giving both an identical
   `@SpringBootTest @AutoConfigureMockMvc @ActiveProfiles("local")`
   signature so Spring's test-context caching reuses one context (and
   one broker) for both — documented on `LocalBrokerConfigTest` as the
   pattern future local-profile tests should follow.

8. **List endpoint** — `QueueService.list()` (via
   `ActiveMQConnection.getDestinationSource()` + the statistics-query
   mechanism), `QueuesController` wiring, `ApiExceptionHandler`/
   `QueueNotFoundException` (error-handling pattern established here,
   reused by later endpoints); tests against the local embedded broker.
   Status: done, with two corrections to the plan found during
   implementation: (1) the per-queue statistics query **does** require
   `StatisticsBrokerPlugin` enabled broker-side after all — confirmed by
   a throwaway spike test that got no reply until the plugin was added
   to `LocalBrokerConfig`. Amazon MQ enables this by default on managed
   brokers (matching CLAUDE.md's original "statistics plugin" wording,
   which I'd second-guessed during planning); we just have to enable it
   ourselves for the broker we control. Reply `MapMessage` fields
   confirmed via the spike: `size` (pending count) and `consumerCount`.
   (2) `QueueService` connects to the broker **lazily** (on first real
   call), not in its constructor — an eager connection would have made
   `mq-proxy` fail to even boot under the default profile (no broker
   listening), breaking unrelated tests/deployments if the broker is
   briefly unreachable at startup. `QueueNotFoundException` deferred to
   task 9 (browse), since `list()` has no queue-name parameter that
   could 404 — introducing it now would've been unused code.

9. **Browse endpoint** — `QueueService.browse()` (`QueueBrowser`,
   client-side `limit`), controller wiring, tests.
   Status: done. `QueueNotFoundException` introduced here (list() had no
   use for it). Design call: JMS/ActiveMQ auto-creates queues on
   reference, so "not found" isn't a natural JMS error — implemented it
   as "not currently in `destinationSource.getQueues()`" (the same set
   `/queues` itself reports), checked before browse/purge/move. `send`
   (task 10) will deliberately skip this check so posting to a
   brand-new queue name still works, matching normal messaging
   semantics — the OpenAPI contract's 404 response for send stays
   declared (harmless) but won't actually trigger.

10. **Send endpoint** — `QueueService.send()`, controller wiring, tests.
    Status: done. Confirmed (as designed in task 9): sending to a
    brand-new queue name succeeds and auto-creates it — no
    `requireQueueExists` check here.

11. **Purge endpoint** — `QueueService.purge()` (transacted consume),
    controller wiring, tests.
    Status: done. `QueuesControllerWiringTest` (the "still-a-stub" wiring
    proof) retargeted to `moveMessages`, the only operation left
    unimplemented after this task.

12. **Move endpoint** — `QueueService.move()` (transacted consume +
    send, single commit), controller wiring, tests.
    Status: done. All five operations now have real logic, so
    `QueuesControllerWiringTest` (whose whole point was proving an
    endpoint still fell through to the generated 501 stub) no longer had
    a subject — deleted; wiring is now proven by every endpoint-specific
    test instead.

13. **`tui/internal/queue/backend.go`** — `Backend` interface
    (`List`/`Browse`/`Send`/`Purge`/`Move`, context-aware) and domain
    types (`Summary`, `Message`), independent of the generated client's
    types.
    Status: done. No test file — bare interface/types, no logic (same
    carve-out already used for `ui.Filterable`).

14. **`tui/internal/queue/proxy`** — the real `Backend` implementation
    wrapping the generated Go client; tests against an `httptest.Server`
    covering all five methods and error propagation.
    Status: done. Uses `oapi-codegen`'s `ClientWithResponses` +
    `WithRequestEditorFn` for Basic Auth. Kept error handling simple —
    no sentinel "not found" error type; callers get a descriptive
    `error` and the message text happens to say "queue not found" for
    404s. Can be revisited if a later UI task needs to special-case it.

15. **`tui/internal/config`: `Queue` section** — `ProxyURL`/`Username`/
    `Password` fields (password overridable via env var), `Default()`,
    save/load round-trip tests, `config.example.yaml` update.
    Status: done. Defaults: `proxyUrl: http://localhost:8081` (matches
    mq-proxy's own default port), `username: admin` (matches the local
    profile's dev default), `password: ""` (never defaulted, even to a
    known-dev-only value — set via config.yaml or
    `MQPROXY_CLIENT_PASSWORD`, which wins over the file and applies even
    if config.yaml is absent entirely).

16. **Settings view: Queue Connection row** — a second, read-only row
    showing the configured proxy URL or "not set" (no picker, no
    editor this pass); test update.
    Status: done.

17. **Queues view — list** — replaces the placeholder's list rendering:
    calls `Backend.List()` in a goroutine, applies results via
    `QueueUpdateDraw`, shows a loading indicator in the status bar while
    in flight; tests using a fake `Backend`.
    Status: done, built together with tasks 18–21 (see their combined
    notes) since the list/detail/actions share too much state to
    sensibly separate. Added a new `activatable` interface: `switchTo`
    now calls `activate()` on the newly active view if it implements
    one, so the queue list reloads each time the view is opened instead
    of only once at startup. **Testing this required real
    infrastructure**: `QueueUpdateDraw` blocks forever unless
    `tv`'s event loop is actually running, so plain `New()`-only tests
    (no `Run()`) would deadlock. Added `runApp`/`waitFor` test helpers
    (`queues_test.go`) using a headless `tcell.SimulationScreen`, and a
    mutex-guarded `fakeBackend`. This also **broke 5 pre-existing tests**
    that used `"queues"` as an arbitrary stand-in view name (its
    `Primitive()` used to be a flat placeholder; now it's a `Pages`
    itself, changing focus-delegation behavior, and `switchTo` now
    triggers a real network call via `activate()`) — retargeted them to
    `"params"`, which is unaffected either way.
    Status: done.

18. **Queues view — browse/detail pane** — selecting a queue shows its
    messages (same non-blocking pattern); tests.
    Status: done (see task 17's notes).

19. **Queues view — send action** — a small input form, calls
    `Backend.Send()`; tests.
    Status: done (see task 17's notes). Bound to `a` (not `s`, already
    claimed globally for Settings) via `SetInputCapture` on the messages
    list, not a global hotkey.

20. **Queues view — purge action** — confirmation step, calls
    `Backend.Purge()`; tests.
    Status: done (see task 17's notes). Bound to `d`; reloads both the
    detail pane and the queue list afterward (pending count changes).

21. **Queues view — move action** — prompts for a target queue, calls
    `Backend.Move()`; tests.
    Status: done (see task 17's notes). Bound to `v`; also reloads both
    panes afterward.

22. **`internal/app/app.go` wiring** — construct the `queue.Backend`
    from config, replace the `queues` placeholder with the real view
    (moving it into `internal/app`, same pattern as `settings`); delete
    `internal/ui/views/queues.go`; update `views_test.go`/`app_test.go`.
    Status: done — completed as part of building `queues.go` (task 17),
    since the pieces were too interdependent to sequence separately.
    `proxy.New(cfg.Queue.ProxyURL, cfg.Queue.Username, cfg.Queue.Password)`
    constructs the backend in `New()`; errors are logged to stderr and
    non-fatal (in practice `proxy.New` can't fail for a plain string
    URL, confirmed by reading its source, so this is a formality).

23. **Verify** — `task build`, `task test` (both Java and Go through
    Task), `gofmt`/`go vet` clean; best-effort run check (same
    tty/tmux caveat as prior features) plus a manual note on what still
    needs a human with a real terminal (and, ideally, a real or
    local-broker-backed queue) to fully exercise interactively.
    Status: done. `task build` and `task test` both succeed cleanly
    (13 Java tests, all Go packages); `gofmt`/`go vet` clean;
    `internal/app` tests stable across 5 repeated runs (no goroutine-
    timing flakiness observed, though `-race` isn't available in this
    environment — no C compiler for CGO). **Beyond the tui's usual 5s
    headless run check**, also did a real smoke test this time: started
    `mq-proxy` via `task run:mq-proxy` (local profile) and hit the live
    server with `curl` — `GET /queues` returned `200 []` with valid Basic
    Auth and `401` with none — then cleanly stopped it (no stray
    processes, port 8081 freed). Still not exercised interactively from
    this sandboxed shell: the tui's queues view UI itself (list
    navigation, detail pane, send/purge/move modals) — recommend running
    `task run:mq-proxy` in one terminal and `task run:tui` in another to
    try it end-to-end.
