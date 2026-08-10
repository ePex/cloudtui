# Plan — FE 38

Verified end-to-end by actually running the generation task against this
repo (not just read the plugin docs) — two things the spec didn't
anticipate came up and are folded into this plan:

## 1. `mq-proxy/build.gradle.kts`

Add the plugin and config:

```kotlin
plugins {
    // ...
    id("org.springdoc.openapi-gradle-plugin") version "1.9.0"
}
```

```kotlin
openApi {
    apiDocsUrl.set("http://localhost:8080/v3/api-docs.yaml")
    outputDir.set(projectDir)
    outputFileName.set("openapi.yaml")
}
```

## 2. `SecurityConfig.kt` — permitAll gap (found live)

`/v3/api-docs.yaml` returned 401 against the existing
`requestMatchers("/swagger-ui/**", "/swagger-ui.html", "/v3/api-docs/**")`
— `/v3/api-docs/**` only matches *sub-paths* of `/v3/api-docs/`, not the
sibling path `/v3/api-docs.yaml` springdoc serves the YAML variant at.
Added `"/v3/api-docs.yaml"` as an explicit permitAll entry alongside it.
No behavior change for anything else; existing 30 tests still pass
unmodified (nothing exercised this exact path before).

## 3. `settings.gradle.kts` + daemon JVM pin — forked-run JVM mismatch (found live)

`generateOpenApiDocs` forks a real Spring Boot run
(`forkedSpringBootRun`) to fetch the live doc. That forked task launches
using **whatever JVM is running Gradle itself** — it has no
`customBootRun` hook to select a Java version/toolchain (confirmed
against the plugin source: `JavaExecFork` uses the ambient JVM, no
launcher override exists). On this machine, Gradle's daemon defaulted to
Java 17 while the project's classes are compiled for its Java 21
toolchain, so the forked process failed immediately:
`UnsupportedClassVersionError`.

Fix: pin Gradle's own daemon JVM to 21, via Gradle's `updateDaemonJvm`
mechanism (stable path since Gradle 8.8+, this project is on 8.13). That
requires a toolchain resolver, so:

```kotlin
// settings.gradle.kts
plugins {
    id("org.gradle.toolchains.foojay-resolver-convention") version "0.8.0"
}
```

then (one-time, output committed): `./gradlew updateDaemonJvm
--jvm-version=21`, which writes `mq-proxy/gradle/gradle-daemon-jvm.properties`
— a portable, checked-in pin (per-OS/arch download URLs, no
machine-specific paths) that makes **every** `./gradlew` invocation for
this project use JDK 21 to run Gradle itself, not just compile against
it. This is a small scope expansion beyond "just export the spec," but
there was no narrower fix available: it's the only supported way to make
`forkedSpringBootRun` (or any other JavaExec-forking plugin) match the
project's toolchain. `build:proxy`/`test:proxy`/`run:proxy` are
unaffected (still pass) — this only fixes a gap those tasks didn't
previously hit because Spring Boot's own `bootRun` task *is*
toolchain-aware and unaffected by this issue.

Foojay itself only participates in build-time toolchain resolution (auto-
locate/download a matching JDK); it has no runtime footprint in the
shipped `mq-proxy.jar`.

## 4. `Taskfile.yml`

New `openapi:proxy` task, `dir: mq-proxy`, same cross-platform gradlew
invocation pattern as `build:proxy:jar`:

```yaml
openapi:proxy:
  desc: Regenerate mq-proxy/openapi.yaml from the running app's live spec.
  dir: mq-proxy
  cmds:
    - '{{if eq OS "windows"}}cmd /c gradlew.bat{{else}}./gradlew{{end}} generateOpenApiDocs'
```

## 5. `mq-proxy/README.md`

Document `openapi.yaml` and `task openapi:proxy` under a new short
section, near the existing Swagger UI mention.

## Testing

- `./gradlew test` (existing 30 tests) — confirms the `SecurityConfig`
  change doesn't regress anything.
- Manual: ran `task openapi:proxy` (well, `./gradlew generateOpenApiDocs`
  directly, pre-Taskfile-wiring) end-to-end, confirmed `openapi.yaml` is
  generated, non-empty, and lists all 7 `QueueController` endpoints.
- No new Go-side code — `tui/` is untouched by this feature.
