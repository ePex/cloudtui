# Plan — FE 20: MQ Proxy Service

## Overview

A standalone Kotlin + Spring Boot application in `mq-proxy/` that exposes a
REST API covering all broker operations the TUI needs, backed by JMS via
`activemq-client` (OpenWire). The TUI's existing Jolokia client is untouched;
a new proxy client will be added in a follow-up feature.

---

## Project structure

```
mq-proxy/
├── build.gradle.kts
├── settings.gradle.kts
├── Dockerfile
├── README.md
└── src/
    ├── main/
    │   ├── kotlin/com/github/epex/mqproxy/
    │   │   ├── MqProxyApplication.kt      # @SpringBootApplication entry point
    │   │   ├── config/
    │   │   │   ├── BrokerProperties.kt    # @ConfigurationProperties for broker
    │   │   │   └── JmsConfig.kt           # ActiveMQConnectionFactory bean
    │   │   ├── api/
    │   │   │   ├── QueueController.kt     # REST endpoints
    │   │   │   └── model/
    │   │   │       ├── QueueSummary.kt    # response data classes
    │   │   │       ├── MessageSummary.kt
    │   │   │       └── MessageDetail.kt
    │   │   └── service/
    │   │       └── BrokerService.kt       # all JMS operations
    │   └── resources/
    │       └── application.yml
    └── test/
        └── kotlin/com/github/epex/mqproxy/
            ├── service/
            │   └── BrokerServiceTest.kt   # unit tests with mocked JMS
            └── api/
                └── QueueControllerTest.kt # @WebMvcTest slice tests
```

---

## Gradle build (`build.gradle.kts`)

- `org.springframework.boot` plugin + `io.spring.dependency-management`
- `kotlin("jvm")` + `kotlin("plugin.spring")` (opens classes for Spring proxies)
- Dependencies:
  - `spring-boot-starter-web` (REST, embedded Tomcat)
  - `spring-boot-starter-activemq` (JMS template + `activemq-client` 5.x)
  - `jackson-module-kotlin` (JSON serialisation of data classes)
  - `spring-boot-starter-security` (HTTP Basic auth)
  - `springdoc-openapi-starter-webmvc-ui` (Swagger UI at `/swagger-ui.html`)
  - `spring-boot-starter-test` + `mockk` + `springmockk` (unit + slice tests)

---

## Configuration (`application.yml`)

```yaml
broker:
  url: tcp://localhost:61616
  username: admin
  password: admin

spring:
  activemq:
    broker-url: ${broker.url}
    user: ${broker.username}
    password: ${broker.password}

server:
  port: 8080
```

Environment variable overrides: `BROKER_URL`, `BROKER_USERNAME`,
`BROKER_PASSWORD`, `SERVER_PORT`.

HTTP Basic credentials are configured separately:

```yaml
proxy:
  auth:
    username: cloudtui
    password: changeme
```

---

## JmsConfig

```kotlin
@Configuration
class JmsConfig(private val props: BrokerProperties) {
    @Bean
    fun connectionFactory(): ActiveMQConnectionFactory =
        ActiveMQConnectionFactory(props.username, props.password, props.url)
            .also { it.isTrustAllPackages = false }
}
```

`JmsTemplate` is auto-configured by Spring Boot from `spring.activemq.*`.
Direct `Connection`/`Session` management is used in `BrokerService` for
operations that require transactions or `QueueBrowser` (JmsTemplate doesn't
expose these cleanly).

---

## BrokerService

All methods open a `Connection → Session` (or reuse a pooled one), perform the
operation, and close cleanly. No singleton session — keeps connection handling
simple and avoids stale-session issues.

### `listQueues(): List<QueueSummary>`

Use `ActiveMQSession.getDestinations()` which returns
`ActiveMQTempQueue`/`ActiveMQQueue` with statistics available via
`DestinationStatistics` (pending, consumers, enqueued, dequeued counts).

Fallback if statistics are unavailable: return names only with counts as -1.

### `browseMessages(queueName: String): List<MessageSummary>`

Open `QueueBrowser` on the named queue. Enumerate
`browser.enumeration` casting each element to `Message`. Extract:
- `JMSMessageID` (string)
- `JMSTimestamp` (Long → ISO-8601)
- body (cast to `TextMessage`, read `.text`; non-text → null)
- string properties via `message.propertyNames` enumeration

### `getMessage(queueName: String, messageId: String): MessageDetail`

Browse + filter by `JMSMessageID`. Returns full headers + properties.

### `deleteMessage(queueName: String, messageId: String)`

```
Session(AUTO_ACKNOWLEDGE) →
  MessageConsumer with selector "JMSMessageID = '<id>'" →
  consumer.receive(timeout) → ACK implicit
```

Timeout: 2 s. If no message received → 404.

### `moveMessage(queueName: String, messageId: String, destination: String)`

```
Session(SESSION_TRANSACTED) →
  consumer = createConsumer(src, selector) →
  msg = consumer.receive(timeout) → if null → rollback + 404
  producer = createProducer(dest) →
  producer.send(msg copy with new destination) →
  session.commit()
```

### `moveAll(queueName: String, destination: String): Int`

Same transacted session pattern, loop until `consumer.receiveNoWait()` returns
null. Returns count of moved messages.

### `sendMessage(queueName: String, body: String)`

