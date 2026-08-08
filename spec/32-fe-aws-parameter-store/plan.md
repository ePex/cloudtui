# Plan — FE 32: AWS Systems Manager Parameter Store integration

## Verified against the real SDK (not assumed)

Temporarily fetched `aws-sdk-go-v2/service/ssm` to check the actual API
shape before committing to this plan (reverted after — nothing lands in
`go.mod` until this plan is approved):

- `GetParametersByPath(ctx, &GetParametersByPathInput{Path, Recursive,
  WithDecryption, NextToken})` → `[]types.Parameter{Name, Type, Value,
  LastModifiedDate, ...}`, paginated via `NextToken`.
- **Important refinement over spec.md's sketch**: `types.Parameter.Value`
  comes back on the *list* call itself, not just on a separate
  `GetParameter`. With `WithDecryption: false`, `String`/`StringList`
  values come back as real plaintext (decryption is irrelevant to them),
  while `SecureString` values come back as ciphertext — never plaintext
  without `WithDecryption: true`. So:
  - `ListParameters` can safely request `WithDecryption: false` and
    already has everything needed to show `String`/`StringList` values
    immediately, no second call.
  - Only `SecureString` needs a follow-up `GetParameter(name,
    WithDecryption: true)` when the user explicitly reveals it — and the
    ciphertext from the list call is simply discarded, never displayed.
- `types.ParameterType` is exactly `"String"` / `"StringList"` /
  `"SecureString"`.
- Credentials: `config.LoadDefaultConfig(ctx,
  config.WithSharedConfigProfile(profileName))` — this is where SSO/
  `credential_process` resolution actually executes (unlike FE 28's
  `LoadSharedConfigProfile`, which only reads config fields). A cached SSO
  token being expired means this can trigger a real browser-based login
  flow — that's AWS SDK behavior cloudtui doesn't control, but it's worth
  knowing before this is "just a fast local read" the way FE 28 was.

## `tui/internal/awsssm/`

```go
type ParameterType string // "String" | "StringList" | "SecureString"

type Parameter struct {
    Name         string
    Type         ParameterType
    Value        string // plaintext for String/StringList; empty for SecureString until revealed
    LastModified time.Time
}

// List fetches every parameter under path (recursively) for profile,
// paginating through all results. String/StringList values are populated;
// SecureString values are left empty (ciphertext is discarded, never
// surfaced) until Reveal is called for that specific name.
func List(ctx context.Context, profile, path string) ([]Parameter, error)

// Reveal fetches and decrypts a single SecureString parameter's value.
func Reveal(ctx context.Context, profile, name string) (string, error)
```

Both build an `ssm.Client` via `config.LoadDefaultConfig` +
`WithSharedConfigProfile(profile)`; an empty `profile` is a caller error
(the view layer is responsible for checking `cfg.ActiveAWSProfile != ""`
before calling in — mirroring `cmd/devtool`'s "must be a jolokia
connection" precondition check pattern, applied at the UI layer here
since there's no CLI wrapper for this one).

## New view: `tui/internal/app/ssmparams.go`

Same shape as `queuesView`/`messagesView`: a `tview.Table` (columns
NAME/TYPE/LAST MODIFIED) + filter input (same `/`-filter convention,
substring on name), registered as a real `ui.View` + `ui.Shortcuttable`,
added to `a.views` and Home's "Apps" section next to `queues`.

- `Activate()` (called by `switchTo`, same as every other registered view)
  triggers `List(ctx, cfg.ActiveAWSProfile, "/")` — errors clearly
  (rendered in the table, same pattern as `queuesView.showError`) if
  `ActiveAWSProfile` is empty, rather than calling into `awsssm` with an
  empty profile and getting a confusing SDK-level failure.
- Async (goroutine + `QueueUpdateDraw`), like `queuesView`/`messagesView`
  — this is a real network call, unlike `awsprofile.List`'s local file
  read.
- `Enter` on a `String`/`StringList` row opens a detail view (reuse
  `messageDetailView`'s rendering shape — label/value pairs — rather than
  building a third near-identical detail view) showing the value
  immediately.
- `Enter` on a `SecureString` row opens the same detail view showing
  "encrypted — press `r` to reveal" in place of a value; `r` there calls
  `Reveal` (async) and updates the same view in place.
- `c` in the detail view copies the currently-displayed value to the
  system clipboard, gated on `dv.param.Value != ""` (so it's a no-op,
  absent from `Shortcuts()`, for an unrevealed `SecureString` — same
  conditional-shortcut shape as `r`). Uses `tcell.Screen.SetClipboard`
  (OSC 52 escape sequence) rather than shelling out to `pbcopy`/`xclip`/
  `clip.exe`: no new dependency (tcell is already required), and it works
  the same over SSH as it does locally, unlike a platform clipboard
  utility. `tview.Application` doesn't expose its `tcell.Screen` directly,
  so `App` captures it once via `SetAfterDrawFunc` at construction time.

## Testing

`awsssm`: this package makes real AWS calls, so — mirroring `devtool`'s
`StartProxy`/`StopProxy` precedent (not unit tested; verified manually,
since spawning a real process/broker isn't hermetic) — the AWS-calling
functions themselves aren't unit tested against a fake AWS endpoint
(no mock server for SSM in scope here). What *is* unit-testable and will
be tested: the pagination loop logic and the `String`/`StringList` vs.
`SecureString` value-population split, refactored into small pure
functions that take an already-fetched `[]types.Parameter` page and build
`[]Parameter` — this is the part with actual logic to get wrong.

`app`: `ssmparamsView` construction, header, filter behavior, and the
"no active AWS profile" error path all use dependency injection the same
way `awsprofiles_test.go` injects `a.listAWSProfiles` — a `listParameters
func(ctx, profile, path) ([]awsssm.Parameter, error)` field on `App`.

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Manual verification per `verify-live`, against this machine's real,
   already-selected AWS profile: list loads real parameters, a
   `String`/`StringList` value shows immediately, a `SecureString` shows
   masked until `r` reveals it. No parameter values (especially decrypted
   ones) get pasted into any commit message or shown in my visible output
   beyond confirming the mechanism works — same discipline as FE 28/29's
   profile-name-only rule, tightened further here since these can be
   actual secrets, not just names.
