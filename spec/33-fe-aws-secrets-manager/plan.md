# Plan — FE 33: AWS Secrets Manager integration

## Verified against the real SDK (not assumed)

Temporarily fetched `aws-sdk-go-v2/service/secretsmanager` and inspected
it via `go doc` before committing to this plan (reverted after — nothing
lands in `go.mod` until this plan is approved):

- `ListSecrets(ctx, &ListSecretsInput{NextToken})` →
  `[]types.SecretListEntry{Name, ARN, LastChangedDate, RotationEnabled,
  ...}`, paginated via `NextToken`. **Confirms spec.md's decision 3**:
  the list call structurally has no value field at all — not even
  ciphertext — unlike `ssm.GetParametersByPath`.
- `GetSecretValue(ctx, &GetSecretValueInput{SecretId})` →
  `GetSecretValueOutput{SecretString *string, SecretBinary []byte, ...}`.
  Exactly one of the two is populated. No `WithDecryption` flag exists —
  Secrets Manager always decrypts on `GetSecretValue`; there's no
  ciphertext-returning list-adjacent call the way SSM has. Omitting
  `VersionId`/`VersionStage` returns `AWSCURRENT`, matching the "no
  version selection in this slice" scope decision.
- `types.SecretListEntry.RotationEnabled` is `*bool`; several other
  fields (`Description`, `KmsKeyId`, ...) exist but aren't needed for
  this slice's list/detail display (per spec.md's out-of-scope).

## `tui/internal/awssecrets/`

```go
type Secret struct {
    Name            string
    ARN             string
    LastChanged     time.Time
    RotationEnabled bool
}

// List fetches every secret's metadata for profile, paginating through
// all results. Never populates a value — ListSecrets doesn't return one.
func List(ctx context.Context, profile string) ([]Secret, error)

// Reveal fetches and decrypts a single secret's current (AWSCURRENT)
// value via GetSecretValue. isBinary is true when the secret has no
// SecretString (a SecretBinary-only secret); value is empty in that case.
func Reveal(ctx context.Context, profile, name string) (value string, isBinary bool, err error)
```

Same construction pattern as `awsssm`: `newClient(ctx, profile)` errors
on an empty profile, else `secretsmanager.NewFromConfig(config.
LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile)))`.

Pure, unit-testable helpers factored out of the AWS-calling functions
(mirrors `awsssm.buildParameters`):

- `buildSecrets(raw []types.SecretListEntry) []Secret` — nil-date
  handling, `*bool`/`*string` unwrapping, sorts by name.
- `extractValue(out *secretsmanager.GetSecretValueOutput) (value string,
  isBinary bool)` — the `SecretString`-vs-`SecretBinary` branch, tested
  directly against a hand-built `GetSecretValueOutput` without a real
  client.

## New view: `tui/internal/app/secrets.go`

Same shape as `ssmParamsView`: a `tview.Table` (columns NAME/ROTATION/
LAST CHANGED) + filter input (`/`-filter convention, substring on
name), registered as `ui.View` + `ui.Shortcuttable`, added to `a.views`
and Home's "Apps" section next to `queues`/`ssm-parameters`.

- `Activate()` triggers `List(ctx, cfg.ActiveAWSProfile)` — errors
  clearly (rendered in the table) if `ActiveAWSProfile` is empty.
- Async (goroutine + `QueueUpdateDraw`), same as `ssmParamsView`.
- Filtered title uses `"(text)"`, never `"[text]"` — FE 32 found that
  `tview.Box.Draw()` runs titles through the same tag-parsing `Print()`
  as `Table` cells, silently swallowing `[text]`. Getting this right
  from the start here, not as a follow-up fix.
- `onGlobalKey` needs an explicit exemption for
  `a.secretsV.filterInput`, same as `a.ssmParamsV.filterInput` — FE 32
  found that only overlay-tracked filter inputs (via a `*Visible` bool)
  are exempted by default; a plain top-level view's filter input needs
  its own line. Covered by a regression test mirroring
  `TestOnGlobalKeyPassesThroughWhenSSMParamsFilterFocused`.

## New view: `tui/internal/app/secretdetail.go`

Mirrors `paramDetailView` closely:

- Shows Name/ARN/Rotation/Last Changed metadata, then "(encrypted —
  press `r` to reveal)" in place of a value.
