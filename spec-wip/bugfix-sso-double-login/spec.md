# Bugfix: AWS SSO login opens the browser twice

Date: 2026-10-17

## What's broken

Loading the queue list of a connection whose password comes from AWS
Secrets Manager, with the AWS SSO session expired, sometimes opens the
browser for the SSO login twice, and the queues then don't load.

Reproduced with a fake `aws` command (spec/14's "AWS SSO session
expired" flow, a profile with an expired cached token):

1. Open the queue list. cloudtui starts `aws sso login` (browser 1)
   and shows "Loading queues… AWS SSO session expired — opening browser
   to log in… Esc to cancel" while it waits.
2. Press Esc while the login is still waiting (or leave and reload).
   The operation's cancellation kills the running `aws sso login`, but
   its browser tab stays open.
3. Reload (`r`, or open the queue list again). A second `aws sso login`
   starts: browser 2.

With AWS CLI v2's current login flow, the browser page reports back to
the CLI process that opened it. Approving in tab 1 reports to a process
that no longer exists, so nothing gets logged in, and the queue load
waits on tab 2.

More generally, nothing makes callers share a login: the queue
connection's password, the AWS views (Parameter Store, Secrets Manager,
CloudWatch Logs) and each watched pipeline each start their own
`aws sso login` when they hit an expired session. Two of them at the
same time also open two browser windows.

## Desired behavior

- **One login per profile at a time.** While `aws sso login` runs for a
  profile, anything else needing a login for that profile waits for
  that same login instead of starting another one. It shows the same
  waiting message (with the verification code and URL, if the CLI
  printed them, spec/14) and, once the login finishes, retries its own
  call once, as today. A login that failed is not retried by the callers
  that waited on it; each reports its error as today.
- **Cancelling stops waiting, not the login.** Esc on an operation that
  is waiting for a login (e.g. "Loading queues") cancels that operation
  as today, but `aws sso login` keeps running, so approving the open
  browser tab still completes the login. A load started while it still
  runs waits for it (above); a load started after it succeeded just
  works, with no new login. Quitting cloudtui ends a login that is still
  running, so no `aws sso login` is left behind.
- **A missing SSO token file counts as an expired session.** For an SSO
  profile that has never logged in (or whose token cache was cleared),
  the SDK reports that the cached token file can't be read. cloudtui
  currently shows that as a plain error; it gets the same login-and-retry
  as an expired token. Non-SSO profiles are unchanged.
- Unchanged: when a login is needed at all (spec/14's `NeedsReauth`,
  extended above), the messages shown, the single retry after a login,
  and every non-SSO error.

## Out of scope

- Showing the browser URL for AWS CLI v2's newer login flow, which
  prints no device code (the status message then shows no code, as
  today).
- A way to abort a running `aws sso login` from within cloudtui other
  than quitting.
- Logins for different profiles: they still run independently, since
  each opens its own SSO session.
