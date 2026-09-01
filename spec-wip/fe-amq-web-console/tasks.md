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

8. [ ] Wire `mq-proxy-web`'s tests into the Taskfile
   (`test:mq-proxy-web` → `node --test mq-proxy-web/`) and into CI
   (`.github/workflows/ci.yml`, a new job alongside the existing
   `tui`/`mq-proxy` jobs — no OS-specific branching needed, `node --test`
   behaves the same on all three runners).

9. [ ] Docs: `mq-proxy-web/README.md` (how to open the page, how to run
   `task test:mq-proxy-web`); `mq-proxy/README.md` gets the new
   `proxy.cors.allowed-origins`/`CORS_ALLOWED_ORIGINS` keys documented
   alongside `BROKER_URL` etc.

10. [ ] Manual end-to-end pass against a live `mq-proxy` + broker (using
    the dev-verification tooling, spec/13, to seed/inspect state): full
    connect → browse → purge → move single → move-all → send → delete
    flow, once served from a local static server (`http://`) and once
    opened directly via `file://` double-click — confirming the CORS
    config (task 7) actually permits both.

11. [ ] Merge-back: add `spec/21-amq-web-console/spec.md` (new area,
    condensed end-state of this feature); update `spec/02-ci-and-release`
    (new CI job) and `spec/10-mq-proxy-service` (CORS) in place; delete
    `spec-wip/fe-amq-web-console/`. Mark the PR ready for review.
