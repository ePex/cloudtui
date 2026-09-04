# CR: publish mq-proxy-web's dist/index.html as a release asset

Date: 2026-09-04

## Purpose

`mq-proxy-web`'s distributable is a single, self-contained
`dist/index.html` (built via `task build:mq-proxy-web` / `esbuild`) —
by design, using it needs nothing but a browser (`README.md`: "Using
it... no install, no TUI"). But today that file is `.gitignore`d and
only ever produced locally by someone who runs the build themselves,
which needs Node/npm — exactly the tooling this artifact exists to let
non-technical users avoid. A release (`v*.*.*` tag → `release.yml` →
goreleaser) currently only publishes the `cloudtui` TUI binaries
(archives) + `checksums.txt`; there's no way for a non-technical user
to get the web console without asking someone who has Node installed
to build it for them.

## Scope

- On every tagged release, build `mq-proxy-web/dist/index.html` and
  publish it as its own downloadable asset on that release (alongside
  the existing TUI archives/checksums), so a non-technical user can
  download one file from the GitHub release page and open it directly
  in a browser — no build step, no Node/npm, no TUI.
- Give the published asset a clear, descriptive filename in the
  release's asset list (not a bare `index.html`, ambiguous next to
  `cloudtui_x.y.z_linux_amd64.tar.gz`) — exact name decided in
  `plan.md`.

## Out of scope

- No change to `mq-proxy-web`'s source, build script (`build.mjs`), or
  local "Using it"/"Building it" workflow (`README.md`) — those stay
  exactly as documented; this CR only adds automatic publishing of the
  artifact they already produce.
- No change to what CI checks on every push (`ci.yml`'s `mq-proxy-web`
  job already runs `task build:mq-proxy-web` to catch build breakage —
  untouched, this CR is about the separate release/tag pipeline).
- No versioned filename requirement — a release's asset list is
  already scoped to one tag/version via the release page itself; the
  file doesn't need its own embedded version number (open question for
  `plan.md` to settle either way).
- `mq-proxy` (the Java REST proxy the web console talks to) and its own
  release/distribution story are untouched — this CR is purely about
  publishing the already-existing static web console artifact.

## Data & config

No new files beyond what the release pipeline produces at release
time. Touches `.github/workflows/release.yml` and/or `.goreleaser.yaml`
(exact split decided in `plan.md`).
