# FE 50 — REST-level verify script for mq-proxy / reference API

Date: 2026-08-13

## What

`tui/scripts/verify-proxy-api.sh`: a `curl`+`jq` script that exercises
the shared REST contract (`mq-proxy`, and — since CR 49 — the reference
API too, now that both speak the same nested-filter shape) directly
against a live backend, without going through the TUI. Run once per
backend (base URL + credentials + a queue name), it: sends 10 messages
(mixed JMS types, one with a correlation ID/custom header), checks the
message-detail fields a message needs to render `tui`'s detail view,
checks filtering (by JMS type and by `maxCount`), deletes one message,
bulk-deletes several, moves one message, bulk-moves several, and purges
what's left — printing pass/fail per step and a final summary.

## Why

Requested directly: manually re-running this same sequence by hand via
individual `curl` calls against `mq-proxy` and the reference API,
repeatedly, across this session's verification work (CR 45 through
CR 49) was tedious and error-prone (e.g. the tab-focus fumbling
`tview.Form` testing needed, or hand-tracking message IDs across
multiple `curl` calls). This is the REST-level counterpart to
`smoke-test.sh` (FE 26, which drives the actual TUI binary in tmux) —
this script skips the TUI/tmux layer entirely and talks to the backend
directly, since what's being checked here is the wire contract both
backends implement, not TUI rendering.

## Scope

- Positional args: `<base-url> <username> <password> <queue>
  [target-queue]`. `queue` must already exist/be authorized for the
  given credentials (mq-proxy auto-creates on first send; the reference
  API's credentials are typically restricted to specific pre-existing
  queues — see CR 44/45's investigation). `target-queue` defaults to
  `<queue>-verify-target`.
- **Destructive by design, clearly documented as such**: the script ends
  by purging both queues, matching `test:queue:add`/`seed:queue`'s
  existing no-confirmation-prompt convention for disposable-queue
  tooling — the usage text is the safeguard, not an interactive prompt.
- Steps, each printing PASS/FAIL with expected-vs-actual: send 10
  messages (7 `order-created`, 3 `invoice-created`, one with a
  correlation ID + custom header) → verify count → verify one message's
  full field set (id, jmsType, timestamp, body, headers) → filter by
  JMS type (both types) and by `maxCount` → delete one (by exact ID) →
  bulk-delete (by JMS type, matches the 3 `invoice-created`) → move one
  (by exact ID) → bulk-move (`maxCount`-capped) → purge both queues.
  No manual delays/retries needed anywhere (bugfix 47 made
  delete/move reliable immediately).
- Exits non-zero if any check failed; final line summarizes pass/fail
  counts.
- `task verify:proxy-api` wired in `Taskfile.yml`, mirroring `task
  smoke:test`'s existing entry for `smoke-test.sh`.

## Out of scope

- **Not TUI-driving** (no tmux) — that's `smoke-test.sh`'s job for the
  Jolokia/mq-proxy golden path; this script is REST-only and covers both
  backends, which `smoke-test.sh` doesn't.
- **No CI wiring** — manual/on-demand only, like `smoke-test.sh`
  (needs a live broker + credentials, not something CI can run
  unattended).
- **No queue creation/deletion via JMX** — the script only uses the REST
  surface it's testing; a leftover empty queue after a run is harmless
  (same as `smoke-test.sh`/`seed:queue`'s existing disposable-queue
  precedent).
