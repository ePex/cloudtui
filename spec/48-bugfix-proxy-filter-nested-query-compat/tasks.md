# Tasks — Bugfix 48

1. [x] `browseQuery` sends each set filter field under both its flat name
   and `filter.<name>`. Update `TestBrowseMessagesFilterQuery` to assert
   both forms are present.

## Manual verification

- Against `local-fibu-proxy` (reference API): filter form Max Count (and
  JMS Type, if a real type is available) actually narrows results.
- Against `local-mq-proxy`: filter form still works as it did after
  FE 46 (regression check).

**Verified 2026-08-13.** Against `local-fibu-proxy` (the reference API):
opened `esprit.businesspartner.fiproxy.fibu` (10 messages), applied the
filter form with Max Count=1 — title showed `(filter: max=1)` and
exactly 1 message was returned (previously all 10). Against
`local-mq-proxy`: sent two messages with different JMS types directly,
confirmed `jmsType` + `filter.jmsType` sent together still filters
correctly (1/2 matched) — no regression, mq-proxy ignores the extra
nested param as expected. Cleaned up test messages.
