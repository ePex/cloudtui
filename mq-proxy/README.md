# mq-proxy

A Kotlin + Spring Boot proxy service that exposes a REST API over ActiveMQ
broker operations. Designed to replace the Jolokia HTTP/JMX API for brokers
that do not expose Jolokia (e.g. AWS Amazon MQ).

## Prerequisites

- Java 21+
- An ActiveMQ broker (local or AWS AMQ) accessible via OpenWire (default port
  `61616`)

## Build

```sh
# From the mq-proxy/ directory:
./gradlew build          # compile, test, assemble JAR
./gradlew bootJar        # assemble JAR only (skips tests)
```

The JAR is written to `build/libs/mq-proxy.jar`.

## Run

### From source

```sh
./gradlew bootRun
```

### From JAR

```sh
java -jar build/libs/mq-proxy.jar
```

### Container (podman)

```sh
./gradlew bootJar
podman build -t mq-proxy .
podman run --rm -p 8080:8080 \
  -e BROKER_URL=tcp://my-broker:61616 \
  -e BROKER_USERNAME=admin \
  -e BROKER_PASSWORD=secret \
  mq-proxy
```

Or use the Taskfile from the repo root:

```sh
task build:proxy   # bootJar + podman build
task run:proxy     # ./gradlew bootRun
```

## Configuration

All settings can be provided via `application.yml` or environment variables.

| Environment variable   | YAML key                  | Default                    | Description                        |
|------------------------|---------------------------|----------------------------|------------------------------------|
| `BROKER_URL`           | `broker.url`              | `tcp://localhost:61616`    | OpenWire broker URL                |
| `BROKER_USERNAME`      | `broker.username`         | `admin`                    | Broker credentials — username      |
| `BROKER_PASSWORD`      | `broker.password`         | `admin`                    | Broker credentials — password      |
| `SERVER_PORT`          | `server.port`             | `8080`                     | HTTP port the proxy listens on     |
| `PROXY_AUTH_USERNAME`  | `proxy.auth.username`     | `cloudtui`                 | HTTP Basic auth username for proxy |
| `PROXY_AUTH_PASSWORD`  | `proxy.auth.password`     | `changeme`                 | HTTP Basic auth password for proxy |
| `CORS_ALLOWED_ORIGINS` | `proxy.cors.allowed-origins` | *(none)*                | Comma-separated `http(s)://` origins allowed to call the API cross-origin (e.g. wherever [`mq-proxy-web`](../mq-proxy-web) is hosted). The `null` origin — what a browser sends for a page opened directly via `file://` — is always allowed and does not need to be listed here. |

Example `application.yml` override:

```yaml
broker:
  url: ssl://my-amq.amazonaws.com:61617
  username: admin
  password: secret

proxy:
  auth:
    username: cloudtui
    password: strongpassword

server:
  port: 9090
```

> **Note on queue statistics:** pending/consumer/enqueue/dequeue counts are
> fetched via the [ActiveMQ Statistics Plugin][stats-plugin]. If the plugin is
> not enabled on your broker, counts are returned as `-1`.

[stats-plugin]: https://activemq.apache.org/statisticsplugin

## API

All endpoints require HTTP Basic authentication using the proxy credentials
(`proxy.auth.*`), **not** the broker credentials.

### Swagger UI

When the proxy is running, an interactive API browser is available at:

```
http://localhost:8080/swagger-ui.html
```

No login required for Swagger UI itself — credentials are entered per-request
in the Authorize dialog.

### OpenAPI spec file

[`openapi.yaml`](openapi.yaml) is a committed, static export of the same
spec Swagger UI serves live — useful for client codegen or reviewing API
changes in a diff without starting the proxy. It's generated, not
hand-written; after changing a controller, regenerate it with:

```sh
task openapi:proxy
```

(or `./gradlew generateOpenApiDocs` from `mq-proxy/`) and commit the
result.

### List queues

```sh
curl -u cloudtui:changeme http://localhost:8080/api/queues
```

Response:
```json
[
  { "name": "orders", "pendingCount": 3, "consumerCount": 1, "enqueueCount": 10, "dequeueCount": 7 }
]
```

### Browse messages

```sh
curl -u cloudtui:changeme http://localhost:8080/api/queues/orders/messages
```

### Get message detail

```sh
curl -u cloudtui:changeme \
  "http://localhost:8080/api/queues/orders/messages/ID:host-1234-1"
```

### Send message

```sh
curl -u cloudtui:changeme -X POST \
  -H "Content-Type: text/plain" \
  -d "hello world" \
  http://localhost:8080/api/queues/orders/messages
```

### Delete message

```sh
curl -u cloudtui:changeme -X DELETE \
  "http://localhost:8080/api/queues/orders/messages/ID:host-1234-1"
```

### Move message

```sh
curl -u cloudtui:changeme -X POST \
  "http://localhost:8080/api/queues/orders/messages/ID:host-1234-1/move?to=dlq"
```

### Move all messages

```sh
curl -u cloudtui:changeme -X POST \
  "http://localhost:8080/api/queues/orders/move?to=archive"
```

Response:
```json
{ "moved": 3 }
```

### Purge queue

```sh
curl -u cloudtui:changeme -X DELETE \
  http://localhost:8080/api/queues/orders/messages
```

Response:
```json
{ "purged": 3 }
```

## Error responses

All errors return JSON:

```json
{ "error": "Message 'ID:...' not found in queue 'orders'" }
```

| HTTP status | Cause                                    |
|-------------|------------------------------------------|
| 401         | Missing or invalid proxy credentials     |
| 404         | Message ID not found in the named queue  |
| 502         | Broker connection or JMS protocol error  |
| 500         | Unexpected internal error                |

## Tests

```sh
./gradlew test
```

30 tests (15 service unit tests + 15 controller slice tests), no broker
connection required.
