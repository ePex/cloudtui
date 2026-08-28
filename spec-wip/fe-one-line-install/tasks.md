# Tasks

1. [x] **Config relocation.** Add `config.DefaultPath()` and
   `migrateLegacyConfig(legacyPath, newPath string) error` to
   `tui/internal/config/config.go`; rewire `LoadDefault`/`SaveDefault` to
   use them; update `tui/cmd/tui/main.go`'s startup log line to use
   `DefaultPath()` instead of `filepath.Abs("config.yaml")`. Add/update
   unit tests per `plan.md` §1 (`TestDefaultPath`,
   `TestMigrateLegacyConfigCopiesWhenDestAbsent`,
   `TestMigrateLegacyConfigNoopWhenDestExists`,
   `TestMigrateLegacyConfigNoopWhenSourceAbsent`,
   `TestLoadDefaultMigratesOnFirstRun`,
   `TestSaveDefaultWritesUnderHomeConfigDir`, plus the rewritten
   `TestLoadDefaultFallsBackWhenAbsent`).
   **Manual verification**: build the binary, temporarily rename
   `~/.cloudtui/config.yaml` out of the way if one exists, run from
   `tui/` (where `config.yaml` — your real one — lives) and confirm a
   log line shows it was migrated to `~/.cloudtui/config.yaml`, then run
   the binary again from a *different* directory and confirm it still
   picks up the same (now-migrated) config. Restore your real
   `~/.cloudtui/config.yaml` afterward if this test moved it.

2. [x] **Install script: macOS/Linux.** Add `scripts/install.sh` per
   `plan.md` §2. Add a `shellcheck` step to `.github/workflows/ci.yml`
   (or a new lint job — whichever fits the existing workflow shape
   better) running against this one script.
   **Manual verification**: run it locally against the real latest
   release (`v0.3.0`) into a scratch `CLOUDTUI_INSTALL_DIR`, confirm the
   binary runs and `--version` matches; run it once more with a bad
   `CLOUDTUI_VERSION` (e.g. `v0.0.0-nope`) and confirm it fails cleanly
   instead of hanging or extracting garbage.

3. [x] **Install script: Windows.** Added `scripts/install.ps1` per
   `plan.md` §2.
   **Manual verification**: no Windows machine available in this
   session, but `pwsh` (PowerShell 7, arm64) was installed locally via
   `brew install powershell` specifically to actually execute this
   script rather than only reading it — caught and fixed two real bugs
   this way (`$env:TEMP`/`$env:LOCALAPPDATA` aren't set outside real
   Windows; switched to `[System.IO.Path]::GetTempPath()` and
   `[Environment]::GetFolderPath(...)`, both of which *do* resolve
   correctly on real Windows too, so this wasn't just a test-environment
   workaround). Confirmed end-to-end against the real `v0.3.0` release:
   downloaded `cloudtui_0.3.0_windows_arm64.zip`, verified the extracted
   binary is a genuine `PE32+ ... Aarch64, for MS Windows` executable,
   confirmed clean failure on a bad `CLOUDTUI_VERSION`, and unit-checked
   the checksum-line-parsing logic in isolation. Still recommend a real
   Windows machine confirms it once, since `pwsh`-on-macOS can't
   exercise real Windows PATH semantics or `.exe` execution.

4. [x] **README: install script section.** Add the macOS/Linux and
   Windows one-liners to `README.md`'s "Installing a release" section
   (ahead of the manual-download fallback, per `plan.md` §7 — the rest
   of that section's restructuring happens incrementally as each
   remaining method lands, task 8 does the final pass).

5. [x] **Homebrew tap.** Created the `ePex/homebrew-tap` repo (with an
   initial commit so it has a default branch to push to). Added a
   `homebrew_casks:` block to `.goreleaser.yaml` per `plan.md` §3 — note
   the correction there: GoReleaser's `brews:` is hard-deprecated as of
   v2.16, replaced with the cask-based mechanism, confirmed via a local
   `goreleaser check` + `goreleaser release --snapshot --clean` dry run
   (inspected the generated `Casks/cloudtui.rb`, correct per-OS/arch
   URLs and checksums). Added the `HOMEBREW_TAP_GITHUB_TOKEN` env
   passthrough to `.github/workflows/release.yml`. User created the PAT
   and ran `gh secret set` — confirmed registered via `gh secret list`
   (name only). No live release exists yet to test the actual push; that
   happens for real at the next tag (task 10).

