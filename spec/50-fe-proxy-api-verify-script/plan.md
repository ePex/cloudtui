# Plan — FE 50

## Approach

Single self-contained bash script, no new Go/Kotlin code. Structure
mirrors `smoke-test.sh`'s style (`set -euo pipefail`-adjacent, a `log`
helper, positional args) but assertions don't abort the script —
`pass`/`fail` helpers record outcomes and the script keeps going, so one
run surfaces every broken step instead of stopping at the first.

1. **Arg parsing**: `BASE_URL`, `USERNAME`, `PASSWORD`, `QUEUE` required;
   `TARGET_QUEUE` defaults to `${QUEUE}-verify-target`. Usage/help text
   on missing args includes the safety note from `spec.md`.
2. **`api()` helper**: wraps `curl -s -u "$USERNAME:$PASSWORD"`, GET or
   POST with a JSON body, returns the raw response body.
3. **`count()` helper**: `list-messages?sourceQueue=<q>[&<extra>]` piped
   through `jq '.data | length'`.
4. **Send phase**: loop sending 10 messages via `send-message`
   (`jmsType` alternating 7×`order-created`/3×`invoice-created`; message
   #1 also carries `correlationId` and a `headers` entry), capturing each
   `messageId` into a bash array via `jq -r '.data.messageId'`. Assert
   final count == 10.
5. **Detail-view field check**: `list-messages?...&filter.maxCount=1`,
   assert via `jq -e` that `messageId`/`jmsType`/`timestamp`/`body`/
   `headers` are all present (non-null) on the first result.
6. **Filter checks**: `filter.jmsType=order-created` → count == 7;
   `filter.jmsType=invoice-created` → count == 3; `filter.maxCount=2`
   (no type filter) → count == 2. Confirms both the reference API's and
   `mq-proxy`'s (post–CR 49) nested query shape.
7. **Delete one**: `delete-messages` with
   `filter:{messageId:<array[0]>,maxCount:1}` → assert response `data`
   has length 1 and count drops to 9.
8. **Delete several**: `filter:{jmsType:"invoice-created",maxCount:3}` →
   assert `data` length 3, count drops to 6.
9. **Move one**: `move-messages` with `filter:{messageId:<array[1]>,
   maxCount:1}` to `TARGET_QUEUE` → assert source count 5, target count 1.
10. **Move several**: `filter:{maxCount:2}` → assert source count 3,
    target count 3.
11. **Purge both**: `delete-messages` with `filter:{}` against `QUEUE`
    then `TARGET_QUEUE` → assert both counts reach 0.
12. **Summary**: print `N/M checks passed`; `exit 1` if any failed, `exit
    0` otherwise.

## Files touched

- `tui/scripts/verify-proxy-api.sh` (new)
- `Taskfile.yml` — `verify:proxy-api` entry
- `spec/50-fe-proxy-api-verify-script/tasks.md` (next gate)

## Key decisions

- **`jq`, not `python3`**, for JSON extraction — more idiomatic for a
  pure-bash REST script (`smoke-test.sh` needs neither, since it drives
  the TUI instead of parsing JSON), and this session already confirmed
  `jq` is available on this machine.
- **Deterministic message layout** (7/3 type split, fixed indices used
  for the one/several delete/move steps) rather than randomized — makes
  expected counts at each step exact and the script's own assertions
  simple, at the cost of the JMS-type distribution being hardcoded
  rather than configurable. Fine for a fixed 10-message run.
- **No interactive confirmation before the purge step** — matches
  `test:queue:add`/`remove`/`seed:queue`'s existing convention; the
  destructive nature is a documentation/usage concern, not a runtime
  gate, consistent with this repo's existing disposable-queue tooling.
- **Assertions don't `exit` on failure** — a diagnostic script is more
  useful reporting "7 of 10 steps passed, here's which 3 failed" than
  stopping at the first broken step.

## Manual verification

This IS the verification tool, so "testing" it means running it for
real against both backends and confirming its own pass/fail output
matches reality:

- Run against an isolated `mq-proxy` instance with a disposable queue —
  confirm all steps report PASS, and independently confirm via a
  separate `curl` that both queues end empty.
- Run against the reference API with an authorized test queue — confirm
  all steps report PASS.
- Deliberately break one assertion's expected value (temporarily) and
  confirm the script reports FAIL for that step, PASS for the others,
  and exits non-zero — confirms the "keep going, report everything"
  behavior actually works, not just the happy path.
