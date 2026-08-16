# Plan — Bugfix 58: retry a proxy GET once on a cold-start timeout

## Approach

`doRequest` currently builds one `*http.Request` and calls
`c.httpClient.Do` once. Split request construction into a small closure so
it can be called a second time on retry, and retry exactly that `Do` call
— not the whole function — when it fails and `method == http.MethodGet`.
Nothing downstream of `Do` (status check, body decode) changes.

```go
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, out interface{}) error {
	newReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.username, c.password)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	}

	req, err := newReq()
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil && method == http.MethodGet {
		// The proxy can time out waiting for headers on its first request
		// after a period of inactivity (JVM/DB connection warmup on
		// AWS-hosted deployments) even though it's otherwise healthy — a
		// second identical request typically succeeds immediately.
		// Retried only for GET: a body-less, side-effect-free request is
		// always safe to repeat; a POST that timed out waiting for a
		// response may already have been applied server-side, so retrying
		// it risks applying it twice.
		if retryReq, rerr := newReq(); rerr == nil {
			resp, err = c.httpClient.Do(retryReq)
		}
	}
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	// ...status check, decode: unchanged...
}
```

`body` is always `nil` for GET (only `getJSON` passes `nil`; `postJSON`
passes an `io.Reader` for POST, which never hits the retry branch), so
`newReq()`'s second call never needs to rewind a consumed body — building
a completely fresh `*http.Request` for the retry is both correct and
simpler than trying to reuse the first one.

Not distinguishing timeout vs. other transport errors (see spec.md) — any
non-nil error from `Do` means no response was received at all, so the
retry condition is just `err != nil && method == http.MethodGet`.

## Files touched

### `tui/internal/queue/proxy/proxy.go`

- `doRequest`: refactored as above. No change to `getJSON`/`postJSON`'s
  signatures or any caller.

### `tui/internal/queue/proxy/proxy_test.go`

- New tests using `httptest.NewServer` with a handler that, on the first
  request only, hijacks the connection and closes it without writing a
  response — this produces an immediate transport-level error
  (`Do` returns non-nil) without needing to wait out a real timeout, so
  the tests stay fast:
  - `TestGetRetriesOnceOnTransportError`: handler closes connection #1,
    responds normally to #2 → `List` (a GET) succeeds, using the second
    response's data.
  - `TestGetFailsAfterTwoTransportErrors`: handler closes both connection
    #1 and #2 → `List` returns an error (confirms exactly one retry, not
    an infinite loop or three attempts).
  - `TestPostDoesNotRetryOnTransportError`: handler closes connection #1
    only, would respond normally to #2 → a POST-backed call (e.g.
    `SendMessage`) still returns an error, proving the second (would-be
    successful) attempt is never made.
- Hijacking requires the test server's handler to type-assert
  `http.ResponseWriter` to `http.Hijacker` — standard library, no new
  dependency.

## Testing

Covered entirely by the three unit tests above — no manual/live step is
required for the fix itself, since the retry logic is exercised
deterministically and quickly via the hijack-and-close technique, not by
reproducing an actual 30s AWS cold start. If a real AWS-hosted mq-proxy
instance is available at review time, a quick sanity check (activate the
connection after a period of inactivity, list queues, confirm no error) is
a nice-to-have but not blocking — noted as an optional manual step in
`tasks.md` rather than a hard requirement, per `tui/CLAUDE.md`'s allowance
for genuinely hard-to-reproduce-live scenarios.
