# Spec — FE 38: Committed OpenAPI spec export for mq-proxy

Date: 2026-08-10

## Background

FE 20 (spec/20-fe-mq-proxy) already wires up
`springdoc-openapi-starter-webmvc-ui`, which serves a *live, dynamically
generated* OpenAPI document at `/v3/api-docs` (and Swagger UI at
`/swagger-ui.html`) whenever the proxy is running. There is no static,
committed spec file — seeing it today requires starting the proxy and
hitting that endpoint.

## Problem

A static `openapi.yaml` checked into the repo is useful independent of a
running instance: client codegen, feeding it to other tooling, reviewing
API surface changes in a PR diff, or handing the file to something that
just wants a spec without standing up the service.

## Decisions (confirmed)

1. **Export, don't hand-write.** The static file is generated *from* the
   same springdoc/controller-annotation source of truth already in place
   — never maintained by hand as a second, driftable copy.
2. **`springdoc-openapi-gradle-plugin` (1.9.0)** does the export: it
   forks a real Spring Boot run of the app (`forkedSpringBootRun`),
   fetches the live doc, writes it to a file, then stops. Confirmed
   viable without a running broker: `JmsConfig`'s
   `ActiveMQConnectionFactory` bean is constructed lazily (no socket
   opened at startup), and every config property
   (`BrokerProperties`/`ProxyAuthProperties`/`server.port`) already has a
   default — the app boots cleanly with zero environment configured.
3. **YAML, not JSON** — more reviewable in a PR diff. The plugin
   generates YAML automatically when `apiDocsUrl` is pointed at
   springdoc's `/v3/api-docs.yaml` (rather than the default `.../api-docs`
   JSON endpoint) — no separate conversion step.
4. **Output committed at `mq-proxy/openapi.yaml`**, generated on demand
   via a new `task openapi:proxy`, not on every build — regeneration is a
   deliberate step a developer runs after an API-visible change, same as
   any other generated-and-committed artifact in this repo (never
   auto-run as part of `build:proxy`/CI in this slice).
5. **Auth**: `/v3/api-docs/**` is already `permitAll` in `SecurityConfig`
   (set up in FE 20), so the forked instance's default proxy-auth
   credentials don't need to be supplied to fetch it.

## Scope

- `mq-proxy/build.gradle.kts`: add plugin
  `id("org.springdoc.openapi-gradle-plugin") version "1.9.0"` and an
  `openApi { ... }` config block (`apiDocsUrl` → `.../v3/api-docs.yaml`,
  `outputDir` → `mq-proxy/`, `outputFileName` → `openapi.yaml`).
- `mq-proxy/openapi.yaml`: generated once as part of this feature,
  committed.
- `Taskfile.yml`: new `openapi:proxy` task (`dir: mq-proxy`, runs
  `./gradlew generateOpenApiDocs` cross-platform, matching the existing
  `.\gradlew.bat`-via-`cmd /c` pattern used by `build:proxy:jar`).
- `mq-proxy/README.md`: document the file and the `task openapi:proxy`
  regeneration step.
- Two fixes needed to make the export actually work, found by running it
  live rather than just reading the plugin docs — see `plan.md` for the
  detail:
  - `SecurityConfig.kt`: the existing `/v3/api-docs/**` permitAll rule
    doesn't cover the sibling `/v3/api-docs.yaml` path springdoc serves
    the YAML variant at (a real gap, not a regression — nothing
    previously requested that exact path).
  - `settings.gradle.kts` + a committed
    `gradle/gradle-daemon-jvm.properties`: pin Gradle's own daemon JVM to
    21 (via the Foojay toolchain resolver + `updateDaemonJvm`), because
    the plugin's forked Spring Boot run has no toolchain hook and
    otherwise inherits whatever JVM happens to be running Gradle.

## Out of scope

- CI enforcement that the committed file is up to date (a drift check
  comparing a fresh generation against the committed copy) — worth
  revisiting once FE 35's CI pipeline exists in more detail, not bundled
  here.
- Client SDK/codegen from the spec.
- Hand-authored examples/descriptions beyond whatever springdoc already
  derives from the Kotlin controller code (no new `@Operation`/`@Schema`
  annotation work in this slice).
