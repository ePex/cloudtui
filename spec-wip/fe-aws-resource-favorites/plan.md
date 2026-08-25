# Plan

## Files touched

- `tui/internal/config/config.go` — new `AWSFavorites` struct, a
  `FavoriteKind` type, and the data-level logic (`IsFavorite`, `Toggle`).
- `tui/internal/config/config_test.go` — unit tests for the new type and
  its YAML round-trip.
- `tui/internal/ui/host.go` — one new `Host` interface method,
  `ToggleFavorite(kind config.FavoriteKind, profile, name string)`.
- `tui/internal/app/host.go` — `App`'s implementation of `ToggleFavorite`
  (mutate + persist, mirroring `SetActiveAWSProfile`/`SaveDatadogConfig`).
- `tui/internal/app/host_test.go`, `tui/internal/dialog/hosttest_test.go`,
  `tui/internal/view/testfake_test.go` — one new stub method each, since
  all three implement the `Host` interface for their own package's tests.
- `tui/internal/view/ssmparams.go`, `secrets.go`, `logs.go` — star column,
  `f` toggle, favorite-first sort, `Shortcuts()` entry. Same shape in all
  three; no shared base type exists between these views today (each is a
  standalone struct), so the sort/column logic is a small package-level
  helper in `internal/view` reused by all three call sites rather than a
  new shared type.
- `tui/internal/view/ssmparams_test.go`, `secrets_test.go`, `logs_test.go`
  — tests for the star column, toggle behavior, and sort ordering, using
  the existing `fakeViewHost` (`testfake_test.go`).
- Merge-back: `spec/15-aws-parameter-store/spec.md`,
  `spec/16-aws-secrets-manager/spec.md`,
  `spec/17-aws-cloudwatch-logs/spec.md`.

No new dependencies.

## Config schema

```go
// FavoriteKind identifies which of AWSFavorites' three namespaces a
// favorite belongs to. Parameters, secrets, and log groups are
// independent namespaces — the same name can be favorited in one and not
// another.
type FavoriteKind string

const (
	FavoriteSSMParameter FavoriteKind = "ssmParameter"
	FavoriteSecret       FavoriteKind = "secret"
	FavoriteLogGroup     FavoriteKind = "logGroup"
)

// AWSFavorites holds favorited item names per AWS profile, one map per
// FavoriteKind. Sparse: an unlisted profile or name means "not
// favorited", not an error.
type AWSFavorites struct {
	SSMParameters map[string][]string `yaml:"ssmParameters,omitempty"` // profile -> favorited parameter names
	Secrets       map[string][]string `yaml:"secrets,omitempty"`       // profile -> favorited secret names
	LogGroups     map[string][]string `yaml:"logGroups,omitempty"`     // profile -> favorited log group names
}

func (f AWSFavorites) IsFavorite(kind FavoriteKind, profile, name string) bool { ... }

// Toggle returns a new AWSFavorites with name's favorite status in kind/profile
// flipped (favorited -> unfavorited or vice versa).
func (f AWSFavorites) Toggle(kind FavoriteKind, profile, name string) AWSFavorites { ... }
```

Added to `Config` as `AWSFavorites AWSFavorites \`yaml:"awsFavorites,omitempty"\``.

### Key decisions

- **One `Host` method parameterized by `FavoriteKind`, not three.** All
  three of `internal/app/host_test.go`, `internal/dialog/hosttest_test.go`,
  and `internal/view/testfake_test.go` implement the full `Host`
  interface; three new methods would mean nine new stub implementations
  across those fakes for what's otherwise identical logic, versus three
  for a single kind-parameterized method. Matches the existing
  `promptCommandTable`-style preference in this codebase for one
  mechanism over N near-identical ones.
