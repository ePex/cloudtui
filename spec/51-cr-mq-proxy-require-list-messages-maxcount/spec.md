# CR 51 — Require `maxCount` on mq-proxy's `list-messages`; `tui` supplies a default

Date: 2026-08-14

## What

`mq-proxy`'s `GET /api/management/command/list-messages` now requires
`filter.maxCount` to be present and a positive integer — a request
without it (or with `maxCount <= 0`) is rejected with 400, instead of
browsing the whole queue unbounded. `tui`'s message browser always sends
a `maxCount`: if the user hasn't set one via the filter form, a
client-side default (500) is applied automatically, for both the proxy
and Jolokia backends.

`delete-messages`/`move-messages` are **not** changed — their
`maxCount` stays optional (unset = today's intentional purge/move-all
behavior). The user is handling bringing the reference API in line with
this decision separately.

## Why

`list-messages`/browsing is the case that bites by accident: a user just
opens a queue with a large backlog (discussed scenario: ~100k messages)
and, since `maxCount` was optional (CR 45), `mq-proxy` browses and
returns every message, and `tui` fetches and renders all of it. Nothing
about opening a queue signals "this could be unbounded" the way an
explicit bulk delete/move does, so this is the endpoint worth protecting
by making the cap non-optional at the API level rather than trusting
every caller to remember to set one.

`delete-messages`/`move-messages` are deliberately left alone: unset
`maxCount` there is today's tested purge-queue/move-all behavior (see
`spec/50-fe-proxy-api-verify-script`), not an accident someone stumbles
into by browsing. Requiring a filter there would force some "unlimited"
sentinel back in through the side door, which doesn't remove the
underlying risk — it just makes it a required parameter to route around.
Per CR 44, `mq-proxy` is the reference implementation going forward, so
this repo isn't required to mirror whatever the live reference API
currently requires on those two endpoints; the user is adjusting that
service separately and this CR doesn't need to wait on it.

**Discussed and confirmed live** (this conversation): scope is
`list-messages` only; TUI-side default is 500.

## Scope

Changes to `mq-proxy` (Kotlin/Spring Boot):

1. `QueueController.listMessages()`: validate `query.filter.maxCount` is
   non-null and `> 0`; throw `IllegalArgumentException` otherwise. The
   shared `QueueMessageFilter.maxCount: Int?` type itself stays nullable
   (still used, unvalidated, by `DeleteMessagesRequest`/
   `MoveMessagesRequest`) — the requiredness check lives only in the
   `list-messages` handler, not the DTO.
2. `GlobalExceptionHandler.kt`: add an `IllegalArgumentException` → 400
   handler (mirroring the existing `BindException` → 400 handler added
   in CR 49), returning the exception's message as the error body.
3. `QueueControllerTest.kt`: update the existing `listMessages` tests
   (`returns 200 with message list`, `passes jmsType and messageId
   filters through`) to include `filter.maxCount`; add tests for missing
   `maxCount` and `maxCount=0`/negative both returning 400.
4. `mq-proxy/openapi.yaml` regenerated (`task openapi:proxy`).
5. `requests.http`: update `list-messages` examples to include
   `filter.maxCount`.

Changes to `tui` (Go):

6. `tui/internal/app/messages.go`: add a `defaultBrowseMaxCount = 500`
   constant and a small helper that returns a copy of a
   `queue.MessageFilter` with `MaxCount` set to the default when it's
   `<= 0`. Applied in two places:
   - `messagesView.load()`, so the filter actually sent to
     `queue.Backend.BrowseMessages` always carries a `maxCount` —
     satisfying `mq-proxy`'s new requirement, and also capping the
     Jolokia backend's client-side-filtered browse (JMX has no
     request-time cap, so this bounds what gets rendered/held in memory
     there too, even though it doesn't reduce what's fetched from the
     broker).
   - `updateTitle()`/`describeMessageFilter()`, so the table title
     always shows the effective `max=N` being applied, even when the
     user never touched the filter form — avoids messages silently
     going missing from the view with no visible explanation.
   `mv.filter` itself is left unmodified by this (still `MaxCount: 0`
   when the user hasn't set one) — only the two use sites above see the
   defaulted value. The filter form (`showMessageFilter()`) keeps
   pre-filling the Max Count field from the raw `mv.filter.MaxCount`
   (blank when unset), unchanged.
7. `messages_test.go`: unit tests for the new default-filter helper
   (zero/negative → default; positive → unchanged) and for `load()`
   sending the defaulted filter to `BrowseMessages`.

## Out of scope

- **No change to `delete-messages`/`move-messages`** on either
  `mq-proxy` or `tui` — see Why.
- **No change to the reference API** — the user is handling that
  separately.
- **No pagination / "load more."** Same as FE 46: `maxCount` caps what's
  fetched: there's still no follow-up affordance to fetch the next
  batch. A queue with more messages than the cap still just shows the
  first `maxCount` (oldest-first, per `BrokerService.browseMessages`'
  existing ordering) with no in-UI indication that more exist beyond the
  title's `max=N` — a discoverable "there may be more" affordance is a
  candidate for a future `fe`, not this CR.
- **No configurability of the TUI default** (e.g. a settings/config
  value for 500) — a fixed constant is enough for now; making it
  user-configurable is a candidate for a future `fe` if 500 turns out to
  be wrong for some workflows.
- **Jolokia backend's underlying JMX browse call** is unchanged — it
  still fetches every message from the broker before filtering
  client-side (per FE 46); this CR only bounds what's kept/rendered
  afterward, not the JMX round-trip itself.
