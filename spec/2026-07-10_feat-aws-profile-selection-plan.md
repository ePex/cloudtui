# Plan — AWS profile selection

Spec: [2026-07-10_feat-aws-profile-selection.md](2026-07-10_feat-aws-profile-selection.md)

## Approach

### Profile discovery (`internal/awsprofile`, new package, stdlib only)

```go
package awsprofile

// List returns the AWS profile names found in the user's shared config
// and credentials files (~/.aws/config, ~/.aws/credentials, or their
// AWS_CONFIG_FILE/AWS_SHARED_CREDENTIALS_FILE overrides), merged and
// de-duplicated. Missing files contribute nothing — not an error.
func List() ([]string, error) {
    return ListFrom(configFilePath(), credentialsFilePath())
}

// ListFrom is List's injectable core, for testing against fixture files
// instead of the real ~/.aws.
func ListFrom(configPath, credentialsPath string) ([]string, error) {
    names := map[string]struct{}{}
    if err := scanConfigProfiles(configPath, names); err != nil {
        return nil, err
    }
    if err := scanCredentialsProfiles(credentialsPath, names); err != nil {
        return nil, err
    }
    out := make([]string, 0, len(names))
    for n := range names {
        out = append(out, n)
    }
    sort.Strings(out)
    return out, nil
}
```

- `scanConfigProfiles`: for each `[...]` section header, `[default]` →
  profile `"default"`; `[profile <name>]` → profile `<name>`; anything
  else (`[sso-session ...]`, etc.) is skipped.
- `scanCredentialsProfiles`: every `[...]` header (including `[default]`)
  *is* a profile name directly — that file has no other section type.
- Both scanners: a missing file (`os.IsNotExist`) contributes nothing and
  isn't an error; other read errors propagate.
- `configFilePath`/`credentialsFilePath`: `AWS_CONFIG_FILE`/
  `AWS_SHARED_CREDENTIALS_FILE` env vars if set, else
  `filepath.Join(home, ".aws", "config"/"credentials")` via
  `os.UserHomeDir()`.

### Config changes (`internal/config`)

```go
type Config struct {
    Logo   []string  `yaml:"logo"`
    Colors Palette   `yaml:"colors"`
    AWS    AWSConfig `yaml:"aws"`
}

type AWSConfig struct {
    Profile string `yaml:"profile"`
}

// Save writes cfg to path as YAML.
func Save(path string, cfg Config) error { ... }

// SaveDefault saves to config.yaml in the working directory, mirroring
// LoadDefault's path resolution.
func SaveDefault(cfg Config) error { return Save("config.yaml", cfg) }
```

`Default().AWS.Profile` stays `""` ("not set").

### Settings view + profile picker (`internal/app`, new `settings.go`)

The `settings` placeholder moves out of `internal/ui/views` entirely —
unlike every other view, it needs live config read/write and to trigger
an overlay, which would otherwise couple the views package to
`internal/config`/`internal/app`. `internal/ui/views/settings.go` is
deleted; `NewSettings` no longer exists there.

- `newSettingsView(a *App) ui.View` builds a `*tview.List` (which embeds
  `*tview.Box`, so it gets a colored border via the existing `bordered`
  wiring automatically — no changes needed there) with one item:
  main text `"AWS Profile"`, secondary text `a.cfg.AWS.Profile` or
  `"not set"`, `selected: a.openProfilePicker`.
- `a.openProfilePicker()`: calls `awsprofile.List()`. On error or an
  empty result, shows a single informational item (`"no profiles
  found"` / `"error: <msg>"`, no `selected` callback) instead of
  crashing. Otherwise one item per profile, `selected:
  func() { a.selectProfile(name) }`; pre-highlights (`SetCurrentItem`)
  `AWS_PROFILE`/`AWS_DEFAULT_PROFILE` if set and present, else
  `"default"` if present. `SetDoneFunc: a.closeProfilePicker` (Escape
  cancels). Shown via a new `"profile-picker"` page on `rootPages`
  (`ShowPage`/`HidePage`, `centered()` — same precedent as `"help"`).
- `a.selectProfile(name)`: sets `a.cfg.AWS.Profile = name`, calls
  `config.SaveDefault(a.cfg)` (logs to stderr on failure, non-fatal —
  the in-memory selection still applies for the session), updates the
  settings list's item text (`SetItemText`) and the top bar's info panel
  (`refreshInfoPanel`), then closes the picker.
- `a.closeProfilePicker()`: hides the page, refocuses `a.pages`.

### Top bar reflects the selection (`internal/app/topbar.go` + `app.go`)

