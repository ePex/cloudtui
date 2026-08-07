# Spec — FE 23: seed-queue dev tool

Date: 2026-08-08

## Background

Testing queue browsing/filtering/message-detail features by hand requires a
queue with some messages in it. There was no quick way to populate a queue
with sample data without using the TUI's own send-message overlay one
message at a time.

## Problem

No convenient way to bulk-seed a queue with sample JSON messages for local
development and manual testing.

## Solution

A small Go command, `cmd/seedqueue`, that sends a given number of sample
JSON messages to a named queue via the active connection's Jolokia backend,
reusing the existing `jolokia.Client.SendMessage`. Wired into `Taskfile.yml`
as `task seed:queue -- <queue> <count>`.

A Go tool (rather than a shell script) was chosen to satisfy the repo's
cross-platform requirement (Windows/Linux/macOS via `task`, no bash/curl
dependency) and to reuse already-tested backend code instead of
reimplementing the Jolokia JMX call shape.

## Scope

### In scope

- `tui/internal/seed/` — message generation (`sampleMessage`) and the send
  loop (`Run`), decoupled from a concrete backend via a small `Sender`
  interface (`SendMessage(ctx, queueName, body string) error`).
- `tui/cmd/seedqueue/` — thin CLI wrapper: loads config, resolves the active
  connection, constructs a `jolokia.Client`, calls `seed.Run`.
- `Taskfile.yml` — `seed:queue` task.
- Unit tests for `seed.Run` (count sent, valid JSON with sequential IDs,
  progress callback, stop-on-error).

### Out of scope

- Sending via the mq-proxy backend (Jolokia only, per explicit choice).
- Creating the destination queue if it doesn't exist — ActiveMQ's JMX
  `sendTextMessage` requires the queue MBean to already exist, same
  constraint the TUI's own send-message overlay has (it only offers
  already-listed queues).
- Configurable message shape/content (fixed sample "order event" JSON
  payload with a sequential `id`).

## Message shape

```json
{"id": 1, "event": "order.created", "timestamp": "...", "amount": 12.34, "customer": "acme-corp"}
```

`event` and `customer` are picked at random from small fixed lists; `id` is
sequential starting at 1.

## Files touched

| File | Change |
|---|---|
| `tui/internal/seed/seed.go` | new — `Run` + sample message generation |
| `tui/internal/seed/seed_test.go` | new — unit tests |
| `tui/cmd/seedqueue/main.go` | new — CLI entrypoint |
| `Taskfile.yml` | add `seed:queue` task |
| `tui/CLAUDE.md` | document the two new packages |

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `task seed:queue -- <queue> <count>` sends `<count>` valid JSON messages
   to `<queue>` on the active Jolokia connection.
3. Verified live against a real broker (`orders` test queue): sent 3
   messages, confirmed valid sequential JSON bodies via `BrowseMessages`,
   then purged to leave the queue as found (it was empty beforehand).
