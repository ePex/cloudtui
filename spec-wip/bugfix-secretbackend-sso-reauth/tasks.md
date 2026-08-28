# Tasks

1. [x] **Wire SSO re-auth into SecretResolver.** Implemented per
   `plan.md`: `SecretResolver`'s new `authTypeFor`/`login`/`onReauth`
   fields, `NewSecretResolver`'s new signature, `Resolve`'s
   `awsauth.WithReauth` wrapping via the local `revealResult` type.
   Updated `internal/app`'s one call site (`a.AWSAuthTypeFor`,
   `a.AWSSSOLogin`, `onReauth` posting to the bottom status bar via
   `QueueUpdateDraw`+`SetStatus`). Added `newTestResolver` and updated
   all 6 existing `secretbackend` tests to use it (non-SSO stub, so
   their behavior is unchanged — confirmed, all still pass). Added the
   three new tests proving the actual reauth wiring
   (`TestResolveTriggersReauthOnSSOExpiredError`,
   `TestResolveSurfacesErrorWhenReauthLoginFails`,
   `TestResolveDoesNotReauthForNonSSOProfile`) — all pass under
   `go test -race`. `go build`/`go vet`/`go test ./...` clean across the
   whole module (confirmed no import cycle: `secretbackend` importing
   `awsauth` compiles fine).
   **Addendum, found via live testing**: the status bar message never
   cleared, and the table's own "Loading queues…" placeholder ignored
   the reauth wait entirely. Fixed per `plan.md`'s addendum: added
   `onReauthDone func()` to `SecretResolver` (fires right after `login`,
   success or failure); added `ui.ReauthStatusShower`
   (`ShowReauthWaiting`/`ShowReauthDone`), a new optional interface
   mirroring `Themeable`/`Shortcuttable`, dispatched via
   `a.activeView()`; `QueuesView` implements it, reusing a new
   `loadingQueuesStatus` const so `Load()` and `ShowReauthDone()` can't
   drift out of sync; `app.go`'s `onReauthDone` closure also clears the
   status bar back to `""`. Added `TestQueuesViewShowReauthWaitingThenDone`
   and extended `TestResolveSurfacesErrorWhenReauthLoginFails` to assert
   `onReauthDone` fires even when `login` fails. All tests pass;
   `go build`/`go vet`/`go test ./...` still clean.
   **Second live-testing round**: showing the message in both the
   status bar *and* the active view's own display at once was
   redundant. Extracted `app.go`'s inline closures into named
   `showReauthWaiting`/`showReauthDone` methods (directly testable
   without touching the real `QueueUpdateDraw`) and made the status-bar
   fallback mutually exclusive with the view-level display, per
   `plan.md`. Added 4 new `internal/app` tests confirming both branches
   (queues view handles it / home view falls back to the status bar).
   All tests pass; `go build`/`go vet`/`go test ./...` still clean.

2. [ ] **Merge-back.** Update `spec/12-named-connections/spec.md`'s
   "Password resolution (AWS-Secrets-Manager-backed passwords)" section
   to document the SSO re-auth behavior (status message, browser login,
   retry — same mechanism as spec/36) as current, shipped behavior.
   Delete `spec-wip/bugfix-secretbackend-sso-reauth/`.
