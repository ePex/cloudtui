# Bugfix: mq-proxy retries a GET the user cancelled

Date: 2026-10-17

## What's broken

The mq-proxy client retries a GET once when it got no response at all
(spec/11, "GET requests retry exactly once"), to get past a proxy that's
slow on its first request after a while. It retries on *any* failure,
including a request whose context was cancelled: Esc on "Loading
queues" (spec/24) or a superseded load. That retry is sent with the
same, already cancelled context, so it fails at once.

Found through the request log (spec/06): Esc during a queue load over
mq-proxy logs the cancelled request and then a second, instantly
cancelled one. Harmless, but it's a pointless request attempt, a
confusing extra log line, and the error returned is the retry's rather
than the original's.

## Desired behavior

- **No retry once the request's context is done** (cancelled, or past
  its deadline): the GET fails with the original error, and only that
  one request is logged.
- **Unchanged:** the retry for every other failure with no response,
  including the proxy client's own 30 s timeout and a refused
  connection (the warm-up case the retry is for); POSTs never retry.

## Out of scope

- Any other change to the retry (count, backoff, which methods).
- Jolokia (it has no such retry).
