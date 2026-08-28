# Tasks

1. [ ] **Wire SSO re-auth into SecretResolver.** Implement per `plan.md`:
   `SecretResolver`'s new `authTypeFor`/`login`/`onReauth` fields,
   `NewSecretResolver`'s new signature, `Resolve`'s `awsauth.WithReauth`
   wrapping via the local `revealResult` type. Update `internal/app`'s
   one call site. Add `newTestResolver` and update every existing
   `secretbackend` test to use it. Add the three new tests
   (`TestResolveTriggersReauthOnSSOExpiredError`,
   `TestResolveSurfacesErrorWhenReauthLoginFails`,
   `TestResolveDoesNotReauthForNonSSOProfile`). `go build`/`go vet`/
   `go test ./...` (whole module — this touches `internal/app` too)
   clean.

2. [ ] **Merge-back.** Update `spec/12-named-connections/spec.md`'s
   "Password resolution (AWS-Secrets-Manager-backed passwords)" section
   to document the SSO re-auth behavior (status message, browser login,
   retry — same mechanism as spec/36) as current, shipped behavior.
   Delete `spec-wip/bugfix-secretbackend-sso-reauth/`.
