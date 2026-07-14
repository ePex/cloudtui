# CLAUDE.md — mq-proxy module

Java-specific conventions for `mq-proxy/`. Repo-wide rules (workflow
gating, `spec/` conventions, cross-platform constraints) live in the
root `CLAUDE.md` and apply here too; this file only adds what's
specific to this module.

## Style and formatting

- Standard Java conventions, 4-space indentation (`.editorconfig`
  enforces this); no reformatting of code you're not otherwise
  touching.
- Constructor injection only — no field injection (`@Autowired` on
  fields), no Lombok. Keeps dependencies explicit and classes
  constructible directly in plain unit tests without a Spring context.
- Javadoc is for *why*, not *what*: reserve it for non-obvious design
  decisions (see `QueueService`'s class-level and method-level comments
  — e.g. why the connection is lazy, why statistics need the broker's
  plugin enabled, why `send`/`move`'s target don't require the queue to
  pre-exist) rather than restating the method signature.
- Custom exceptions (`QueueNotFoundException`, `QueueOperationException`)
  extend `RuntimeException` and are translated to HTTP responses
  centrally by `ApiExceptionHandler` — don't catch-and-translate
  ad hoc in individual controllers.

## Package layout

- `config/` — Spring `@Configuration` classes, including the `local`
  profile's embedded-broker setup (`LocalBrokerConfig`) and HTTP Basic
  Auth wiring (`SecurityConfig`).
- `queues/` — the actual JMS/OpenWire queue operations (`QueueService`)
  and the controller implementing the generated API interface
  (`QueuesController`), plus the domain exceptions above.
- `error/` — cross-cutting exception handling (`ApiExceptionHandler`).
- `generated/` (under `target/`, not `src/`) — produced by the
  `openapi-generator-maven-plugin` from `api/openapi.yaml` at build
  time; never committed, never hand-edited. Controllers implement the
  generated interface rather than being hand-written from scratch.

## Configuration and secrets

- Configuration via `application.yaml` + environment variables; no
  credentials in config files, ever — including in the `local` profile.
- Both the `local` profile and AWS deployment use the same HTTP Basic
  Auth mechanism (credentials from Secrets Manager in AWS, from local
  config for `local`) — don't special-case auth per environment.

## Testing

- JUnit 5 + AssertJ (`assertThat`). No Mockito-style JMS mocking for the
  queue-operation tests: `QueuesController*Test` classes run
  `@SpringBootTest` + `@AutoConfigureMockMvc` with `@ActiveProfiles
  ("local")` against the real embedded ActiveMQ broker, since JMS/
  ActiveMQ's async advisory-based destination discovery
  (`DestinationSource`) is exactly the kind of timing behavior a mock
  would hide — tests poll for eventual state rather than asserting
  once immediately.
- `spring-boot-starter-webmvc-test` (not bundled in
  `spring-boot-starter-test` as of Spring Boot 4) is required
  separately for `MockMvc` support — don't assume it comes for free.
- A thin `@Configuration` class or generated model with no branching
  logic falls under the "genuinely untestable" carve-out in root
  `CLAUDE.md`'s testing rule; say so explicitly rather than writing a
  test that asserts nothing meaningful.

## Dependencies

- Currently: Spring Boot starters (web, validation, security, test),
  `activemq-client`/`activemq-broker` (the broker jar is only used by
  the `local` profile but must stay on the classpath since
  `LocalBrokerConfig` references `BrokerService` at compile time),
  `openapi-generator-maven-plugin` for the generated API layer.
- Justify any new dependency in the relevant spec's `plan.md` before
  adding it to `pom.xml`.
