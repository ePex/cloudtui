# Tasks — FE 50

1. [x] `tui/scripts/verify-proxy-api.sh`: all steps from `plan.md`
   (send/count/detail-field-check/filter/delete-one/delete-several/
   move-one/move-several/purge-both), with pass/fail tracking and a
   final summary + exit code.
2. [x] `Taskfile.yml`: `verify:proxy-api` entry.

## Manual verification

- Run against an isolated `mq-proxy` instance + disposable queue — all
  steps PASS; independently confirm both queues end empty.
- Run against the reference API + an authorized test queue — all steps
  PASS.
- Temporarily break one expected value, confirm the script reports that
  one step FAIL, the rest PASS, and a non-zero exit code; then revert.

**Verified 2026-08-13.** First run against a real, non-empty reference
API target queue caught two real bugs in the script itself (not the
backends), both fixed:

- The detail-view field check didn't request `returnBody=true`
  explicitly — the reference API defaults it to `false` (mq-proxy
  defaults `true`), so `body` came back null there and the check
  spuriously failed. Fixed by requesting it explicitly, matching what
  `tui` itself always does (FE 46).
- Absolute count assertions broke when `target-queue` already had
  messages on it (a real pre-existing queue, not something this run
  added). Rewrote every assertion to be relative to a baseline count
  captured at the start for both `queue` and `target-queue`, with a
  `WARNING` line when either starts non-empty — makes the script
  correct even when a queue isn't perfectly empty, rather than requiring
  the user to guarantee that by hand (deliberately tested: reran with a
  1-message pre-existing target queue, confirmed the warning fires and
  all 18 checks still pass).

Final confirmed runs, all 18/18 PASS: twice against an isolated
mq-proxy instance (`SERVER_PORT=8090`, not the user's own running one),
once against the real reference API with a genuinely non-empty target
queue, once more against mq-proxy with a deliberately non-empty target
queue (warning-path check). Also confirmed the deliberate-breakage path
(one wrong expected value → exactly that step FAILs, 17 others still
PASS, exit code 1) still holds after the rewrite. All test queues ended
empty (via the script's own purge step) or were left with only the
harmless single leftover message from the warning-path test.
