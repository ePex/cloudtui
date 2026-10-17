# Plan: mq-proxy retries a GET the user cancelled

Implements `spec.md` in this folder.

## Where it happens

`proxy.Client.doRequest` (`internal/queue/proxy/proxy.go`):

```go
resp, err := c.httpClient.Do(req)
if err != nil && method == http.MethodGet {
	// retry once with a new request on the same ctx
}
```

The retry condition doesn't look at why `Do` failed, so a request
whose `ctx` was cancelled (Esc, a superseded load) or ran past its
deadline is sent again on that same, already done `ctx`.

## Approach

Retry only while the request's context is still live:

```go
if err != nil && method == http.MethodGet && ctx.Err() == nil {
```

The comment above the retry gains why: a done context means the
caller gave up, and a retry on it can only fail.

Why this keeps the warm-up retry: the proxy client's 30 s limit is
`http.Client.Timeout`, which `net/http` enforces with its own internal
deadline on the request, not by cancelling the caller's `ctx`; after
such a timeout `ctx.Err()` is still nil, so the retry happens as
before. Same for a refused connection.

## Files

- `tui/internal/queue/proxy/proxy.go` (+ `proxy_test.go`).
- Merge-back: spec/11 (the retry rule: not after the caller's context
  is done), spec/06 (drop the note that a cancelled mq-proxy GET logs
  a second, instantly cancelled line).

## Testing

`proxy_test.go`, against an `httptest` server counting requests:

- **cancelled:** the server holds the request until its context ends;
  the test cancels the caller's context once the request arrived:
  `List` returns an error wrapping `context.Canceled`, and the server
  saw exactly 1 request.
- **deadline:** the same with a caller context that times out:
  1 request, the error wraps `context.DeadlineExceeded`.
- **client timeout still retried:** the client's `httpClient.Timeout`
  set to a short value, the server slow only on the first request:
  `List` succeeds, the server saw 2 requests.
- The existing warm-up test (`TestWithRequestLogLogsTheWarmupRetry`,
  a dropped connection) keeps passing.
- Mutation check: without `ctx.Err() == nil`, the cancelled and
  deadline tests see 2 requests and fail.
- Live (isolated HOME, local broker, `task dev:proxy:start`): Esc
  during a queue load over mq-proxy; the log has one cancelled line
  for that load, not two.
