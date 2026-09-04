# Plan

## Approach

Everything release-prep-related already lives in `.goreleaser.yaml`'s
`before.hooks` (currently just `cd tui && go test ./...`) rather than
as separate `release.yml` workflow steps — this CR follows that same
placement instead of splitting release logic across two files.

### 1. `.github/workflows/release.yml`: add `task` to the runner

The `before.hooks` step below needs `task` on `PATH` before
`goreleaser-action` runs. Mirrors `ci.yml`'s `mq-proxy-web` job, which
already relies on the GitHub-hosted runner's preinstalled Node with no
extra `setup-node` step — same here, no Node setup added, since CI
already proves the runner has what `task build:mq-proxy-web` needs:

```yaml
      - uses: actions/setup-go@v7
        with:
          go-version-file: tui/go.mod
      - uses: go-task/setup-task@v2
      - uses: goreleaser/goreleaser-action@v7
```

(new line inserted between the existing `setup-go` and
`goreleaser-action` steps.)

### 2. `.goreleaser.yaml`: build + publish the asset

```yaml
before:
  hooks:
    - sh -c "cd tui && go test ./..."
    # Builds mq-proxy-web's distributable and copies it to the
    # well-known name release.extra_files below publishes — see
    # spec/21-amq-web-console and this file's own release.extra_files
    # entry for why: non-technical users need this as a plain
    # downloadable file on the release page, not something they build
    # themselves.
    - sh -c "task build:mq-proxy-web && cp mq-proxy-web/dist/index.html mq-proxy-web/dist/mq-console.html"
```

```yaml
release:
  extra_files:
    - glob: ./mq-proxy-web/dist/mq-console.html
```

(`release:` is a new top-level goreleaser section — nothing else in
`.goreleaser.yaml` currently configures it.)

Renaming to `mq-console.html` (rather than uploading `index.html`
as-is) happens via the `cp` in the hook, not `extra_files` itself —
goreleaser's `extra_files.glob` uploads a file under its own existing
basename, no renaming/templating built in, so the simplest reliable
way to control the published name is to make a copy with that name
before goreleaser runs.

## Files touched

- `.github/workflows/release.yml` — one new step
  (`go-task/setup-task@v2`).
- `.goreleaser.yaml` — one new `before.hooks` entry, one new top-level
  `release.extra_files` section.
- No application code, no `mq-proxy-web/` source changes.

## Testing

- No unit tests apply — this is release-pipeline configuration, not
  application code.
- Manual verification: `goreleaser release --snapshot --clean` locally
  (doesn't push/publish, but runs `before.hooks` and produces the
  `dist/` goreleaser output including what `extra_files` would
  publish) to confirm `mq-proxy-web/dist/mq-console.html` gets built
  and that goreleaser's own output references it correctly, without
  needing to actually cut a real tag/release to find out. If
  `goreleaser` isn't available locally to run this, at minimum run
  `task build:mq-proxy-web && cp mq-proxy-web/dist/index.html
  mq-proxy-web/dist/mq-console.html` by hand and confirm the file
  exists and opens correctly, plus a careful read-through of the YAML
  diff (this is infrastructure config, easy to get subtly wrong in
  ways only a real or snapshot release run would catch).
- The real, final proof is the next actual tagged release — call this
  out explicitly in `tasks.md` as something to confirm post-merge, not
  something blocking the merge itself (consistent with how config/CI
  changes in this repo generally ship and get proven by the next real
  run, e.g. the earlier `mq-proxy-web` CI job addition).

## Key decisions / trade-offs

- **No filename version suffix** (`mq-console.html`, not
  `mq-console_v0.6.0.html`) — the release page itself is already
  scoped to one version; every other asset on it is implicitly "this
  version" too. A stable, predictable filename also means a
  bookmarked/shared download link pattern doesn't need to change
  between releases (`.../releases/latest/download/mq-console.html`
  always resolves to the current release's asset, a real GitHub
  releases URL convention).
- **`before.hooks`, not a separate `release.yml` step** — keeps
  release-prep logic in one place (`.goreleaser.yaml`) rather than
  split across the workflow YAML and goreleaser config, matching where
  the existing `go test ./...` prep step already lives.
- **No `task test:mq-proxy-web` added to the release hook.** `ci.yml`
  already runs the mq-proxy-web test suite on every push, so by the
  time a tag is pushed the commit has already been tested. The
  existing `go test ./...` hook for the TUI binaries arguably exists
  as an extra last-line-of-defense specifically because goreleaser
  hooks run in a fresh checkout via `fetch-depth: 0` right before
  cutting real, user-facing binaries — but the user only asked for
  building + publishing the asset here, not new release-time test
  coverage; adding it would be scope creep beyond what this CR needs.
  Flagged here rather than silently decided either way.
