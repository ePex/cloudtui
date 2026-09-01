# Backlog

Ideas and follow-ups that are worth doing eventually but aren't queued up
as active work. Not a substitute for `spec-wip/` — when one of these
gets picked up, it still goes through the normal spec → plan → tasks
gate in `CLAUDE.md`.

- **Loading indicators + SSO re-auth status for the remaining AWS
  views.** `QueuesView` has both: an immediate "Loading queues…"
  placeholder (bugfix-queues-loading-indicator) and, when an AWS-secret
  connection's SSO session expires mid-fetch, a message that switches to
  "AWS SSO session expired — opening browser to log in…" and back
  (bugfix-secretbackend-sso-reauth, via the `ui.ReauthStatusShower`
  interface). SSM Parameters, Secrets Manager, CloudWatch Logs, and
  CodePipeline (list + detail) have none of this — no loading
  placeholder at all today, and their existing `awsauth.WithReauth`
  calls just show the SSO message via their own `showStatus` until the
  final render/error overwrites it (a real but minor staleness window,
  never reported as confusing). Doing this properly means giving each
  of those views its own loading indicator first (mirroring
  `QueuesView.Load()`), then implementing `ui.ReauthStatusShower` on top
  — the interface doesn't help without something to revert *to*.

- **In-queue message search/filter.** The message browser (spec/08)
  lists every message on a queue with no way to narrow by body/header
  content; a grep-style filter would help on queues with thousands of
  messages, similar to the search CloudWatch/Datadog Logs already offer.

- **Bulk message actions.** Purge/move/send (spec/09) act on a single
  message or the whole queue; there's no multi-select for "requeue
  these 5 poison messages" without operating on the entire queue.

- **Connection health at a glance.** Home shows queues per active
  connection, but switching to a named connection (spec/12) that's
  unreachable is a fail-and-wait; a quick reachability indicator next
  to each connection would surface that up front.

- **Bookmarks/pins for AWS resources.** SSM Parameters, Secrets
  Manager, and CloudWatch/Datadog saved queries all start from a flat
  list each session — pinning frequently-checked entries would save
  re-finding them.

- **Export search results to a file.** CloudWatch/Datadog Logs results
  and the message browser have no "save to file" action, which would
  help when pasting results into an incident doc.
