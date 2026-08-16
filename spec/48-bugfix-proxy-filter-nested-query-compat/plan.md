# Plan — Bugfix 48

## Approach

`browseQuery` (`tui/internal/queue/proxy/proxy.go`) already builds a
`url.Values` from `toFilterDTO(filter)`'s five optional fields. Add the
`filter.`-prefixed duplicate of each `q.Set(...)` call right next to the
existing flat one, so every filter field is sent twice under two names.
No new types, no backend-detection branching — the dual-send approach
confirmed safe against both APIs in `spec.md`.

## Files touched

- `tui/internal/queue/proxy/proxy.go` (`browseQuery`)
- `tui/internal/queue/proxy/proxy_test.go` — extend
  `TestBrowseMessagesFilterQuery` to assert both the flat and
  `filter.`-prefixed param are present for each field.
- `spec/48-bugfix-proxy-filter-nested-query-compat/tasks.md` (next gate)

## Key decisions

- **Duplicate every filter field, not just `maxCount`.** The user's
  report was about `maxCount` specifically, but `jmsType`/`messageId`/
  `fromDate`/`toDate` have the exact same nested-vs-flat mismatch against
  the reference API — fixing only the one field reported would leave the
  same bug latent for the others.
- **No conditional/backend-aware logic.** Confirmed both APIs silently
  ignore the param shape they don't recognize, so unconditionally
  sending both is simpler and more robust than trying to detect which
  backend is on the other end (which `browseQuery` has no reliable way
  to do anyway — it only knows a base URL).

## Manual verification

- Against `local-other-proxy` (reference API): open a queue with several
  messages, set Max Count in the filter form, Apply — confirm the result
  is actually capped, not all messages. Repeat for JMS Type if a real
  (non-`text`) JMS type is available on a queue there.
- Against `local-mq-proxy`: repeat the same filter-form check — confirm
  no regression (still filters correctly, per FE 46's original
  verification).