- **`fetched` and `revealed` are tracked separately** (spec.md decision
  4): `fetched` means `GetSecretValue` has completed successfully at
  least once; `revealed` means the value has actually been rendered on
  screen. `r` requires `!revealed`; `c` requires `!(fetched && isBinary)`
  — i.e. available immediately, only disappearing once a fetch reveals
  the secret is binary (nothing to ever copy). Both funnel through a
  shared `fetchThen(onSuccess func())`: if already `fetched`, calls
  `onSuccess` immediately (no network call); otherwise calls
  `a.revealSecret` (async `GetSecretValue` + `QueueUpdateDraw`) and calls
  `onSuccess` once it completes. `r`'s `onSuccess` sets `revealed = true`
  and re-renders; `c`'s copies the value and never touches `revealed`,
  so the screen stays masked. For a binary secret, `r` renders "(binary
  secret — cannot display)"; `c` (if reached before `r` establishes
  `isBinary`) shows a "cannot copy: binary secret" status instead of
  copying.
- The actual fetch-outcome handling is factored into
  `handleFetchResult(value, isBinary, err, onSuccess)`, called from
  `fetchThen`'s `QueueUpdateDraw` callback — this is what's directly
  unit-tested (success/binary/error branches), since the goroutine +
  `QueueUpdateDraw` wrapper itself can't be exercised in a test without a
  running tview event loop (established constraint — see Testing below).
- **JSON pretty-printing** (spec.md decision 5): a pure
  `prettyPrintJSON(raw string) string` helper — `json.Unmarshal` into
  `any`, then `json.MarshalIndent(..., "", "  ")`; returns `raw`
  unchanged if it doesn't parse as JSON. Applied once, when the value is
  fetched (cached on `dv.displayValue`), regardless of whether that
  fetch came from `r` or `c`. Directly unit-testable with table cases
  (valid object, valid array, plain string, invalid JSON, empty string).
- `c` copies the cached (pretty-printed, if applicable) value via the
  existing `App.copyToClipboard` — reused as-is, no changes needed there.
- Not a registered `ui.View` (opened via `App.openSecretDetail`,
  returns to `"secrets-manager"` on Esc/Backspace) — same reasoning as
  `paramDetailView`.

## `App` wiring

- New fields: `secretsV *secretsView`, `secretDetailV *secretDetailView`,
  `listSecrets func(ctx context.Context, profile string)
  ([]awssecrets.Secret, error)`, `revealSecret func(ctx context.Context,
  profile, name string) (value string, isBinary bool, err error)`.
- `a.listSecrets = awssecrets.List`, `a.revealSecret = awssecrets.Reveal`
  in `New()`, same spot as FE 32's equivalents.
- Home's "Apps" section gains `{Name: "secrets-manager", Description:
  "Browse AWS Secrets Manager secrets"}`.
- New pages `"secrets-manager"` / `"secret-detail"`; `table.
  SetSelectedFunc` → `a.openSecretDetail(a.secretsV.filtered[idx])`.
- `theme.go` gains a secrets-manager table/filter block and a
  secret-detail textview block, mirroring FE 32's blocks
  (`p.ViewColor("secrets-manager")`).

## Testing

`awssecrets`: same precedent as `awsssm` — the AWS-calling functions
(`List`, `Reveal`) aren't unit tested against a fake endpoint; `newClient`'s
empty-profile guard, `buildSecrets`, and `extractValue` are, since that's
where the actual logic to get wrong lives.

`app`: `secretsView`/`secretDetailView` construction, header, filter,
the no-active-profile error path, and the detail view's reveal/binary/
copy paths — all via injected `listSecrets`/`revealSecret`, no real AWS
calls. `prettyPrintJSON` gets its own direct table-driven test. The
filtered-title render test (`renderedScreenText`, already added in FE
32's `queues_test.go`) gets a `secrets_test.go` counterpart from the
start, rather than being retrofitted after a live bug report.

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Manual verification per `verify-live`, against this machine's real,
   already-selected AWS profile: list loads real secrets, `r` reveals a
   value (JSON pretty-printed if applicable), `c` copies it, binary
   secrets (if any exist) show the "cannot display" message instead of
   garbling bytes onscreen. No secret values get pasted into any commit
   message or shown beyond confirming the mechanism works — same
   discipline as FE 32, if anything tighter here since every value in
   this feature is, by definition, a secret.