- `newInfoPanel`'s text-building logic is extracted into
  `infoPanelText(cfg config.Config) string`, so it can be reused for
  both the initial render and later refreshes.
- `topBar` gains an `info *tview.TextView` field (the same primitive
  `newInfoPanel` already builds, just retained instead of discarded).
- `App` gains an `infoPanel *tview.TextView` field (from `tb.info`) and
  a `refreshInfoPanel()` method that re-sets its text from `a.cfg` —
  called once at startup (covers a profile already persisted from a
  prior session) and again from `selectProfile`.

## Files touched

- `tui/internal/awsprofile/awsprofile.go`, `awsprofile_test.go` (new)
- `tui/internal/config/config.go` (modified) — `AWSConfig`, `Save`/
  `SaveDefault`
- `tui/internal/config/config_test.go` (modified) — round-trip test,
  new field's default
- `tui/internal/ui/views/settings.go` (deleted)
- `tui/internal/ui/views/views_test.go` (modified) — drop the
  `"settings"` case
- `tui/internal/app/settings.go`, `settings_test.go` (new)
- `tui/internal/app/topbar.go` (modified) — expose `info`, extract
  `infoPanelText`
- `tui/internal/app/topbar_test.go` (modified)
- `tui/internal/app/app.go` (modified) — construct the settings view via
  `newSettingsView(a)`, wire `openProfilePicker`/`selectProfile`/
  `closeProfilePicker`/`refreshInfoPanel`, add the `"profile-picker"`
  `rootPages` page
- `tui/internal/app/app_test.go` (modified) — views slice construction
  changes for `"settings"`
- `tui/config.example.yaml` (modified) — document `aws.profile` as
  normally set via the picker, not hand-edited

No new dependency (still stdlib-only for this feature; `aws-sdk-go-v2`
isn't needed until a feature actually calls AWS).

## Key decisions / trade-offs

- **No `aws-sdk-go-v2` dependency yet.** Section-name parsing is simple
  enough that pulling in the SDK (or a general ini library) isn't
  justified for this pass; it arrives when secrets/params/queues get
  real backends.
- **`ListFrom(configPath, credentialsPath)` is the injectable core**,
  mirroring the existing `config.Load(path)`/`LoadDefault()` split —
  easy to unit test against temp fixture files instead of the real
  `~/.aws`.
- **`sso-session` (and any other non-`[default]`/`[profile ...]`)
  sections in `~/.aws/config` are explicitly excluded**; everything in
  `~/.aws/credentials` is treated as a profile (that file has no other
  section type).
- **Settings moves into `internal/app`, out of `internal/ui/views`** —
  the one exception to "views are stateless placeholders," for the same
  reason border-color wiring lives in `app.go` rather than the views
  package: it needs things (config, overlay control) the views package
  intentionally doesn't depend on.
- **Profile-picker tests set `AWS_CONFIG_FILE`/
  `AWS_SHARED_CREDENTIALS_FILE` via `t.Setenv`** to point at fixture
  files, rather than adding an injectable "profile lister" field to
  `App` — keeps `New()`'s signature unchanged and avoids DI machinery
  for a single test concern.
- **Persistence failures are logged, not fatal** — a read-only
  `config.yaml` shouldn't crash the picker, just fail to remember the
  choice for next time.
- **No profile validation.** Any name found in the files is selectable
  regardless of whether it resolves to usable credentials — out of
  scope per the approved spec.

## Testing

- `internal/awsprofile`: `ListFrom` against temp files covering
  `[default]`, `[profile foo]`, a `[sso-session x]` block (must be
  excluded), `[bar]` in credentials, overlapping names across both
  files (de-duplicated), and both files missing (empty result, no
  error).
- `internal/config`: `Save`→`Load` round-trip preserves `AWS.Profile`
  alongside the existing fields; `Default().AWS.Profile == ""`.
- `internal/app/settings_test.go` (using `t.Setenv` for
  `AWS_CONFIG_FILE`/`AWS_SHARED_CREDENTIALS_FILE` against fixture
  files): the settings list's item reflects `a.cfg.AWS.Profile` (or
  `"not set"`); `openProfilePicker` populates one item per discovered
  profile with correct pre-selection; `selectProfile` updates
  `a.cfg.AWS.Profile`, the settings list text, and the info panel, then
  closes the picker; Escape (`closeProfilePicker`) leaves `a.cfg`
  unchanged.
- `internal/app/topbar_test.go`: `infoPanelText` reflects a configured
  `AWS.Profile` vs. the existing placeholder text when unset.
