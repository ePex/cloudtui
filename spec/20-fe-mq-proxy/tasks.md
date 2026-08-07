# Tasks — FE 20: MQ Proxy Service

Spec: [spec.md](spec.md) | Plan: [plan.md](plan.md)

## Gradle project scaffold

1. [x] **`settings.gradle.kts` + `build.gradle.kts`** — create Gradle project
   with Kotlin JVM, Spring Boot, `spring-boot-starter-web`,
   `spring-boot-starter-activemq`, `spring-boot-starter-security`,
   `jackson-module-kotlin`, `spring-boot-starter-test`, `mockk`. Generate
   Gradle wrapper (`./gradlew wrapper`). Minimal `MqProxyApplication.kt` stub
   added so `bootJar` resolves main class. `./gradlew build` passes.
   `Taskfile.yml` gains `build:proxy`, `run:proxy`, `test:proxy` tasks.

## Configuration layer

2. [x] **`BrokerProperties.kt`** — `@ConfigurationProperties("broker")` data
   class with `url`, `username`, `password`.

3. [x] **`application.yml`** — default broker URL (`tcp://localhost:61616`),
   credentials, server port (`8080`), proxy auth credentials.

4. [x] **`JmsConfig.kt`** — `@Configuration` bean producing
   `ActiveMQConnectionFactory` from `BrokerProperties`. No pooling for now.

5. [x] **`SecurityConfig.kt`** — HTTP Basic auth over `/api/**`; credentials
   from `proxy.auth.*`; CSRF disabled. `ProxyAuthProperties.kt` added to bind
   `proxy.auth.username` / `proxy.auth.password`.

## Response model

6. [x] **Data classes** — `QueueSummary`, `MessageSummary`, `MessageDetail`
   in `api/model/`. No logic; just fields as documented in plan.

## BrokerService — read operations

7. [x] **`listQueues()`** — open session, call
   `ActiveMQSession.getDestinations()`, map to `QueueSummary` list with
   pending/consumer/enqueue/dequeue counts. Return names-only with counts `-1`
   if statistics unavailable. Stats fetched via ActiveMQ Statistics Plugin
   (send to `ActiveMQ.Statistics.Destination.<name>`, read reply properties).
   `NotFoundException.kt` also created here.

8. [x] **`browseMessages(queueName)`** — `QueueBrowser` enumeration, map each
   `TextMessage` to `MessageSummary` (ID, ISO-8601 timestamp, body, string
   properties).

9. [x] **`getMessage(queueName, messageId)`** — browse + filter by ID, return
   `MessageDetail` with full headers + properties. Throw `NotFoundException`
   if not found.

## BrokerService — write operations

10. [x] **`deleteMessage(queueName, messageId)`** — `AUTO_ACKNOWLEDGE` session,
    consumer with JMS selector `JMSMessageID = '<id>'`, `receive(2000)`. Throw
    `NotFoundException` on timeout.

11. [x] **`moveMessage(queueName, messageId, destination)`** — transacted
    session: consume with selector from src, send copy to dest, commit. Rollback
    + throw `NotFoundException` on timeout.

12. [x] **`moveAll(queueName, destination)`** — transacted session: loop
    `receiveNoWait()` from src + send to dest until null, commit. Return count.

13. [x] **`sendMessage(queueName, body)`** — create `TextMessage`, send via
    `MessageProducer`.

14. [x] **`purgeQueue(queueName)`** — `AUTO_ACKNOWLEDGE` consumer, loop
    `receiveNoWait()` until null. Return count.

## REST controller

15. [x] **`QueueController.kt`** — all 8 endpoints wired to `BrokerService`
    as documented in plan. `{id:.+}` path pattern for message ID. Returns
    appropriate HTTP status codes (200, 204, 201).

16. [x] **`GlobalExceptionHandler.kt`** — `@ControllerAdvice` mapping
    `NotFoundException` → 404, `JMSException` → 502, `Exception` → 500;
    all with `{ "error": "..." }` JSON body.

## Entry point

17. [x] **`MqProxyApplication.kt`** — `@SpringBootApplication` main function.
    Created as part of task 1 scaffold; `@ConfigurationPropertiesScan` added
    in task 2. Context load verified by `./gradlew build`.

## Tests

18. [x] **`BrokerServiceTest.kt`** — MockK unit tests for all 7 service
    methods: happy path + error/not-found cases (see plan for full list).
    15 tests, all passing.

19. [x] **`QueueControllerTest.kt`** — `@WebMvcTest` slice tests for all 8
    endpoints: 200/204 happy path, 404 on `NotFoundException`, 502 on
    `JMSException`, 401 on missing auth. `springmockk:4.0.2` added as test
    dependency for `@MockkBean`. POST/DELETE tests use `with(csrf())`.
    15 controller tests + 15 service tests = 30 total, all passing.

## Packaging and documentation

20. [x] **`Dockerfile`** — `eclipse-temurin:21-jre` base, copies
    `build/libs/mq-proxy.jar`, exposes 8080. Image builds successfully with
    podman. `Taskfile.yml` `build:proxy` task runs `bootJar` + `podman build`.

21. [x] **`mq-proxy/README.md`** — build, run (JAR + Docker), configuration
    reference (env vars and YAML keys), example `curl` commands for each
    endpoint.

22. [x] **`Taskfile.yml` tasks** — `build:proxy`, `run:proxy`, `test:proxy`
    added (task 1); `test:proxy` wired into top-level `test` task (this task).
    All delegate to `./gradlew` inside `mq-proxy/`.

23. [x] **Swagger UI** — added `springdoc-openapi-starter-webmvc-ui:2.8.9`;
    `/swagger-ui/**`, `/v3/api-docs/**` opened in `SecurityConfig` (no auth
    required); `OpenApiConfig` bean registers HTTP Basic security scheme and
    applies it globally so the Authorize button appears in the UI; README
    updated. UI available at `http://localhost:8080/swagger-ui.html`.
