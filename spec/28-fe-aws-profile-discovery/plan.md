# Plan — FE 28: read AWS connection profiles from `~/.aws`

## Dependency footprint (verified, not estimated)

I temporarily `go get`-ed `github.com/aws/aws-sdk-go-v2/config` to check its
actual public API and transitive cost before committing to this plan (then
reverted — nothing is added to `go.mod` yet). It pulls in 13 modules:
`aws-sdk-go-v2` (core), `smithy-go`, `aws-sdk-go-v2/credentials`,
`feature/ec2/imds`, `internal/configsources`, `internal/endpoints/v2`,
`internal/v4a`, `service/{sso,ssooidc,sts,signin}`,
`service/internal/{accept-encoding,presigned-url}`. That's a real, visible
jump from this repo's current dependency footprint (`tview`/`tcell` only)
— flagging concretely, since "use the SDK" was agreed in the abstract but
this is the actual bill. It's the direct consequence of the decision
already made, not a new question — proceeding on that basis.

## A real API gap this plan works around

`aws-sdk-go-v2/config`'s public API has **no function to list all profile
names** — only `LoadSharedConfigProfile(ctx, name, ...)`, which requires
already knowing the name. The only place profile-name enumeration exists
is `aws-sdk-go-v2/internal/ini`, which — being `internal` — cloudtui
cannot import at all (Go's internal-import rule; different module tree).

So this is a hybrid, not a pure SDK solution:

1. **Hand-rolled, minimal section-header scan** of both files, just to get
   the set of names — genuinely simple and low-risk (`^\[(profile )?(.+)\]$`
   line matching; AWS's own format for this hasn't changed in years and is
   this project's actual textual concern, not the parts that are easy to
   get wrong).
2. **`config.LoadSharedConfigProfile`** for each discovered name — this is
   where the real complexity (`source_profile` chains, `sso-session`
   blocks, `[profile x]` vs `[x]` validation, credential precedence) lives,
   and is exactly what's not worth reimplementing.

Both steps must look at the *same* files, so file-path resolution
(`AWS_CONFIG_FILE`/`AWS_SHARED_CREDENTIALS_FILE` env overrides, else
`config.DefaultSharedConfig{Filename,CredentialsFilename}()`) is done once
and shared — verified from the SDK's own source
(`shared_config.go:655`) that `LoadSharedConfigProfile` does **not** check
those env vars itself when called directly (only the higher-level
`LoadDefaultConfig` does) — so this plan reads them explicitly and passes
the resolved paths to `LoadSharedConfigProfile` via
`WithSharedConfigFiles`/`WithSharedCredentialsFiles`, rather than
duplicating AWS's env-var-precedence logic in application code.

## `tui/internal/awsprofile/awsprofile.go`

```go
type AuthType string

const (
    AuthStaticKeys        AuthType = "static-keys"
    AuthSSO               AuthType = "sso"
    AuthAssumeRole        AuthType = "assume-role"
    AuthCredentialProcess AuthType = "credential-process"
    AuthUnknown           AuthType = "unknown"
)

type Profile struct {
    Name     string
    Region   string
    AuthType AuthType
}

// List returns every profile discoverable in the shared AWS config and
// credentials files. Read-only: no credentials are resolved, refreshed, or
// validated, and no network calls are made. A profile that fails to parse
// is skipped (logged), not fatal to the rest of the list — this is a
// best-effort discovery aid, not a strict validator.
func List(ctx context.Context) ([]Profile, error)
```

Classification order (a profile can have fields for more than one method
at once — e.g. this machine's real profiles have both `sso_start_url` and
`credential_process` set, which matches a common pattern where an internal
tool wraps SSO login behind a `credential_process` script; the SDK's own
credential provider chain resolves `credential_process` first when present,
so classification follows the same precedence rather than inventing its
own):

1. `CredentialProcess != ""` → `credential-process`
2. `RoleARN != ""` → `assume-role`
3. `SSOSessionName != "" || SSOStartURL != ""` → `sso`
4. `Credentials.AccessKeyID != ""` → `static-keys`
5. else → `unknown`

No `~/.aws` directory / no files at all → empty list, `nil` error (checked
via `os.IsNotExist` on the section-scan step, not surfaced as failure).

## UI: Settings → "AWS Profiles"

Same shape as the existing Theme/Connection entries in `settings.go`
(`tview.List`, item per entry, `Enter` opens an overlay):

- New list item: `"AWS Profiles: N found"` (or `"...: none found"`).
- Opens a new read-only `tview.Table` overlay (columns: NAME, REGION, AUTH)
  — no row selection semantics beyond navigation, since this slice does
  nothing with a chosen profile. `r` re-runs discovery (files may have
  changed since cloudtui started); `Esc` closes.
- `App` gets a `listAWSProfiles func(context.Context) ([]awsprofile.Profile, error)`
  field, defaulting to `awsprofile.List`, following the same
  dependency-injection-via-field pattern already used for `backend
  queue.Backend` — lets tests substitute a fixed profile list instead of
  depending on this machine's real `~/.aws` (or lack thereof).
- Synchronous call (no goroutine/`QueueUpdateDraw`) — this is local file
  I/O, not a network call; the existing async pattern in `queues.go`/
  `messages.go` is specifically for broker round-trips.

## Testing

`awsprofile_test.go`: `t.Setenv("AWS_CONFIG_FILE", ...)` /
`t.Setenv("AWS_SHARED_CREDENTIALS_FILE", ...)` pointing at
`t.TempDir()` fixtures for: static-keys-only, SSO, assume-role
(`source_profile` chain), `credential_process` (+ the mixed
SSO-and-credential_process case seen on this real machine), a profile
present in only one of the two files, a profile present in both
(dedup — one `Profile` in the result, not two), and neither file existing
(empty list, no error).

`app`/`settings` tests: new Settings item present; opening it with an
injected `listAWSProfiles` populates the overlay table with the expected
rows; `r` re-invokes the injected function (so a test can prove refresh
actually re-queries rather than reusing a stale result).

## No changes to `config.Connection` or the connection editor (this slice).
