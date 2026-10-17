# Plan: log every queue backend HTTP request

Implements `spec.md` in this folder.

## Where requests happen today

- `jolokia.Client` (`internal/queue/jolokia/`) sends every request
  through its own `c.http` (`*http.Client`), from about a dozen call
  sites: the queue search (`GET …/search/…`), bulk metric reads, browse,
  send, remove, move, purge, queue size reads. Mutations use
  `sendCtx` (`context.WithoutCancel`), so they're never cancelled.
- `proxy.Client` (`internal/queue/proxy/`) sends everything through
  `doRequest` on `c.httpClient` (30 s timeout), which retries GETs
  while a slow mq-proxy warms up; each retry is its own request.
- Both are built by `secretbackend.buildBackend(conn)`, which knows the
  connection's name. `cmd/seedqueue` (a dev tool) also builds a
  `jolokia.Client`.

## Approach: a logging `http.RoundTripper`

A new package `internal/queue/httplog` with a `Transport` that wraps
another `RoundTripper` and logs each exchange. Wrapping the transport
covers every call site without touching them, and retries come out as
separate lines for free.

`httplog.Transport` fields: `Base` (the wrapped transport, default
`http.DefaultTransport`), `Backend` and `Connection` (names for the
log line), `Describe func(*http.Request) string` (the operation) and
`AppStatus func(body []byte) string` (a status from the response body,
or ""; Jolokia only). `RoundTrip`:

1. Takes the operation from `Describe` before sending. A request body
   is read through `req.GetBody` (a fresh copy, which
   `http.NewRequestWithContext` sets for the `bytes.Reader` bodies both
   clients use), never from `req.Body` itself.
2. Sends the request through `Base`.
3. On an error: if the request's context was cancelled
   (`context.Canceled`, i.e. Esc), logs `INFO` "cancelled"; otherwise
   `WARN` with the error (including the proxy client's 30 s timeout).
4. On a response: reads the whole body (both clients read it fully
   anyway), closes it, and hands the client an identical body
   (`io.NopCloser(bytes.NewReader(body))`; a read error is passed on
   after the bytes that were read, so the client sees the same failure).
   That gives the size, a duration that includes the body, and the
   bytes for `AppStatus`.
5. Logs one line: `INFO`, or `WARN` if the status isn't 2xx or
   `AppStatus` reports a failure.

The line: message `queue backend request`, attributes `backend`,
`connection`, `method`, `url` (copy of the URL with `User` removed),
`op`, `status`, `jolokiaStatus` (Jolokia only), `duration`, `bytes`,
and `error` on failures. Headers and bodies never go into it; only the
`Describe`/`AppStatus` results, which extract named fields.

The logger is `slog.Default()` at the time of each request (set up in
`main.go`, spec/06); tests set a `Logger` field instead.

## Backend specifics

- **Jolokia** (`internal/queue/jolokia/httplog.go`, new):
  - `describeRequest`: a `GET` is a search (`search`). A `POST` body is
    one request object or an array of them; only `type`, `mbean`,
    `operation` and `attribute` are decoded (arguments are never
    touched). One object: `exec removeMessage(…)` →
    `exec removeMessage on <mbean>`, `read QueueSize on <mbean>`. An
    array: `bulk: <n> × <type> <names joined by "/">` (e.g. `bulk: 48 ×
    read QueueSize/ConsumerCount/EnqueueCount/DequeueCount`).
  - `appStatus`: decodes only `status` from the reply (one object, or an
    array): `200`, or for a bulk reply the counts per status (`200×46,
    404×2`). Anything other than all-200 makes the line `WARN`. A body
    that isn't JSON gives "".
- **mq-proxy** (`internal/queue/proxy/httplog.go`, new): the operation
  is `req.URL.Path`; no `AppStatus` (the HTTP status already carries
  the result).
- Each client gets a `WithRequestLog(connection string)` method that
  wraps its `http.Client`'s transport in an `httplog.Transport`.
  `secretbackend.buildBackend` calls it with `conn.Name`.
  `cmd/seedqueue` doesn't, so the dev tool's output stays as it is.

Mutations' `sendCtx` (never cancelled) is unchanged; their lines are
never "cancelled".

## Trade-offs

- Reading the body in the transport holds a response in memory once
  more while it's handed on. The clients already decode whole responses
  (browse replies of up to a few thousand messages), so this doubles a
  short-lived buffer rather than adding a new kind of cost.
- `INFO` for every request: a busy session (paging through messages,
  bulk moves of single messages) writes many lines; the file is still
  truncated on each start (spec/06). Accepted per the spec.

## Files

- `tui/internal/queue/httplog/httplog.go` (+ test): new.
- `tui/internal/queue/jolokia/httplog.go` (+ test), `jolokia.go`
  (`WithRequestLog`).
- `tui/internal/queue/proxy/httplog.go` (+ test), `proxy.go`
  (`WithRequestLog`).
- `tui/internal/queue/secretbackend/secretbackend.go`: call it.
- Merge-back: spec/06 (what's logged at runtime), spec/07 and spec/11
  (a line on each backend), spec/03 and `tui/CLAUDE.md` (the new
  package).

## Testing

- **`httplog.Transport`**, against an `httptest` server, with a
  capturing `slog` handler:
  - a 200 response: one `INFO` line with every attribute; the client
    gets the body byte for byte;
  - a 500: `WARN`; an `AppStatus` failure: `WARN`;
  - a refused connection: `WARN` with the error;
  - a request cancelled through its context: `INFO`, "cancelled";
  - the request's `Authorization` header value, a password in the URL
    and the request/response bodies never appear in the log output;
  - the request body still reaches the server intact after `Describe`
    read it.
- **Jolokia**: `describeRequest` and `appStatus` table tests (single
  exec, single read, bulk, search GET, non-JSON); and one test of the
  real `Client` with `WithRequestLog` against a fake Jolokia server:
  `List` logs the search and the bulk read.
- **mq-proxy**: the operation is the path; a `Client` with
  `WithRequestLog` logs a line per request, including each warm-up
  retry.
- **Mutation checks:** no transport wrapping in `WithRequestLog`; a
  `Describe` that reads `req.Body` (the server gets an empty body);
  logging headers; no `WARN` for a Jolokia error status.
- **Live (isolated HOME, local broker):** browse a scratch queue,
  delete a message, purge, then check the `:log` view and the file:
  one line per request, the expected operations, no password. Repeat
  with the mq-proxy backend (`task dev:proxy:start`). A cancelled
  request is hard to catch against a fast local broker; the unit test
  covers it, and the live check tries Esc during a large browse.
