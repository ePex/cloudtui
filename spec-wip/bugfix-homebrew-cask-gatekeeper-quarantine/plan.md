# Implementation plan

## Approach

`.goreleaser.yaml`'s `homebrew_casks:` block gains a `hooks.post.install`
entry (verified against GoReleaser's own `homebrew_casks` docs, which
cover this exact scenario — a plain-binary cask with no code signing):

```yaml
homebrew_casks:
  - name: cloudtui
    repository:
      owner: ePex
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/ePex/cloudtui"
    description: "A terminal UI for managing cloud resources."
    license: "MIT"
    binaries:
      - cloudtui
    hooks:
      post:
        install: |
          if OS.mac?
            system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/cloudtui"]
          end
```

No other file changes needed — this only touches how the cask formula
itself is generated, not the build/archive/checksum pipeline.

## Verification

Two stages, since the cask is only ever actually *published* by a real
release (a `--snapshot` dry run writes the `.rb` file locally but pushes
nothing, and Homebrew only ever installs from the published tap repo):

1. **Local, in this PR**: `goreleaser check` + `goreleaser release
   --snapshot --clean --skip=publish,announce,sign`, then inspect
   `dist/homebrew/Casks/cloudtui.rb` for a correctly rendered
   `postflight do ... end` block (Homebrew Cask's own compiled form of
   `hooks.post.install`).
2. **Live, after the next real release** (part of that release's own
   verification, not blocking this PR's merge): `brew uninstall --cask
   cloudtui`, update the local tap
   (`cd $(brew --repository ePex/tap) && git pull`), `brew install
   --cask ePex/tap/cloudtui`, then run `cloudtui --version` immediately
   — no manual `xattr -d` step this time — confirm it prints the version
   instead of being killed. This session already has the cask installed
   and already reproduced the bug directly (`spctl -a -vv` → `rejected`,
   `SIGKILL` on launch), so re-testing here directly proves the fix
   rather than relying on inference from the generated Ruby alone.

## Merge-back

`spec/02-ci-and-release/spec.md`'s Homebrew paragraph (the one added for
fe-one-line-install, describing `homebrew_casks:`) gets a short addition
naming the postflight quarantine-removal hook and why it exists (adhoc
signing, no notarization, no paid Apple Developer account for this
project) — so a future reader doesn't wonder why a `system_command`
`xattr` call is sitting in release config.

## Release

Once merged and CI is green, cut the next patch version (`vX.Y.Z` +
`tui/vX.Y.Z`, per the dual-tag convention) specifically to get the fixed
cask out — anyone who installed via Homebrew since v0.4.1 currently
cannot run the binary at all.
