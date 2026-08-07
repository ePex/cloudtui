# Plan — FE 23: seed-queue dev tool

## Approach

Two new packages plus one Taskfile entry, no new dependencies.

## `tui/internal/seed/seed.go`

```go
type Sender interface {
    SendMessage(ctx context.Context, queueName, body string) error
}

func Run(ctx context.Context, sender Sender, queueName string, count int, progress func(sent, total int)) error
```

`Sender` is a narrow interface (not `queue.Backend`) so `seed.Run` doesn't
depend on the full backend surface and is trivial to fake in tests.
`jolokia.Client` already satisfies it since `SendMessage` has the matching
signature.

Sample messages are a small `sampleEvent` struct marshaled to JSON, with
`id` sequential (1..count) and `event`/`customer` chosen via `math/rand`
from fixed lists (auto-seeded; Go 1.20+ global source).

## `tui/cmd/seedqueue/main.go`

Thin wrapper, consistent with `cmd/tui/main.go`'s "entrypoint only" rule:

1. Parse `<queue-name> <count>` positional args via `flag`.
2. `config.LoadDefault()` → `cfg.ActiveConn()`.
3. Reject non-jolokia active connections with a clear error (out of scope
   per spec.md).
4. `jolokia.NewClient(conn.Queue)`, call `seed.Run` with a progress callback
   that prints `sent %d/%d to %q`.

## Taskfile

```yaml
seed:queue:
  desc: 'Send sample JSON messages to a queue via Jolokia. Usage: task seed:queue -- <queue> <count>'
  dir: tui
  cmds:
    - go run ./cmd/seedqueue {{.CLI_ARGS}}
```

## Testing

`seed_test.go` uses a `fakeSender` (in-memory, records bodies, can be told
to fail on the Nth call) — no network/broker dependency:

- `TestRunSendsCountMessages`
- `TestRunMessagesAreValidJSONWithSequentialIDs`
- `TestRunReportsProgress`
- `TestRunStopsAndReturnsErrorOnSendFailure`

`cmd/seedqueue` itself has no unit tests (thin wiring, same as `cmd/tui`) —
verified manually against a live broker instead (see spec.md, Definition of
done #3).

## No new dependencies

Only standard library (`encoding/json`, `math/rand`, `flag`, `strconv`) plus
existing `internal/config` and `internal/queue/jolokia`.
