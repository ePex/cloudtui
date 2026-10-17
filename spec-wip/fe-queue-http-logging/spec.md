# Feature: log every queue backend HTTP request

Date: 2026-10-17

## What and why

The log (`~/.cloudtui/cloudtui.log`, the `:log` view, spec/06) records
startup and a few failures, but nothing about what the app actually
asks the broker. When the queue list stays empty, a purge seems to do
nothing, or a load is slow, the log can't say which requests were sent,
which failed, or how long they took.

Every HTTP request the queue backends make gets one log line once it
completes: Jolokia (spec/07) and mq-proxy (spec/11), for every
operation (list, browse, scan, send, delete, move, purge, and their
retries).

## Desired behavior

- **One line per request**, written when the response arrives (or the
  request fails), at `INFO`, with:
  - the backend (`jolokia` or `proxy`) and the connection's name,
  - the HTTP method and URL, with any credentials in the URL removed,
  - the **operation**:
    - Jolokia: the request type and what it targets, e.g.
      `exec removeMessage` on the queue's MBean, or `read QueueSize`.
      A bulk request (several operations in one HTTP request, as the
      queue list sends) is summarized: how many operations, and which
      kinds (e.g. `bulk: 48 × read QueueSize/ConsumerCount/EnqueueCount/
      DequeueCount`). Operation arguments (message IDs, message bodies,
      headers) are not logged.
    - mq-proxy: the operation is the URL path (e.g.
      `/api/management/command/list-messages`); its query string (queue
      name, filter) is part of the URL.
  - the HTTP status, and for Jolokia the status inside its JSON
    response too (Jolokia answers HTTP 200 even when the operation
    failed),
  - how long it took and the response size in bytes.
- **Failures are louder.** A request that fails (no response, a
  non-2xx HTTP status, or a Jolokia status other than 200) is logged at
  `WARN` with the error, instead of `INFO`. A request cancelled by the
  user (Esc, spec/24) is logged at `INFO` as cancelled.
- **Never logged:** request or response bodies, headers (including the
  `Authorization` header with the broker password), and passwords in
  any form. The log stays safe to attach to a ticket.
- Unchanged: the log file's location, truncation on startup, the
  `:log` view, and everything already logged.

## Out of scope

- Datadog and AWS SDK requests (possible follow-ups).
- A log-level setting, or switching this logging off.
- Logging bodies or headers, even optionally.
- Log rotation or a size limit beyond the existing truncate-on-startup
  (a long session with many requests grows the file until the next
  start).
