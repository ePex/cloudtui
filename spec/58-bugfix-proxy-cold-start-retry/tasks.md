# Tasks — Bugfix 58: retry a proxy GET once on a cold-start timeout

1. [x] Refactor `doRequest` in `tui/internal/queue/proxy/proxy.go`: extract
       request construction into a `newReq()` closure; on a GET whose
       `httpClient.Do` call fails, build a fresh request via `newReq()` and
       retry the `Do` call exactly once. No change to `getJSON`/
       `postJSON`'s signatures or any caller.
2. [x] Add to `tui/internal/queue/proxy/proxy_test.go`:
       `TestGetRetriesOnceOnTransportError`,
       `TestGetFailsAfterTwoTransportErrors`,
       `TestPostDoesNotRetryOnTransportError` — using an `httptest` handler
       that hijacks and closes the connection on the first request (to
       simulate a transport-level failure fast, without a real 30s wait).
3. [x] Verify: `go build ./...` and `go test ./...` pass in `tui/`; if a
       real AWS-hosted mq-proxy instance is available, an optional sanity
       check — activate the connection after a period of inactivity, list
       queues, confirm no error — but not blocking (unit tests are the
       real coverage here, per plan.md).

       `gofmt`, `go vet`, `go build ./...`, `go test ./...` all clean.

       The optional live check against the real AWS-hosted mq-proxy
       instance was **not performed** — that instance is only reachable
       from the user's own environment/AWS profile, not this session.
       Unit test coverage (`TestGetRetriesOnceOnTransportError`,
       `TestGetFailsAfterTwoTransportErrors`,
       `TestPostDoesNotRetryOnTransportError`) stands in, per plan.md — it
       deterministically exercises the exact failure/retry mechanics
       (transport-level error on `Do`, GET retries once, POST never
       retries) without depending on reproducing a real ~30s cold start.
       The user reported the original symptom firsthand and can confirm
       against the real instance after this ships.
