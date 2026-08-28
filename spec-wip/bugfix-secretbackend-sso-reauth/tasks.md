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

2. [ ] **Merge-back.** Update `spec/12-named-connections/spec.md`'s
   "Password resolution (AWS-Secrets-Manager-backed passwords)" section
   to document the SSO re-auth behavior (status message, browser login,
   retry — same mechanism as spec/36) as current, shipped behavior.
   Delete `spec-wip/bugfix-secretbackend-sso-reauth/`.
