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
