# Tasks

1. [x] **Add the postflight quarantine-removal hook.** Added
   `hooks.post.install` to `.goreleaser.yaml`'s `homebrew_casks:` block
   per `plan.md`. Verified with `goreleaser check` and a local
   `goreleaser release --snapshot --clean --skip=publish,announce,sign`
   — the generated `Casks/cloudtui.rb` contains the exact expected
   `postflight do ... end` block. (Noted in passing: the snapshot dry
   run's fabricated version/URLs got confused between the `v0.4.1` and
   `tui/v0.4.1` tags now both sitting near HEAD — a `--snapshot`-only,
   local-`git describe` artifact; confirmed harmless since the real
   CI-triggered v0.4.1 release already correctly published and installed
   against the plain `v0.4.1` tag before this task started.)

2. [ ] **Merge-back.** Add a short note to
   `spec/02-ci-and-release/spec.md`'s Homebrew paragraph naming the
   postflight hook and why it exists. Delete
   `spec-wip/bugfix-homebrew-cask-gatekeeper-quarantine/`.

3. [ ] **Cut a patch release and verify live.** Once CI is green and the
   PR is merged, cut the next patch version (both `vX.Y.Z` and
   `tui/vX.Y.Z`). Once published: `brew uninstall --cask cloudtui`,
   refresh the local tap (`cd $(brew --repository ePex/tap) && git
   pull`), `brew install --cask ePex/tap/cloudtui`, then run `cloudtui
   --version` immediately — confirm it runs without the manual `xattr
   -d com.apple.quarantine` workaround this session needed before the
   fix.
