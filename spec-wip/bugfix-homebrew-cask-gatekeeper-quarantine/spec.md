# Homebrew cask: strip Gatekeeper quarantine on install

Date: 2026-08-28

## What

The `cloudtui` binary distributed via the Homebrew cask
(`ePex/homebrew-tap`, shipped in the just-released fe-one-line-install
feature) currently fails to run at all after a plain `brew install --cask
ePex/tap/cloudtui`: macOS kills the process (`SIGKILL`, exit 137) on
every invocation, with no visible error in a terminal. `spctl -a -vv`
confirms Gatekeeper actively rejects it. Fix: add a `postflight` hook to
the cask (GoReleaser `homebrew_casks:`'s `hooks.post.install`) that
strips the `com.apple.quarantine` extended attribute macOS/Homebrew Cask
sets on any downloaded file, immediately after install.

## Why

Confirmed live, right after cutting v0.4.1 (the first release since the
cask started actually publishing successfully): a freshly
`brew install --cask`'d — and separately, `brew upgrade --cask`'d —
`cloudtui` binary is `adhoc`-signed (Go's default; no Apple Developer ID,
no notarization — a paid Apple Developer Program account would be needed
for real code signing, out of scope for this project) and quarantined by
Homebrew Cask as "downloaded from the internet." Combined, macOS
Gatekeeper refuses to execute it — confirmed both via the actual `SIGKILL`
on every invocation and via `spctl -a -vv .../cloudtui` reporting
`rejected`. Removing the quarantine attribute by hand
(`xattr -d com.apple.quarantine`) immediately fixes execution, confirming
the quarantine flag (not corruption, not a build issue) is the entire
cause. This means **every macOS user who has installed or upgraded via
the Homebrew cask since it started working (v0.4.1) currently cannot run
the binary at all**, silently — a `SIGKILL` with no message in a
terminal is exactly the kind of failure a user has no way to
self-diagnose.

This is GoReleaser's own documented pattern for exactly this situation
(a plain-binary cask with no code signing) — not a novel workaround;
their docs explicitly caveat it as "not real security compliance," which
is an accurate, accepted trade-off for a small open-source project
without a paid Apple Developer account.

## Scope

- `.goreleaser.yaml`'s `homebrew_casks:` block gains:
  ```yaml
  hooks:
    post:
      install: |
        if OS.mac?
          system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/cloudtui"]
        end
  ```
- Verify via a local `goreleaser release --snapshot` dry run: inspect the
  generated `Casks/cloudtui.rb` for the correct `postflight do ... end`
  block, and (since this session already has the real cask installed)
  confirm live that a fresh `brew uninstall --cask` +
  `brew install --cask` actually runs without the manual `xattr -d`
  workaround this time.
- `spec/02-ci-and-release/spec.md`'s Homebrew paragraph gets a note about
  the postflight quarantine-removal hook and why it's there.
- Cut a new patch release (next version after whatever's current at
  implementation time) once merged, specifically to get the fixed cask
  out to anyone who already hit this.

## Out of scope

- Real code signing / Apple notarization — would need a paid Apple
  Developer Program account and a more involved CI signing step; not
  pursued for this project, `xattr` removal is the accepted trade-off.
- Anything about the Scoop (Windows) or install-script distribution
  paths — Windows has no equivalent Gatekeeper mechanism, and the
  install script downloads a plain binary with no cask/app-bundle
  quarantine semantics involved. This is Homebrew-cask-specific.
