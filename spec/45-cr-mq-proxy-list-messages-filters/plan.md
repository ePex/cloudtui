# Plan — CR 45

## Approach

Purely additive, server-side change to `mq-proxy`. No new abstractions:
reuse the existing `QueueMessageFilter` type and its `toSelector()`
already shared by `deleteMessages`/`moveMessages` — `browseMessages`
just needs to receive a fully-populated filter instead of a
partially-populated one, plus a `maxCount` cap and a `returnBody` switch
at the call site.

1. **`QueueController.listMessages()`**
   (`mq-proxy/src/main/kotlin/com/github/epex/mqproxy/api/QueueController.kt`):
   add `fromDate: String?`, `toDate: String?`, `maxCount: Int?`,
   `returnBody: Boolean?` as optional `@RequestParam`s, and build the
   `QueueMessageFilter` passed to `browseMessages` from all five fields
   instead of just `jmsType`/`messageId`.

2. **`BrokerService.browseMessages()`**
   (`mq-proxy/src/main/kotlin/com/github/epex/mqproxy/service/BrokerService.kt`):
   - Add a `maxCount` cap on the enumeration loop, mirroring the
     `while (filter.maxCount == null || <n>.size < filter.maxCount)`
     pattern in `deleteMessages`/`moveMessages` (lines 116, 136) — a
     `QueueBrowser`'s `enumeration` doesn't support `break` mid-iteration
     as cleanly as `receiveNoWait()`, so the loop changes from a
     `while (enum.hasMoreElements())` to also check the count.
   - Thread a `returnBody: Boolean` parameter through to `toSummary()`
     (line 171), which currently unconditionally sets
     `body = (this as? TextMessage)?.text`. When `returnBody` is false,
     `body` is `null` instead. Default `returnBody = true` on the
     `browseMessages`/`toSummary` signatures so every other existing
     caller (just the one call site in `browseMessages` itself, plus
     tests) keeps today's behavior unless the new query param says
     otherwise.

3. **`mq-proxy/openapi.yaml`**: regenerate via `task openapi:proxy`
   (springdoc-openapi-gradle-plugin, per CR 38) — no hand-editing.

4. **`mq-proxy/requests.http`**: add one or two example requests under
   the existing "List messages" section demonstrating `fromDate`/
   `toDate`/`maxCount`/`returnBody`, consistent with how CR 44 documented
   the filtered delete/move requests there.

## Files touched

- `mq-proxy/src/main/kotlin/com/github/epex/mqproxy/api/QueueController.kt`
- `mq-proxy/src/main/kotlin/com/github/epex/mqproxy/service/BrokerService.kt`
- `mq-proxy/src/test/kotlin/com/github/epex/mqproxy/service/BrokerServiceTest.kt`
  (new cases: `maxCount` caps `browseMessages`, `returnBody = false`
  omits body, existing filter/selector tests keep passing unchanged)
- `mq-proxy/openapi.yaml` (regenerated, not hand-edited)
- `mq-proxy/requests.http` (example requests)
- `spec/45-cr-mq-proxy-list-messages-filters/tasks.md` (next gate)

`tui` is untouched — confirmed out of scope in the spec.

## Key decisions

- **`returnBody` defaults to `true`** (i.e. omitting the param preserves
  current behavior) rather than defaulting to `false` like the reference
  API's doc implies. Flipping the *default* would silently change
  today's `list-messages` response shape for the only current consumer
  (`tui`, which always wants the body — see `tui/internal/queue/proxy/proxy.go`
  `BrowseMessages`) even though `tui` itself isn't being changed here.
  Making it opt-out rather than opt-in keeps this CR additive/
  non-breaking, matching the "breaking changes need their own CR"
  posture CR 44 set (it declared its breaking change explicitly; this
  one isn't breaking, so it shouldn't quietly become one).
- **No new Go types.** Since `tui` isn't calling these new params yet,
  there's nothing to add to `queue.MessageFilter` or the proxy client —
  adding unused fields there would be dead code by this repo's own
  "no dead code" rule.
- **`maxCount` on `browseMessages` reuses the exact wording/pattern**
  from `deleteMessages`/`moveMessages` rather than inventing a different
  idiom, since all three now share one filter type and one mental model.