```
Session → MessageProducer → producer.send(session.createTextMessage(body))
```

### `purgeQueue(queueName: String): Int`

```
Session(AUTO_ACKNOWLEDGE) →
  consumer = createConsumer(queue) →
  loop: msg = consumer.receiveNoWait() until null → count++
```

Returns count of purged messages.

---

## QueueController

`@RestController @RequestMapping("/api")`

| Method | Path | Delegates to |
|--------|------|--------------|
| GET | `/queues` | `brokerService.listQueues()` |
| GET | `/queues/{name}/messages` | `brokerService.browseMessages(name)` |
| GET | `/queues/{name}/messages/{id}` | `brokerService.getMessage(name, id)` |
| DELETE | `/queues/{name}/messages/{id}` | `brokerService.deleteMessage(name, id)` |
| POST | `/queues/{name}/messages/{id}/move` | `brokerService.moveMessage(name, id, to)` |
| POST | `/queues/{name}/move` | `brokerService.moveAll(name, to)` |
| POST | `/queues/{name}/messages` | `brokerService.sendMessage(name, body)` |
| DELETE | `/queues/{name}/messages` | `brokerService.purgeQueue(name)` |

Error handling: `@ExceptionHandler` in a `@ControllerAdvice` maps:
- `NotFoundException` (internal) → 404
- `JMSException` → 502 with message
- generic `Exception` → 500

URL-encoded message IDs: Spring MVC decodes path variables automatically, but
JMS message IDs contain colons (`ID:...`) — use `{id:.+}` pattern to capture
the full ID including dots and colons.

---

## Security

`SecurityConfig` configures HTTP Basic auth over all `/api/**` endpoints.
Credentials from `proxy.auth.username` / `proxy.auth.password`.
CSRF disabled (stateless REST API).

Swagger UI paths (`/swagger-ui/**`, `/swagger-ui.html`, `/v3/api-docs/**`)
are permitted without authentication so the UI is accessible without
pre-configuring credentials in the browser. Credentials are entered in the
Swagger UI Authorize dialog per-request.

---

## Response model (data classes)

```kotlin
data class QueueSummary(
    val name: String,
    val pendingCount: Long,
    val consumerCount: Long,
    val enqueueCount: Long,
    val dequeueCount: Long,
)

data class MessageSummary(
    val id: String,
    val timestamp: String,      // ISO-8601
    val body: String?,
    val properties: Map<String, String>,
)

data class MessageDetail(
    val id: String,
    val timestamp: String,
    val body: String?,
    val deliveryMode: Int,
    val priority: Int,
    val correlationId: String?,
    val replyTo: String?,
    val destination: String,
    val redelivered: Boolean,
    val properties: Map<String, String>,
)
```

---

## Testing

### `BrokerServiceTest`

Unit tests using MockK to mock `Connection`, `Session`, `QueueBrowser`,
`MessageConsumer`, `MessageProducer`. Tests cover:
- `listQueues()` returns mapped summaries
- `browseMessages()` maps TextMessage fields correctly
- `deleteMessage()` receives with selector, no message → throws NotFoundException
- `moveMessage()` commits transaction, no message → rolls back + throws
- `sendMessage()` creates and sends TextMessage
- `purgeQueue()` drains until receiveNoWait returns null, returns count

### `QueueControllerTest`

`@WebMvcTest(QueueController::class)` with `BrokerService` mocked via MockK.
Tests cover:
- Happy path for each endpoint (200/204 + JSON body)
- 404 when service throws NotFoundException
- 502 when service throws JMSException
- Unauthenticated request → 401

---

## Dockerfile

```dockerfile
FROM eclipse-temurin:21-jre
WORKDIR /app
COPY build/libs/mq-proxy.jar app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "app.jar"]
```

Build: `./gradlew bootJar && docker build -t mq-proxy .`

---

## Files touched

| File | Change |
|------|--------|
| `mq-proxy/settings.gradle.kts` | new |
| `mq-proxy/build.gradle.kts` | new |
| `mq-proxy/src/main/resources/application.yml` | new |
| `mq-proxy/src/main/kotlin/.../MqProxyApplication.kt` | new |
| `mq-proxy/src/main/kotlin/.../config/BrokerProperties.kt` | new |
| `mq-proxy/src/main/kotlin/.../config/JmsConfig.kt` | new |
| `mq-proxy/src/main/kotlin/.../config/SecurityConfig.kt` | new |
| `mq-proxy/src/main/kotlin/.../api/model/*.kt` | new (3 data class files) |
| `mq-proxy/src/main/kotlin/.../api/QueueController.kt` | new |
| `mq-proxy/src/main/kotlin/.../service/BrokerService.kt` | new |
| `mq-proxy/src/test/kotlin/.../service/BrokerServiceTest.kt` | new |
| `mq-proxy/src/test/kotlin/.../api/QueueControllerTest.kt` | new |
| `mq-proxy/Dockerfile` | new |
| `mq-proxy/README.md` | new |

No changes to existing `tui/` code in this feature.

---

## No new repo-level dependencies

Gradle wrapper is generated via `gradle wrapper` inside `mq-proxy/`; the
wrapper files (`gradlew`, `gradlew.bat`, `gradle/`) are committed. The root
`Taskfile.yml` gains a `proxy:build` and `proxy:run` task pointing into
`mq-proxy/`.
