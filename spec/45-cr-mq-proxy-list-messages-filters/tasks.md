# Tasks — CR 45

1. [x] Extend `QueueMessageFilter` handling in `BrokerService.browseMessages()`:
   add a `maxCount` cap on the browse loop (mirroring
   `deleteMessages`/`moveMessages`) and a `returnBody: Boolean = true`
   parameter threaded through to `toSummary()`, which nulls out `body`
   when false. Add/update unit tests in `BrokerServiceTest.kt` covering:
   `maxCount` caps the number of browsed messages, `returnBody = false`
   omits the body, default behavior (no params) is unchanged.
2. [x] Wire `fromDate`, `toDate`, `maxCount`, `returnBody` as optional
   query params on `QueueController.listMessages()`, building the full
   `QueueMessageFilter` and passing `returnBody` through to
   `browseMessages()`.
3. [x] Regenerate `mq-proxy/openapi.yaml` via `task openapi:proxy` and
   add example requests for the new filters to `mq-proxy/requests.http`.

## Manual verification

Since this only touches `mq-proxy`'s Kotlin/JMS layer (no `tui` changes),
unit tests cover the logic, but confirm end-to-end against a real broker
before checking off task 3:

- Start `mq-proxy` against a broker with a queue holding messages spread
  across a date range and more than one `jmsType`.
- `GET /list-messages?sourceQueue=<q>&maxCount=1` returns exactly one
  message.
- `GET /list-messages?sourceQueue=<q>&fromDate=...&toDate=...` returns
  only messages within that window.
- `GET /list-messages?sourceQueue=<q>&returnBody=false` returns entries
  with `body: null`; omitting the param (or `returnBody=true`) still
  returns bodies as before.

**Verified 2026-08-13** against the local broker (`task dev:proxy:start`,
with `JAVA_HOME` overridden to a JDK 21 install — the sdkman `current`
default on this machine is JDK 17, which mq-proxy's toolchain rejects;
this is a local environment quirk, not a repo issue). Seeded a disposable
`cr45-verify` queue with 5 messages via `task test:queue:add`/
`task seed:queue` (temporarily on the `default` jolokia connection, since
`add-queue`/`seedqueue` require a jolokia backend), then hit
`local-mq-proxy`'s `list-messages` directly with curl:
- unfiltered: 5 results, bodies present.
- `maxCount=2`: exactly 2 results.
- `returnBody=false`: all 5 results, every `body` is `null`.
- `fromDate=2030-...` (future): 0 results. `toDate=2020-...` (past):
  0 results. `fromDate=2020-...` (past): all 5 results.
Cleaned up: stopped mq-proxy (`task dev:proxy:stop`), removed the test
queue (`task test:queue:remove`), restored `config.yaml`'s active
connection to what it was before.