- **Reads bypass `Host` entirely.** `Host.Config()` already returns the
  full `config.Config` by value, and every view already reads fields off
  it directly (e.g. `p := a.Config().Colors`) rather than through a
  dedicated accessor — `pv.host.Config().AWSFavorites.IsFavorite(...)` is
  consistent with that, so only the *mutating* half
  (`ToggleFavorite`) needs a `Host` method, mirroring how
  `SetActiveAWSProfile`/`SaveDatadogConfig` exist for writes but there's
  no `GetDatadogConfig` — reads go through `Config()`.
- **`Toggle` returns a new value rather than mutating in place.** Matches
  `config.ApplyPaletteOverrides`'s existing style in this file (returns a
  new `Palette` rather than mutating the receiver) — keeps `AWSFavorites`
  itself trivially comparable/testable without aliasing concerns over its
  maps.

## View changes (SSM Parameters, Secrets Manager, CloudWatch Logs)

Each view's `repaint()` currently builds `pv.filtered` from `pv.all` (the
last-loaded full list) plus the active text filter, then renders rows in
that order. This changes to:

1. Filter (unchanged).
2. Sort: favorite-first (favorited rows before non-favorited), each group
   keeping the view's existing order (currently just as-loaded/name order
   — none of these three views has a column-sort toggle like Queues'
   `o`/`O`, so "existing order" here just means stable, not a second
   explicit sort key to preserve).
3. Render: a new leftmost column (header blank, like the existing header
   row's other columns) showing `★` for a favorited row, blank otherwise.

`f` on the table (mirroring the existing `case 'r':`/`case '/':` switch in
each view's `table.SetInputCapture`) calls
`pv.host.ToggleFavorite(kind, profile, name)` for the selected row, then
re-runs the same repaint the row would already go through on reload —
no separate code path needed.

### Key decisions

- **A package-level helper, not a shared embedded type.** `SSMParamsView`,
  `SecretsView`, and `LogsView` are three independent structs with no
  common base — introducing one now, for three call sites, would be more
  restructuring than this feature needs. A free function like
  `sortFavoritesFirst[T any](items []T, isFavorite func(T) bool)
  []T` (or the equivalent without generics if that reads more in line
  with this codebase's existing style — decided during implementation)
  in `internal/view` is reused by all three `repaint()` methods instead.
- **Star column is genuinely a new column**, not text prepended to the
  name cell — per the approved design (star column + pinned to top).
  Each view's `setHeader()` gains one more entry; `onSelect`'s
  `idx := row - 1` math is unaffected (row-to-item indexing doesn't care
  how many columns exist).

## Testing

- `config_test.go`: table-driven tests for `IsFavorite`/`Toggle` — toggle
  on then off returns to empty state, independent kinds don't interact,
  independent profiles don't interact, YAML round-trip via existing
  `Load`/`Save` test helpers.
- Each view's `_test.go`: extend the existing `fakeViewHost`-backed tests
  — toggling via a simulated `f` keypress (or calling the view's toggle
  path directly, whichever this codebase's existing input-capture tests
  already do for `r`/`/`) moves a row to the top and shows a star;
  toggling again removes it; a different profile's favorites don't leak
  into the current one (fake host returns per-profile data, matching how
  `ListParameters` etc. are already faked).
- Manual verification: real AWS profile with real parameters/secrets/log
  groups (or two profiles, to check the per-profile scoping isn't
  accidentally global) — favorite an item in each view, confirm the star
  and top-sort, switch profile and confirm the set of favorites changes
  with it, switch back and confirm persistence, and check `config.yaml`
  directly after a toggle. No `verify-live`-style broker needed (this
  feature doesn't touch queue/message/connection behavior).

## Trade-offs / risks accepted

- `FavoriteKind` as a typed string (not an int enum) — consistent with
  this codebase's existing preference for readable YAML/log output over
  the smallest possible on-disk representation (e.g. `Connection.Backend`
  is also a string).
- No migration path for existing `config.yaml` files — `AWSFavorites`
  being `omitempty` and absent means "no favorites yet," which is exactly
  the desired behavior for every config that predates this feature.
