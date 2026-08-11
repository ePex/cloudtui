# Tasks — FE 38

1. [x] `mq-proxy/build.gradle.kts`: add `springdoc-openapi-gradle-plugin`
   + `openApi { }` config block.
2. [x] `SecurityConfig.kt`: permit `/v3/api-docs.yaml` alongside the
   existing `/v3/api-docs/**`.
3. [x] `settings.gradle.kts`: add Foojay toolchain resolver; run
   `./gradlew updateDaemonJvm --jvm-version=21`, commit the generated
   `gradle/gradle-daemon-jvm.properties`.
4. [x] Generate and commit `mq-proxy/openapi.yaml`
   (`./gradlew generateOpenApiDocs`) — verified non-empty, covers all 7
   `QueueController` endpoints.
5. [x] `Taskfile.yml`: add `openapi:proxy` task.
6. [x] `mq-proxy/README.md`: document `openapi.yaml` + `task openapi:proxy`.
7. [x] Re-run `./gradlew test` (regression check on the `SecurityConfig`
   change) and confirm `task openapi:proxy` works end-to-end through the
   Taskfile (not just the raw gradlew command already verified). Both
   confirmed: 30/30 tests pass, `task openapi:proxy` regenerated
   `openapi.yaml` successfully from the repo root.