6. [x] **Scoop bucket.** Created `ePex/scoop-bucket` (with an initial
   commit). Added the `scoops:` block to `.goreleaser.yaml` (this key is
   not deprecated, unlike `brews:`) and the `SCOOP_BUCKET_GITHUB_TOKEN`
   env passthrough. Verified via a local `goreleaser check` +
   `--snapshot` dry run, inspected the generated `scoop/cloudtui.json`
   (correct `64bit`/`arm64` entries, `bin: cloudtui.exe`). User created
   the PAT and secret — confirmed via `gh secret list`.

7. [x] ~~apt/rpm/apk via nfpm + Gemfury~~ — **descoped 2026-08-28, user
   request**. The `nfpms:` block was added, verified via a local
   `goreleaser --snapshot` dry run (real `.deb`/`.rpm`/`.apk` built,
   metadata confirmed by unpacking one), then removed again before the
   Gemfury account/`publishers:` step — the user decided the account
   setup wasn't worth it for a third channel beyond Homebrew/Scoop. See
   `plan.md` §5 for the full record of what was built and why it was
   pulled. No secrets were ever created for this one.

8. [x] **README: finish the rewrite.** Folded in Homebrew, Scoop, and a
   `go install` line — completing the "Installing a release" restructure
   started in task 4, in the order from `plan.md` §7. Manual archive
   download stays as the documented fallback. While writing the
   `go install` line, actually ran it locally and found it produced a
   binary named `tui` (Go names it after the package directory, not the
   module) — inconsistent with every other method's `cloudtui`. Per
   `plan.md` §6, renamed `tui/cmd/tui/` → `tui/cmd/cloudtui/` (`git mv`)
   to fix it at the source rather than document around it; updated every
   real reference (`.goreleaser.yaml`, `Taskfile.yml`, `tui/README.md`,
   `tui/CLAUDE.md`, `tui/scripts/smoke-test.sh`,
   `.claude/skills/verify-live/SKILL.md`) and re-verified: `go build`/
   `go vet`/`go test ./...` all clean, `task build:tui` still works,
   `goreleaser check` + a `--snapshot` dry run still produce a correctly
   named `cloudtui` binary inside the archive, and `go install
   ./cmd/cloudtui` now genuinely installs a binary named `cloudtui`.
   `spec/01`/`spec/06`'s own `cmd/tui` references are intentionally left
   for task 9 (merge-back), matching the rule that `spec/` only changes
   there.

9. [ ] **Merge-back.** Update `spec/02-ci-and-release/spec.md` (replace
   the stale out-of-scope line, document the four methods, the new
   `.goreleaser.yaml` blocks, the two new repos, and the new secrets by
   name) and `spec/01-repo-and-tui-shell/spec.md` (the theming section's
   `config.yaml` reference → `~/.cloudtui/config.yaml` + migration note,
   **and** its repo-layout tree's `cmd/tui/` → `cmd/cloudtui/`, per the
   rename in task 8). Also fix `spec/06-logging/spec.md`'s
   `cmd/tui/main.go` reference to `cmd/cloudtui/main.go`. Delete
   `spec-wip/fe-one-line-install/`.

10. [ ] **Cut a release and verify end-to-end.** Once CI is green and
    the PR is merged, cut the next tag (following the existing
    manual-tag convention, spec/02) and confirm: the GitHub Release gets
    the usual 6 archives + checksums, a Homebrew cask lands in
    `ePex/homebrew-tap`, and a Scoop manifest lands in
    `ePex/scoop-bucket`. Then actually run `brew install --cask
    ePex/tap/cloudtui` and the install script once against the real new
    release.
