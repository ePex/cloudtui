# Plan — CR 49

## Approach

The design was already validated live before writing this plan (see
`spec.md`) — implementation is mostly transcribing the prototype into
the real files plus updating tests/docs that reference the old flat
shape.

1. **`QueueController.kt`**: add `data class ListMessagesQuery(val
   sourceQueue: String, val returnBody: Boolean? = null, val filter:
   QueueMessageFilter = QueueMessageFilter())` above the controller
   class; change `listMessages` to take a single
   `@ModelAttribute query: ListMessagesQuery` parameter and read
   `query.sourceQueue`/`query.filter`/`query.returnBody` in the call to
   `brokerService.browseMessages`.
2. **`GlobalExceptionHandler.kt`**: add
   `@ExceptionHandler(BindException::class) @ResponseStatus(BAD_REQUEST)`
   returning `ex.bindingResult.allErrors.firstOrNull()?.defaultMessage`,
   placed before the generic `Exception` handler (order matters for
   `@ExceptionHandler` specificity, though Spring resolves by most-specific
   type regardless of declaration order — kept above `handleGeneric` for
   readability, matching the file's existing most-specific-first layout
   with `JMSException`).
3. **`QueueControllerTest.kt`**: change
   `listMessages passes jmsType and messageId filters through`'s request
   URL from `&jmsType=...&messageId=...` to
   `&filter.jmsType=...&filter.messageId=...`. Add
   `listMessages returns 400 when sourceQueue is missing` (no mock setup
   needed — binding fails before the controller body runs).
4. **`openapi.yaml`**: `task openapi:proxy`.
5. **`requests.http`**: update the two filtered `list-messages` example
   requests (JMS-type filter, and CR 45's date-range/maxCount/returnBody
   example) to `filter.jmsType=...`, `filter.fromDate=...`, etc.
6. **`tui/internal/queue/proxy/proxy.go`**: `browseQuery`'s `set` helper
   currently does `q.Set(name, value); q.Set("filter."+name, value)` —
   drop the first line, keep only the `filter.`-prefixed one. Update the
   function doc comment (currently describes the dual-send rationale
   from bugfix 48) to describe the new nested-only shape and point at
   this CR instead.
7. **`proxy_test.go`**: `TestBrowseMessagesFilterQuery`'s `checkBoth`
   helper becomes a single nested-only assertion (rename accordingly);
   `TestBrowseMessages`'s zero-value-filter loop drops the flat-key
   check, keeping only the `filter.`-prefixed one.

## Files touched

- `mq-proxy/src/main/kotlin/com/github/epex/mqproxy/api/QueueController.kt`
- `mq-proxy/src/main/kotlin/com/github/epex/mqproxy/api/GlobalExceptionHandler.kt`
- `mq-proxy/src/test/kotlin/com/github/epex/mqproxy/api/QueueControllerTest.kt`
- `mq-proxy/openapi.yaml` (regenerated)
- `mq-proxy/requests.http`
- `tui/internal/queue/proxy/proxy.go`
- `tui/internal/queue/proxy/proxy_test.go`
- `spec/49-cr-mq-proxy-nested-filter-query/tasks.md` (next gate)

## Key decisions

- **Wrapper data class, not a bare `@ModelAttribute filter` param** —
  the only one of the two prototyped shapes that actually performs
  nested binding (see `spec.md`'s "Prototyped and confirmed live"
  section). This isn't a style preference, it's the only one that works.
- **Fix the 400-vs-500 regression in the same change**, not deferred —
  it's a direct, mechanical side effect of switching to constructor
  binding, not a separate concern, and shipping the nested-query change
  without it would be a real (if narrow) regression for any caller that
  omits `sourceQueue`.
- **Breaking change, no back-compat shim** — consistent with CR 44's
  precedent (`mq-proxy`'s API isn't versioned, `tui` is its only real
  consumer today, and the reference API this happens to also align with
  isn't a compatibility target we're promising against either).
- **`tui`'s client drops flat entirely, doesn't keep dual-send "just in
  case"** — once `mq-proxy` no longer accepts flat params, sending them
  is dead weight per this repo's "no dead code" rule, not
  defensive-in-depth.

## Manual verification

- Start `mq-proxy` (`task dev:proxy:start`). Confirm `filter.maxCount=1`
  and `filter.jmsType=<real-type>` each correctly narrow a multi-message
  queue via `curl`, and that a request with no `sourceQueue` returns 400
  (not 500).
- Drive the real `tui` (proxy backend) message filter form (`f`) against
  `mq-proxy` — confirm it still works end-to-end after the client-side
  simplification (regression check for FE 46/bugfix 48's functionality).
- Re-run the exact repro from bugfix 48 against the reference API
  (`local-other-proxy`): filter form Max Count=1 on a multi-message queue
  — confirm still correctly narrowed (the reference API only ever
  understood nested, so this should be unaffected by dropping the flat
  half, but worth confirming nothing else broke).
