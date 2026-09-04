# Tasks

1. [x] `internal/config/config.go`: added `settingsFile` struct and the
   `favoritesPath`/`connectionsDir` path helpers (plus
   `loadConnectionList`/`loadFavorites`/`saveConnectionList`); rewrote
   `Load` to read the 3-file layout (with the legacy-embedded fallback
   when neither connections nor favorites files exist) and `Save` to
   write it, per `plan.md`. `Config`'s own fields, `Default()`,
   `ApplyPaletteOverrides`, `PaletteUserOverrides`,
   `PaletteForTheme`/`AvailableThemes`, `ActiveConn()`,
   `SecretAWSProfile()`, `AWSFavorites`' methods, `DefaultPath()`,
   `migrateLegacyConfig`, `LoadDefault`/`SaveDefault` signatures all
   unchanged, confirmed via full diff review.

   `go build`/`go vet` pass. `go test ./...` (full repo) **also
   already passes**, ahead of task 2's schedule — the existing
   round-trip tests are Save-then-Load through the same code path, so
   they pass regardless of on-disk layout, and the existing legacy
   hand-written-YAML tests still hit the fallback path correctly since
   no split files exist for them. Task 2 is still needed: today's
   round-trip tests only assert on the in-memory result, not the
   actual multi-file structure on disk, and the "new format" test
   needs reframing as legacy-fallback coverage — passing today isn't
   the same as *testing the new behavior*.

2. [x] `internal/config/config_test.go`: strengthened
   `TestSaveLoadRoundTripWithPasswordSecret` (mixed jolokia+proxy
   connections) and `TestSaveLoadRoundTripWithAWSFavorites` to assert
   directly on the on-disk split files via
   `loadConnectionList`/`loadFavorites`, not just the round-tripped
   in-memory result. Renamed/reframed `TestLoadConnectionsNewFormat` →
   `TestLoadLegacyEmbeddedConnections` (doc comment explains why: it
   was "new" relative to the pre-FE22 top-level fields, now it's the
   legacy-fallback path relative to the split). Added
   `TestLoadPrefersSplitConnectionsOverLegacyEmbedded`,
   `TestLoadPrefersSplitFavoritesOverLegacyEmbedded`, and
   `TestLoadPartiallyMigratedState` (favorites.yaml present,
   connections still legacy-embedded — each half loads independently
   and correctly). Confirmed every test not about file layout (theme
   loading, palette overrides, env var injection, `ActiveConn()`,
   `SecretAWSProfile()`, `TestLoadMigrationFromLegacyFields`/
   `...QueueFields`, ...) needed no changes.

   One bug caught by the new tests during writing (in the test, not
   production): `TestLoadPrefersSplitConnectionsOverLegacyEmbedded`
   initially called `saveConnectionList` directly without
   `os.MkdirAll`-ing `connections/` first — `Save()` itself always
   does this before calling `saveConnectionList`, so production is
   unaffected; fixed by adding the same `MkdirAll` call to the test's
   setup.

   `go build`/`go vet`/`go test ./...` all pass, full suite, no
   exceptions.

3. [ ] Split `tui/config.example.yaml` into 4 files
   (`tui/config.example.yaml` trimmed to settings,
   `tui/connections/jolokia.example.yaml`,
   `tui/connections/proxy.example.yaml`,
   `tui/favorites.example.yaml`), per `plan.md`. Manual check: copy
   each example to its real `~/.cloudtui/...` location (in a scratch
   `$HOME` or similar) and confirm `cloudtui` starts cleanly and shows
   the expected settings/connection/favorites.

4. [ ] Merge-back: update `spec/01-repo-and-tui-shell/spec.md`,
   `spec/12-named-connections/spec.md`, and every other spec file a
   fresh grep for `config.yaml`/`awsFavorites`/`~/.cloudtui` turns up
   to describe the new file layout accurately (some may need only a
   path/filename correction, others a fuller rewrite of the section
   describing the format — judge per file, not a mechanical
   find-replace). Delete `spec-wip/fe-split-config-files/`. Mark the
   PR ready for review.
