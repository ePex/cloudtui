# Tasks

1. [ ] **Add the postflight quarantine-removal hook.** Add
   `hooks.post.install` to `.goreleaser.yaml`'s `homebrew_casks:` block
   per `plan.md`. Verify with `goreleaser check` and a local
   `goreleaser release --snapshot --clean --skip=publish,announce,sign`,
   inspecting the generated `dist/homebrew/Casks/cloudtui.rb` for a
   correct `postflight do ... end` block.

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
