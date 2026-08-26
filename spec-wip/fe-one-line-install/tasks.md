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

3. [ ] **Install script: Windows.** Add `scripts/install.ps1` per
   `plan.md` §2.
   **Manual verification**: this session only has direct access to
   macOS — note in this task's checkbox comment (or ask the user) how
   Windows verification was actually performed (a Windows machine/VM, or
   deferred to a later manual check) since it can't be exercised
   directly here.

4. [ ] **README: install script section.** Add the macOS/Linux and
   Windows one-liners to `README.md`'s "Installing a release" section
   (ahead of the manual-download fallback, per `plan.md` §7 — the rest
   of that section's restructuring happens incrementally as each
   remaining method lands, task 8 does the final pass).

5. [ ] **Homebrew tap.** Create the `ePex/homebrew-tap` repo (empty,
   public). Add the `brews:` block to `.goreleaser.yaml` per `plan.md`
   §3, and the `HOMEBREW_TAP_GITHUB_TOKEN` secret + env passthrough to
   `.github/workflows/release.yml`. **Blocked on the user**: creating the
   PAT and running `gh secret set HOMEBREW_TAP_GITHUB_TOKEN --repo
   ePex/cloudtui` — this task pauses for that before it can be
   considered done (verify with `gh secret list`, name only, never the
   value). No live release exists yet to test this against; verification
   happens for real at the next tag push (task 10 covers cutting one).

6. [ ] **Scoop bucket.** Same shape as task 5, targeting
   `ePex/scoop-bucket` and `SCOOP_BUCKET_GITHUB_TOKEN`.

7. [ ] **apt/rpm/apk via nfpm + Gemfury.** Add the `nfpms:` block to
   `.goreleaser.yaml` per `plan.md` §5. **Blocked on the user**: creating
   a Gemfury account and generating a push token — needed before the
   `publishers:` block's exact repo-add instructions (for README) can be
   confirmed against the real account. This task pauses here; once the
   account/token exist, add the `publishers:` block, the
   `FURY_PUSH_TOKEN`/`FURY_ACCOUNT` secrets, and the env passthrough to
   `release.yml`.

8. [ ] **README: finish the rewrite.** Fold in Homebrew, Scoop, apt (the
   real Gemfury repo-add one-liner from task 7), and a `go install`
   line — completing the "Installing a release" restructure started in
   task 4, in the order from `plan.md` §7. Manual archive download stays
   as the documented fallback.

9. [ ] **Merge-back.** Update `spec/02-ci-and-release/spec.md` (replace
   the stale out-of-scope line, document the five methods, the new
   `.goreleaser.yaml` blocks, the two new repos, and the new secrets by
   name) and `spec/01-repo-and-tui-shell/spec.md` (the theming section's
   `config.yaml` reference → `~/.cloudtui/config.yaml` + migration note).
   Delete `spec-wip/fe-one-line-install/`.

10. [ ] **Cut a release and verify end-to-end.** Once CI is green and
    the PR is merged, cut the next tag (following the existing
    manual-tag convention, spec/02) and confirm: the GitHub Release gets
    the usual 6 archives + checksums, a Homebrew formula lands in
    `ePex/homebrew-tap`, a Scoop manifest lands in `ePex/scoop-bucket`,
    and the `.deb`/`.rpm`/`.apk` show up on Gemfury. Then actually run
    `brew install ePex/tap/cloudtui` and the install script once against
    the real new release.
