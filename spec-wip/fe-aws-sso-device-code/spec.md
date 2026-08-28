# Show the AWS SSO device verification code and URL during re-auth

Date: 2026-08-28

## What

When `awsauth.WithReauth` triggers a fresh `aws sso login` (an expired
SSO session detected mid-fetch), the AWS CLI's device-authorization flow
prints a **verification code** and a **verification URL** to stdout
right when it opens the browser — this is the exact code/URL the SSO
authorization page in the browser shows too, and matching them is the
whole point: it's how the user confirms the browser page that just
opened is legitimately theirs to approve, not a stale/hijacked/wrong
session (anti-phishing measure inherent to OAuth device-authorization
grant, RFC 8628). Today cloudtui runs `aws sso login` with its output
fully discarded until the command exits, so this code is never shown —
the user has no way to actually verify what they're approving, and no
way to recover if the wrong browser/profile opened (no visible URL to
copy into the right one).

Confirmed by reading the installed AWS CLI's own source
(`awscli/customizations/sso/utils.py`, `OpenBrowserHandler.__call__`),
not assumed — the real printed output (in order) is:

```
Attempting to automatically open the SSO authorization page in your default browser.
If the browser does not open or you wish to use a different device to authorize this request, open the following URL:

https://device.sso.<region>.amazonaws.com/

Then enter the code:

WDJB-MJHT
```

This fix: `awsauth.Login` streams `aws sso login`'s output live (instead
of buffering until exit) and parses out the URL and code as soon as both
appear, invoking a new callback with them. Every existing
`awsauth.WithReauth` call site's "AWS SSO session expired — opening
browser to log in…" message gets updated in place, once the code/URL
are known, to include both — the same message, now more useful, not a
new separate display.

## Why

Reported directly, as a natural follow-up to the secretbackend SSO
re-auth fix: showing *that* a re-auth is happening isn't enough for the
user to actually verify it safely, and if the wrong browser opens (a
real possibility depending on OS/profile default-browser settings), they
currently have no way to complete the login at all short of digging into
a terminal to run `aws sso login` by hand.

## Scope

- `awsauth.Login(ctx, profile, onCode)`: signature gains a
  `onCode func(code, url string)` parameter (nil-safe). Implementation
  switches from `exec.Command(...).CombinedOutput()` (buffers
  everything, returns only after the whole login completes) to a live
  `bufio.Scanner` over the subprocess's combined stdout/stderr, calling
  `onCode` exactly once, as soon as both the URL and code have been seen
  in the stream — well before the command itself exits (it blocks until
  the user finishes the browser flow or times out). Parses by anchoring
  on the literal substrings confirmed above ("open the following URL:",
  "Then enter the code:"), not a code-format regex — robust to a future
  AWS CLI version changing the code's exact character set. The full
  output is still captured and included in the wrapped error on failure,
  same as today.
- `awsauth.WithReauth`: gains an `onCode func(code, url string)`
  parameter, threaded straight through to `login`.
- `ui.Host`/`ui.ViewHost`'s `AWSSSOLogin` method signature updates to
  match `awsauth.Login`'s new shape; `internal/app`'s implementation and
  field wiring follow.
- `ui.ReauthStatusShower`'s `ShowReauthWaiting()` gains a `msg string`
  parameter (was 0-arg, hardcoded to a single fixed message) — so the
  same call can be reused to *update* the message once the code/URL
  arrive, not just set it once at the start. `ShowReauthDone()` is
  unchanged.
- Every existing `awsauth.WithReauth` call site updated to pass a real
  `onCode` callback that updates its own status message with the
  code/URL once available: `QueuesView` (via `secretbackend`),
  `SSMParamsView`, `SecretsView`, `LogsView` (CloudWatch Logs group
  list), `CodePipelineListView`, `CodePipelineDetailView`. Message shape
  (exact wording confirmed in `plan.md`): the existing "AWS SSO session
  expired — opening browser to log in…" line, extended to include the
  code and URL once known.
- `PipelineWatcher.pollPipeline` (the background pipeline-poll loop, the
  one existing call site that already deliberately passes `onReauth:
  nil` — "no in-progress status message — this isn't a visible search
  view") keeps passing `onCode: nil` too, for the same reason: no
  display mechanism exists there and this isn't the place to add one.

## Out of scope

- Any UI beyond updating the existing status line — no modal, no
  separate dedicated code-display widget, no QR code, no clipboard
  auto-copy of the code/URL.
- Showing `verificationUriComplete` (the code-prefilled autofill URL) —
  it's never printed by `aws sso login`'s default (browser-opening)
  mode, only by its `--no-browser` flag's `PrintOnlyHandler` path, which
  cloudtui doesn't use; getting it would require calling the SSO OIDC
  `StartDeviceAuthorization` API directly instead of shelling out to the
  CLI, a much larger change not justified here. The plain
  `verificationUri` (manual entry, same URL cloudtui already opens
  automatically) plus the code is what's shown.
- `PipelineWatcher`'s background poll loop gaining a status display —
  already out of scope by the existing design it inherits.
