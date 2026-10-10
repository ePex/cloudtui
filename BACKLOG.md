# Backlog

Ideas and follow-ups that are worth doing eventually but aren't queued up
as active work. Not a substitute for `spec-wip/` — when one of these
gets picked up, it still goes through the normal spec → plan → tasks
gate in `CLAUDE.md`.

- **Search message headers and full bodies.** The message browser's
  quick search (`/`, spec/08) only matches the JMS Type and preview text
  of the rows already loaded, and its filter (`f`) covers JMS Type and
  time range. Matching on header/property values or anywhere in the
  full body would help on large queues.

- **Connection health at a glance.** Home shows queues per active
  connection, but switching to a named connection (spec/12) that's
  unreachable is a fail-and-wait; a quick reachability indicator next
  to each connection would surface that up front.

- **Saved Datadog queries.** Parameter Store, Secrets Manager and
  CloudWatch log groups have favorites (starred entries), but the
  Datadog Logs view starts from an empty query every session. Saving
  frequently used queries would save retyping them.

- **Export search results to a file.** CloudWatch/Datadog Logs results
  have no "save to file" action, which would help when pasting results
  into an incident doc. (Queue messages got one: spec/23.)

- **Export and remove ("drain to file").** The message export
  (spec/23) only browses, so on a queue bigger than the
  broker's browse limit (~400 by default) it can't contain everything.
  A destructive variant would consume the messages in a transaction,
  write them to the file, and commit only once the file is complete —
  e.g. to archive and clear a DLQ. Needs a new `mq-proxy` endpoint (and
  the same in other servers speaking that API) plus a purge-style
  confirm.

- **Import an export.** Re-send the messages from an NDJSON export
  (spec/23) into a queue — e.g. to move them to another
  environment — keeping JMS type, correlation ID and headers. Could
  also go the other way: write an export as snippet files (spec/22) so
  single messages can be re-sent via "Load snippet…".

- **Demo GIFs for the AWS (and Datadog) features.** The README demos
  (`docs/demo/`) only cover ActiveMQ, because recording against real
  AWS accounts would show real profile, account, parameter, secret, log
  group and pipeline names.
  - **Parameter Store, Secrets Manager, CloudWatch Logs:** should work
    without code changes. Run a local AWS emulator (e.g. `ministack`, a
    LocalStack drop-in) next to the example broker, seed it with made-up
    data, and give the demo `HOME` an `~/.aws/config` profile called
    `example` with `endpoint_url = http://localhost:4566` and dummy
    credentials (the app builds its AWS clients with the SDK's standard
    config loading, which honors `endpoint_url`).
  - **CodePipeline:** only if the emulator supports it (LocalStack's
    free edition doesn't).
  - **SSO re-login:** can't be shown with an emulator.
  - **Datadog:** needs a small feature first. The API address is
    hard-coded as `https://api.<site>`, so there's no way to point it at
    a local mock server without an override (e.g. an env var for the
    API URL).
