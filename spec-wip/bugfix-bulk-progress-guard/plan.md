# Plan: bulk delete/move can repeat for hours when the server over-reports

See `spec.md`. All code changes are in `tui/internal/queue/proxy/proxy.go`
and its tests; no view changes.

## 1. Requests return message IDs, not a count

`deleteOnce`/`moveOnce` return the response's message IDs (`[]string`)
instead of `len(env.Data)`. Entries with an empty `messageId` are dropped
right there — the spec counts only entries with an ID, for single
requests too:

- `DeleteMessages`/`MoveMessages` with a `MaxCount` (incl. `RemoveMessage`/
  `MoveMessage`'s `MaxCount: 1`): count = IDs returned. So
  `RemoveMessage`/`MoveMessage` now report "not found" when the server
  answers with an entry that has no ID, instead of claiming success.
- No `MaxCount`: rounds (section 2).

## 2. `inRounds` tracks IDs and stops when a round makes no progress

`inRounds(ctx, round func() (ids []string, reported int, err error))`:
`reported` is the number of response entries (with or without an ID),
`ids` the non-empty ones. A `seen` set of IDs per operation:

- `reported == 0` → done, return the total (unchanged).
- New IDs = `ids` not in `seen`; add them; `total += len(new)`.
- `reported > 0` and no new IDs → **no progress**: return the total with
  `errBulkNoProgress`, wrapped as `incomplete: the server kept reporting
  <k> message(s) it didn't remove (<total> removed)`, `<k>` = this
  round's `reported`. With a server that reports stuck messages, that's
  after at most one round past the last real progress (the first round
  where only stuck ones are left).
- Otherwise continue; context check, error-with-total and the
  `maxBulkRounds` cap stay as they are.

**Why "seen before" and not the queue's pending count:** no extra
request per round, and it doesn't misfire when producers add messages
while purging. A really removed message can't be reported again: `mq-proxy`
consumes (each message once), and a moved message gets a new ID in the
target, so the source IDs stay unique.

**Why an error, not a warning on the result line:** it's an incomplete
operation, like the existing `errBulkIncomplete`. Both views already
show bulk errors (`purgeFlow`'s and move-all's `onError`/`showError`), and
the message carries both numbers the spec asks for — no view change. The
cost: the count isn't in the green `Purged <n>` line but in the error
text.

## 3. Tests (`proxy_test.go`)

- `cappedServer` (from PR #40) numbers IDs from 0 in every round, which
  the new check would read as "no progress". It gets a running counter
  so every reported message has a unique ID, like a real server.
- New `cappedServer` options: `stuck` IDs reported in every round (the
  over-reporting server), and `noID` entries without a messageId.
- Cases:
  - 1,000 messages at 400 per round plus 1 stuck message → stops with
    `errBulkNoProgress`, total 1,000, a handful of requests (not 1,000);
    the error mentions 1 message kept reported and 1,000 removed.
  - Only ID-less entries → stops after the first round, total 0,
    `errBulkNoProgress`.
  - New IDs mixed with ID-less entries → only the IDs count.
  - `RemoveMessage`/`MoveMessage` with an ID-less entry → "not found".
  - The existing round tests (incl. the 1,000-request cap with a server
    that keeps reporting *new* IDs) still pass unchanged in meaning.
- Mutation-check: disable the no-progress check → the stuck test runs
  into the 1,000-request cap instead.

## 4. Specs (merge-back)

- `spec/11` Go client: the round rule gains "stops when a round reports
  nothing new"; counts only entries with an ID; single-message calls
  treat an ID-less entry as not found; why "seen before" over pending
  count.
- `spec/09`'s purge count note: counts are of messages reported for the
  first time.

## Manual verification

This repo's `mq-proxy` never over-reports, so the new stop can't be
triggered live against it; the unit tests with the fake server cover
it. Live, against `mq-proxy`: a regression check that purge and
move-all on a 1,000+ message queue still empty it with the right count.
Against the other server (before its own fix), the user can check that
a purge with a message held by a live consumer stops within seconds
with the "incomplete" error.

No new dependencies.
