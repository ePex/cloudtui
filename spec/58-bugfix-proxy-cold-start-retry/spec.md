# Spec — Bugfix 58: retry a proxy GET once on a cold-start timeout

Date: 2026-08-16

## Background

`internal/queue/proxy/proxy.go`'s `Client` talks to mq-proxy over HTTP with
a fixed 30s `http.Client{Timeout: ...}` and no retry logic anywhere.

## Problem

Against an AWS-hosted mq-proxy instance, the very first request after a
period of inactivity (list queues, in the reported case) times out waiting
for response headers — the underlying JVM/DB connection needs to "warm up"
before it can respond. The identical request immediately after succeeds
fast, with no other change. From the user's point of view, cloudtui shows a
spurious error against an otherwise perfectly healthy proxy.

## Solution

Retry exactly once, and only for GET requests (`getJSON`, which backs
`List` and `BrowseMessages`), when the HTTP round trip itself fails —
i.e. `http.Client.Do` returns a non-nil error (connection/timeout/DNS
failure — no response was ever received) — not when a response comes back
with an error status code or malformed body, which are legitimate
application-level failures a retry can't fix.

Scoped to GET only, matching the same reasoning already established for
`secretBackend`'s read/write asymmetry
(`spec/56-fe-amq-connection-aws-secret-password`): a GET is idempotent and
side-effect-free, so replaying it after a failed attempt is always safe. A
POST (send/delete/move message) that times out waiting for a response may
have already been applied server-side — retrying it blindly risks applying
it twice. This also happens to match the reported symptom, since list
operations are exactly what a user does first after switching to a cold
connection.

Not distinguishing further between timeout vs. other transport errors
(connection refused, DNS failure, etc.) — any of them means no response was
received, so a single blind retry is equally safe and simple for all of
them. Not retrying more than once — if the second attempt also fails, the
proxy has a real problem and that should surface as an error, not loop.

## Scope

### In scope

- `proxy.Client.doRequest`: on a GET whose `httpClient.Do` call fails,
  build a fresh `*http.Request` (bodies are always `nil` for GET, so
  there's nothing to rewind) and retry once before giving up.
- Unit tests using `httptest` + connection hijack-and-close (to produce an
  immediate transport-level failure without waiting out a real timeout):
  GET retries once and succeeds when the second attempt would; GET still
  fails when both attempts fail; POST does *not* retry.

### Out of scope

- The jolokia backend — not reported to have this problem, and Jolokia's
  typical deployment (a co-located ActiveMQ web console) doesn't share the
  AWS cold-start characteristics of a separately-hosted mq-proxy instance.
  Can be revisited if it turns out to need the same treatment.
- Retrying POST requests, or any smarter idempotency-aware retry for them.
- Proactive "warm up the connection on activation" — a background no-op
  call fired when a proxy connection becomes active, to reduce the odds of
  ever hitting the cold path at all. Would complement this fix but is a
  separate, more involved change (needs its own async wiring); this bugfix
  is the minimal, guaranteed-effective fix for the reported symptom.
- Changing the 30s timeout value itself.
- Retry count/backoff configuration — always exactly one retry, no delay.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. A GET that fails once (transport-level) then would succeed on retry
   returns the successful result, transparently.
3. A GET that fails on both attempts returns the second attempt's error.
4. A POST that fails once is *not* retried — returns the error from the
   single attempt.
5. Manually verified against a real cold mq-proxy instance if practical
   (see `tasks.md`); if not practical to reproduce live, the unit test
   coverage above stands in, per `tui/CLAUDE.md`'s "say so explicitly"
   guidance for genuinely hard-to-reproduce-live scenarios.
