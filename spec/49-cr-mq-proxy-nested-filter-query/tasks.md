# Tasks — CR 49

1. [x] `mq-proxy`: `ListMessagesQuery` + `@ModelAttribute`-bound
   `listMessages` (`QueueController.kt`), `BindException → 400` handler
   (`GlobalExceptionHandler.kt`), updated/new tests in
   `QueueControllerTest.kt`.
2. [x] `mq-proxy`: regenerate `openapi.yaml`, update `requests.http`'s
   filtered `list-messages` examples to nested query params.
3. [x] `tui`: `browseQuery` (`proxy.go`) sends only nested `filter.*`
   params (drop the flat half of bugfix 48's dual-send); update
   `proxy_test.go` accordingly.

## Manual verification

- `mq-proxy` via `curl`: `filter.maxCount=1` and `filter.jmsType=...`
  each correctly narrow results; missing `sourceQueue` → 400, not 500.
- Real `tui` against `local-mq-proxy`: filter form still works
  end-to-end (regression check).
- Real `tui` against `local-other-proxy` (reference API): re-run bugfix
  48's exact repro (Max Count=1 on a multi-message queue) — still works.

**Verified 2026-08-13**, using an isolated second `mq-proxy` instance
(`SERVER_PORT=8090`) so the user's own running instance wasn't disturbed
(nor its port 8080, which `openApi.apiDocsUrl` in `build.gradle.kts` is
hardcoded to — had to temporarily point it at 8090 to regenerate
`openapi.yaml` against the right server, then revert; hand-fixed the
one `servers: url` line the temporary port leaked into the generated
file).

- `curl` against the isolated instance: `filter.maxCount=1` and
  `filter.jmsType=<nonexistent>` on `sample.department.proxy.demo`
  (10 messages) correctly returned 1 and 0 respectively; a request with
  no `sourceQueue` returned 400 with a real message, not a raw 500.
- Real `tui` (temporary connection at :8090): opened the same queue (10
  messages), applied the filter form with Max Count=1 — title showed
  `(filter: max=1)`, exactly 1 message shown.
- Real `tui` against `local-other-proxy` (the actual reference API):
  same queue, same Max Count=1 filter-form check — exactly 1 message
  shown, confirming the client-side simplification (dropping bugfix
  48's flat half) didn't regress the reference-API case it was
  originally added for.
- Cleanup: temporary connection removed, `config.yaml` restored,
  isolated instance killed, tmux sessions killed, scratch files removed.
