# Bugfix: bulk delete/move can repeat for hours when the server over-reports

Date: 2026-10-10

## Problem

Since bugfix-purge-completeness (PR #40, spec/11), the proxy client
repeats purge, move-all and bulk delete/move without a `maxCount` until a
request acts on 0 messages, capped at 1,000 requests. That trusts the
server's count to mean "messages actually removed".

Nothing in the wire contract guarantees that. A server that implements
delete/move as "browse, then consume each browsed message by ID" can
report one entry per *browsed* message even when the consume for it came
back empty — found by reading such an implementation, not reproduced
live. A message it can browse but not consume (most plausibly one
already dispatched to a live consumer) is then reported in every round:

- No round ever reports 0, so the client keeps repeating until the
  1,000-request cap.
- Each round waits for the consume to time out on every stuck message
  (10 s per message in the implementation read), so a single stuck
  message means close to 3 hours before the "incomplete" error.
- The purge runs in the background, so the TUI stays usable, but no
  result appears for that long, and the reported count includes
  messages that were never removed.

This repo's `mq-proxy` consumes rather than browses, so it reports only
messages it actually received; it isn't affected.

## Fix (expected behavior)

- **A round that makes no progress ends the repetition.** "No progress":
  every message the round reported was already reported by an earlier
  round of the same operation — a message that was really removed can't
  be reported again — or the round reported no message IDs at all.
- **Only messages reported for the first time count** toward the total
  (the `Purged <n>`/`Moved <n>` numbers). Entries without a message ID
  don't count.
- **When it stops for lack of progress while the server still reports
  messages, the user is told the operation is incomplete**, with how
  many were removed and how many the server kept reporting — within
  seconds of the last real progress, not hours. (Exact wording and
  whether it's an error or a warning on the result line: plan stage.)
- Unchanged: a round reporting 0 ends the operation normally; the
  1,000-request cap stays as a last resort; single-message and
  `maxCount` calls make one request; POSTs never retry; against this
  repo's `mq-proxy` nothing changes (every reported message is new).
- No extra requests: the check uses only the IDs in the responses the
  client already gets.

## Out of scope

- Changing other servers (a separate change in that server's own repo:
  report only messages actually consumed).
- Detecting *why* a message can't be removed (e.g. which consumer holds
  it).
- The Jolokia backend (its bulk operations don't repeat).
