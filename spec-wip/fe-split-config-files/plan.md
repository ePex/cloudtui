# Plan

## File layout

```
~/.cloudtui/
  config.yaml                  # settings (unchanged path)
  favorites.yaml                # new
  connections/
    jolokia.yaml                # new
    proxy.yaml                  # new
  cloudtui.log                  # unchanged, unrelated
```

A subdirectory for connections (not `connections-jolokia.yaml`/
`connections-proxy.yaml` flat files) so a third backend type later
doesn't mean hyphenated-filename clutter in `~/.cloudtui/` directly.

Each file's top-level content **is** the section, no wrapper key —
the filename already scopes it, so nesting under a redundant
`connections:`/`awsFavorites:` key inside the file itself would be
pure noise:

```yaml
# ~/.cloudtui/connections/jolokia.yaml
- name: default
  backend: jolokia
  queue:
    brokerName: localhost
    url: http://localhost:8161/api/jolokia
    username: admin
```

```yaml
# ~/.cloudtui/favorites.yaml
ssmParameters:
  my-profile:
    - /app/db/password
secrets:
  my-profile:
    - prod/db
logGroups: {}
```

`backend: jolokia` stays in each entry even though it's implied by
which file it's in — per `spec.md`'s "organization only" scope, the
`Connection` struct/schema doesn't change at all, so both files
literally marshal `[]Connection` with the exact same type or on
the same tag. Simpler than introducing per-file trimmed types, and
keeps `Connection` a single, uniform type everywhere else in the
codebase (dialogs, `ActiveConn()`, `SecretAWSProfile()`, ...).

## `internal/config/config.go` changes

`Config` (the in-memory struct every other package already uses) is
**unchanged** — same fields, same JSON/YAML tags where they matter for
other purposes. Only the on-disk representation and the functions that
produce/consume it change:

```go
// settingsFile is config.yaml's on-disk shape: everything except
// connections and favorites, which live in their own files (see
// spec-wip/fe-split-config-files, now spec/01 and spec/12).
type settingsFile struct {
	ActiveConnection string        `yaml:"activeConnection"`
	ActiveAWSProfile string        `yaml:"activeAWSProfile"`
	Datadog          DatadogConfig `yaml:"datadog"`
	Theme            string        `yaml:"theme"`
	Logo             []string      `yaml:"logo"`
	Colors           Palette       `yaml:"colors"`
}
```

No new type needed for connections files (`[]Connection` directly) or
favorites (`AWSFavorites` directly — its existing fields already have
the right yaml tags for a bare top-level document).

**Path helpers** (`favoritesPath`/`jolokiaConnectionsPath`/
`proxyConnectionsPath`, all deriving from `filepath.Dir(settingsPath)`)
so `Load(path)`/`Save(path, cfg)` keep their existing single-path
signature — callers (`LoadDefault`/`SaveDefault`/every existing test)
don't change at all; the sibling files just land next to whatever path
they already pass:

```go
func favoritesPath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), "favorites.yaml")
}
func connectionsDir(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), "connections")
}
```

**`Load(path string) (Config, error)`**:
1. Read+unmarshal `path` into `settingsFile`, apply to a `Config`
   seeded from `Default()` (theme/colors/logo/etc. — same merge
   logic as today, unchanged).
2. Read `connectionsDir(path)/jolokia.yaml` and `.../proxy.yaml` if
   present; concatenate into `cfg.Connections`.
