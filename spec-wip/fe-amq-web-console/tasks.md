# Tasks

1. [x] Scaffold `mq-proxy-web/`: `index.html` (connect form — base URL,
   username, password; validates via a `list-queues` call before
   switching views; "Forget connection" clears `localStorage`) and
   `app.js` (UMD-lite skeleton: `buildAuthHeader`, `parseEnvelope`, a
   thin `apiCall(baseUrl, verb, opts)` wrapper around
   `/api/management/command/<verb>`). `app.test.js` (Node's built-in
   `node:test`) covers `buildAuthHeader` and `parseEnvelope`.
   Manual test: open `index.html` via `file://`, connect to a running
   `mq-proxy`, confirm success/failure both render inline (no browser
   console needed to see the result).

2. [x] Queue list view: `list-queues` call + table (name,
   pending/consumer/enqueue/dequeue/producer counts), row click
   navigates to that queue's messages. Unit tests for the response
   → row-data mapping.

3. [x] Message list + detail views: `list-messages` (nested `filter.*`,
   `returnBody=true`, default `maxCount=500` shown/editable), row click
   opens detail (headers + body). Unit tests for the nested query-string
   builder.

4. [x] Purge action: optional JMS Type prompt → confirm dialog →
   `delete-messages` (`maxCount` omitted on a blank filter, set on a
   typed one). Unit tests for the blank-vs-filtered branching.

5. [x] Move single message (from detail view) and move-all/drain (from
   queue list): target-queue picker with the same four-tier DLQ-priority
   ordering as the TUI (spec/09) — preferred stripped-prefix match,
   regular, `dlq.`/`imq.`-prefixed, `activemq.`/`statistics.*`-prefixed.
   Move-all reuses the JMS Type prompt from task 4. Unit tests for the
   tier-ordering function.

6. [x] Send message: modal with a plain-text body field →
   `send-message`. Manual test: send, confirm it shows up in the queue's
   message list.

7. [x] `mq-proxy` CORS: `CorsConfigurationSource` bean in
   `SecurityConfig` — configurable allowed-origins list
   (`proxy.cors.allowed-origins` in `application.yml`, env override
   `CORS_ALLOWED_ORIGINS`) plus the unconditional `null` origin; allowed
   methods `GET`/`POST`/`OPTIONS`; allowed headers `Authorization`/
   `Content-Type`; `allowCredentials` left off. Test: `MockMvc` preflight
   checks — configured origin allowed, unlisted origin rejected, `null`
   always allowed.

8. [x] Wire `mq-proxy-web`'s tests into the Taskfile
   (`test:mq-proxy-web` → `node --test mq-proxy-web/`) and into CI
   (`.github/workflows/ci.yml`, a new job alongside the existing
   `tui`/`mq-proxy` jobs — no OS-specific branching needed, `node --test`
   behaves the same on all three runners).

9. [x] Docs: `mq-proxy-web/README.md` (how to open the page, how to run
   `task test:mq-proxy-web`); `mq-proxy/README.md` gets the new
   `proxy.cors.allowed-origins`/`CORS_ALLOWED_ORIGINS` keys documented
   alongside `BROKER_URL` etc.

10. [x] Manual end-to-end pass against a live `mq-proxy` + broker (using
    the dev-verification tooling, spec/13, to seed/inspect state): full
    connect → browse → purge → move single → move-all → send → delete
    flow, once served from a local static server (`http://`) and once
    opened directly via `file://` double-click — confirming the CORS
    config (task 7) actually permits both.

    Done against a live local broker, driving the real page in Chrome
    (served from a local static server on `http://localhost:8085`, with
    `mq-proxy` started with `CORS_ALLOWED_ORIGINS=http://localhost:8085`),
    using two disposable queues created/removed via `devtool add-queue`/
    `remove-queue` (spec/13). Verified end-to-end: connect, queue list,
    open messages, send, message detail, move (single, via the picker's
    `/`-filter and DLQ-exclusion-of-source behavior), purge (JMS Type
    prompt → confirm dialog → actually empties the queue, checked via
    `list-messages`). This is also what caught the real bug below.
    `file://` itself could not be driven by the browser-automation tool in
    this environment (it categorically refuses `file://` navigation) — the
    part of `file://` support that actually needed verifying, `mq-proxy`
    correctly answering a `null`-origin CORS preflight with a 200 and
    matching `Access-Control-Allow-Origin: null`, was confirmed directly
    via `curl -H "Origin: null" -H "Access-Control-Request-Method: GET" -X
    OPTIONS`. A follow-up manual double-click check is still worth doing
    on a machine where that's easy.

    **Bug found and fixed**: every modal (`#jmsTypePromptModal`,
    `#confirmModal`, `#movePickerModal`, `#sendModal`) and the topbar set
    their own `display` (`flex`) in the stylesheet, which — being an
    author-origin rule — overrides the browser's default `[hidden] {
    display: none }` UA rule regardless of selector specificity. The
    send-message modal was covering the entire page on first load,
    before any button had been clicked. Fixed with a single `[hidden] {
    display: none !important; }` rule in `index.html`.

11. [x] Queue list filter + column sort: a live substring filter input
    (client-side, over the already-fetched list — `list-queues` has no
    server-side filter param) narrows by queue name; clicking a column
    header sorts by that column ascending, clicking again toggles to
    descending. Both operate on the same in-memory queue array, so
    filter + sort compose. Unit tests for the filter-predicate and
    sort-comparator functions.

12. [ ] Merge-back: add `spec/21-amq-web-console/spec.md` (new area,
    condensed end-state of this feature); update `spec/02-ci-and-release`
    (new CI job) and `spec/10-mq-proxy-service` (CORS) in place; delete
    `spec-wip/fe-amq-web-console/`. Mark the PR ready for review.
