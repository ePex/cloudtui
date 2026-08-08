# Tasks — FE 28: read AWS connection profiles from `~/.aws`

Plan: [plan.md](plan.md)

1. [x] Add `github.com/aws/aws-sdk-go-v2/config` to `tui/go.mod` (`go get`
   + `go mod tidy`); confirm `go build ./...` still passes with no code
   using it yet, so the dependency-only change is isolated and reviewable
   on its own.
2. [x] `tui/internal/awsprofile/awsprofile.go`: `AuthType`/`Profile`
   types and the file-path resolution helper (env var override, else SDK
   defaults) — no discovery logic yet, just the shared plumbing both the
   section-scanner and `List()` will use.
3. [x] Hand-rolled section-name scanner (`tui/internal/awsprofile/scan.go`)
   — the piece the SDK doesn't provide.
4. [x] `List()` (`tui/internal/awsprofile/list.go`): wire the scanner's
   names through `config.LoadSharedConfigProfile` and the classification
   order from plan.md; skip (log, don't fail) a profile that errors.
5. [x] `awsprofile/list_test.go`: all fixture cases from plan.md
   (static-keys, SSO, assume-role, credential-process, the mixed
   SSO+credential-process case — verified against this machine's real
   profiles, which actually have this shape — one-file-only, both-files
   dedup, no-files-at-all, sorted output).
6. [x] `App.listAWSProfiles` field (defaulting to `awsprofile.List`).
7. [x] Settings UI: new "AWS Profiles" list item + read-only table overlay
   (`tui/internal/app/awsprofiles.go`; NAME/REGION/AUTH columns), `r` to
   refresh, `Esc` to close, themed via `theme.go`. **Deviation from the
   task wording**: the settings-list label is a static "AWS Profiles",
   not "AWS Profiles: N found" — a live count would mean re-reading
   `~/.aws` on every settings-list refresh (which also fires on unrelated
   theme/connection changes), for a count that's shown inside the overlay
   itself anyway (title becomes "AWS Profiles (N)" once opened).
8. [x] `app`/`settings` tests (`awsprofiles_test.go`, updated
   `settings_test.go`): item present, overlay populates from an injected
   fake, empty-region rendering, error rendering, `r` re-invokes the
   lister, `Esc` closes.
9. [x] `go build ./...`, `go vet ./...`, `go test ./...` — all pass.
10. [x] Manual verification per `verify-live`: built the real binary,
    drove it via tmux, opened Settings → AWS Profiles against this
    machine's real `~/.aws/config` — **69 real profiles** discovered and
    rendered correctly (mix of `sso` and `credential-process` auth types,
    matching the real config's mixed-auth pattern), `r` refresh and `Esc`
    close both confirmed working. Only profile names/regions/auth-type
    labels appeared in any output — no SSO URLs, account IDs, role names,
    or credential_process commands were shown or pasted anywhere.

## Also updated (cross-cutting, per "spec sync after every code change")

- `tui/CLAUDE.md`: package layout (`internal/awsprofile`) and Dependencies
  section (the new `aws-sdk-go-v2/config` dependency, pointing at this
  spec's `plan.md` for justification).
- `spec/22-fe-connections/spec.md`: noted the Settings list now has a
  third item, added later by this spec (that spec's own description only
  covered the two it introduced).