3. Read `favoritesPath(path)` if present; set `cfg.AWSFavorites`.
4. **Legacy-format fallback**: if *neither* a connections file nor a
   favorites file exists yet, fall back to today's behavior — unmarshal
   `path`'s raw content into the *old* full-`Config`-shaped struct
   (still checking for a `connections:`/`awsFavorites:` key present in
   the settings file, same "raw second unmarshal" trick already used
   for `raw.Colors`/legacy top-level `backend`/`queue`/`proxy` today)
   and use that. **Purely in-memory** — matches the existing legacy
   migrations' behavior exactly: no eager rewrite on load, the new
   split files get written the next time anything calls `Save`
   (i.e. `SaveDefault`, which every mutation already goes through per
   `spec.md`'s call-site audit). A user who never changes anything
   keeps running on the old combined file indefinitely, which is fine.
5. Env-var password/access-token injection: unchanged, still applied
   after connections/settings are loaded.

**`Save(path string, cfg Config) error`**:
1. Marshal a `settingsFile` (extracted from `cfg`) to `path`.
2. Partition `cfg.Connections` by `.Backend` into two slices; write
   each to `connectionsDir(path)/jolokia.yaml` /
   `.../proxy.yaml` (creating the `connections/` dir with
   `os.MkdirAll` first). An empty slice still writes an empty-list
   file (`[]` or nothing) rather than leaving a stale file from before
   a connection was deleted — matches `Save`'s existing
   whole-value-wins semantics (it's not a partial patch today, so this
   isn't a new class of behavior).
3. Marshal `cfg.AWSFavorites` to `favoritesPath(path)`.
4. Any single write failing returns an error immediately (matches
   today's `Save`'s all-or-nothing framing) — `plan.md`'s "Key
   decisions" below has the one nuance worth flagging: partial-write
   risk on a mid-sequence failure.

**`DefaultPath()`**: unchanged (still the settings file path — the
"one path that resolves everything else" role it already plays for
`LoadDefault`/`SaveDefault`, now also implicitly the anchor
`favoritesPath`/`connectionsDir` derive from).

**`migrateLegacyConfig`**: unchanged — it only concerns the
pre-relocation `tui/config.yaml` → `~/.cloudtui/config.yaml` move,
orthogonal to this split.

## `tui/config.example.yaml`

Split to mirror the real layout:
- `tui/config.example.yaml` — trimmed to settings only (theme, logo,
  colors, activeConnection, activeAWSProfile, datadog), with a note at
  the top pointing to the other three example files.
- `tui/connections/jolokia.example.yaml`,
  `tui/connections/proxy.example.yaml` — today's `connections:` list
  content, split by backend, unwrapped (bare list, matching the real
  file's shape).
- `tui/favorites.example.yaml` — today's `awsFavorites:` content,
  unwrapped.

## Files touched

- `tui/internal/config/config.go` — as above.
- `tui/internal/config/config_test.go` — substantial updates, not a
  pass-through (see Testing below): several existing tests assert on
  single-file behavior that no longer matches the new on-disk shape
  and need rewriting, not just re-verifying.
- `tui/config.example.yaml` split into 4 files as above.
- `spec/01-repo-and-tui-shell/spec.md`, `spec/12-named-connections/spec.md`,
  and the ~8 other spec files a grep for `config.yaml`/`awsFavorites`
  turns up (final list confirmed at merge-back time) — updated to
  describe the new file layout.
- No changes anywhere in `internal/app`, `internal/dialog`,
  `internal/view`, `internal/devtool` — confirmed via `spec.md`'s
  call-site audit.

## Testing

**This is not a "no test file needs changes" refactor** — unlike the
production-only refactors earlier this session, the on-disk *format*
itself is changing, which several existing tests assert on directly
(e.g. `TestLoadConnectionsNewFormat` currently writes a single
combined `config.yaml` with an embedded `connections:` key and asserts
it loads — post-refactor that's the *legacy* fallback path, not "new
format," so the test needs reframing, not just re-running).

- Update existing round-trip tests
  (`TestSaveLoadRoundTripWithConnection`,
  `...WithPasswordSecret`, `...WithActiveAWSProfile`,
  `...WithDatadogConfig`, `...WithAWSFavorites`) to verify against the
  new multi-file layout (e.g. asserting `connections/jolokia.yaml`
  exists and contains the right content after `Save`, not just that
  `Load` reads back what `Save` wrote — both matter now).
- Rename/reframe `TestLoadConnectionsNewFormat` to something like
  `TestLoadLegacyEmbeddedConnections` (or fold into the legacy-fields
  tests) — it's now testing the migration fallback, not "new format."
- New tests: `Save` correctly partitions mixed jolokia+proxy
  connections into their respective files; `Load` correctly merges
  them back; a `favorites.yaml`-only or `connections/`-only partial
  split-out state (e.g. hand-crafted) still loads sensibly; the
  legacy-fallback path only activates when *neither* new-format file
  exists (not e.g. only `favorites.yaml` existing while connections
  are still embedded — decide/document exact precedence in the task
  that implements this).
- Every existing non-format-specific test (theme loading, palette
  overrides, env var injection, `ActiveConn()`, `SecretAWSProfile()`,
  ...) should need no changes — those don't touch file *layout* at
  all.
- `go build`/`go vet`/`go test ./...` after every task, `gofmt` before
  every commit, same as always.

## Key decisions / trade-offs

- **No wrapper key inside each file** (bare list/struct at the
  document root) — the filename already scopes the content type;
  wrapping it again inside the file is pure redundancy `config.yaml`
  itself doesn't have for its own sections either (well, it does
  today, since everything shares one file — but each *new* file
  should read like "the thing it's named after," not "a fragment of
  the old combined shape").
- **`backend` field kept redundant in per-file entries** rather than
  introducing per-backend trimmed types — matches the confirmed
  "organization only, not a data-model change" scope; a bigger
  refactor here would ripple into `Connection`-consuming code this CR
  otherwise doesn't need to touch at all.
- **Legacy migration stays lazy** (in-memory only until the next
  `Save`), matching the two existing legacy-migration precedents in
  this exact file — no new eager-rewrite-on-load behavior introduced
  for this one.
- **Partial-write risk on `Save`, not addressed here**: if e.g. the
  settings file writes successfully but the jolokia connections file
  write then fails (disk full, permissions), `Save` returns an error
  but the settings file is already updated — a pre-existing class of
  risk (today's single-file `Save` isn't atomic either — a failure
  mid-`os.WriteFile` can already leave a truncated file), just now
  spread across more files where a failure could leave them
  inconsistent with each other rather than just with the caller's
  intent. Not fixed here (no atomic-write/temp-file-rename mechanism
  exists in this codebase today for *any* file write) — flagged as a
  pre-existing category of risk this CR makes marginally wider, not a
  new one it introduces from scratch, and out of scope to fix as part
  of a file-layout change.
- **Precedence when both legacy-embedded and new-format files
  exist**: new-format files win (see `Load` step 4) — a user who's
  already migrated and then hand-edits stale content back into
  `config.yaml` shouldn't have it silently override their real,
  current connections/favorites files.
