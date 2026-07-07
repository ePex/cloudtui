# cloudtui

A cross-platform terminal UI (k9s-style) for managing cloud resources.

Currently targeting AWS:

- **Secrets Manager** — list, inspect (masked by default), create, update
- **SSM Parameter Store** — browse by path, get/put parameters
- **Amazon MQ (ActiveMQ)** — list queues, browse/send/purge/move messages

## Repository layout

| Path        | Description                                                        |
|-------------|--------------------------------------------------------------------|
| `tui/`      | Go TUI application (tview/tcell)                                   |
| `mq-proxy/` | Spring Boot REST proxy for queue mutations on Amazon MQ (JMS-based)|
| `api/`      | OpenAPI specification for the mq-proxy — single source of truth    |
| `docs/`     | Architecture notes and decisions                                   |

## Why a Java proxy?

Amazon MQ exposes the ActiveMQ Jolokia API **read-only** and does not expose
remote JMX. Mutating queue operations (purge, delete, move) therefore go
through a small Spring Boot service that talks to the broker over JMS/OpenWire.
For local development, the TUI can talk to a local ActiveMQ's Jolokia API
directly — both backends implement the same Go interface.

## Status

Early scaffolding. See `CLAUDE.md` for repository conventions.
