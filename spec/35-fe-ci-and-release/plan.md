# Plan — FE 35: CI and first release via GitHub Actions

## Verified against real docs (not assumed)

Checked GoReleaser's, Task's, and the relevant GitHub Actions' actual
current documentation before committing to this plan (infra/CI YAML is
expensive to get wrong by guessing — a bad config only fails once
pushed):

- **GoReleaser monorepo building is Pro-only** (`monorepo:` section).
  The OSS-available way to build a Go module that isn't at the repo
  root is simply `builds[].dir: tui` in a repo-root `.goreleaser.yaml`
  — no Pro license needed for this project's shape (one Go module in
  a subdirectory).
- **GoReleaser v2 requires a top-level `version: 2` field**; omitting
  it produces a deprecation warning today and will hard-fail in a
  future version.
- **`archives[].formats` is a list** (`formats: [tar.gz]`), the plural
  form recommended since v2.6 — `format:` (singular string) still
  works but is deprecated. Per-OS override is `format_overrides:
  [{goos: windows, formats: [zip]}]`.
- **`before.hooks` entries can be objects** with `cmd`/`dir`/`output`
  fields, so `go test ./...` can run scoped to `tui/` as a pre-release
  safety net (GoReleaser doesn't run tests on its own).
- **The official GitHub Actions workflow GoReleaser documents**:
  `actions/checkout@v4` with `fetch-depth: 0` (full history, needed
  for changelog generation), `actions/setup-go@v7` with
  `go-version-file`, then `goreleaser/goreleaser-action@v7` with
  `version: "~> v2"` and `args: release --clean`, gated by a
  top-level `permissions: contents: write` block (the action needs
  that to create the Release and upload assets) and
  `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` (the automatic
  per-run token — no new secret to create).
- **`go-task/setup-task@v2`** is the task-runner org's own official
  action for installing Task in a workflow (there's also a
  longer-established third-party `arduino/setup-task`; the official
  one is preferred here). Works the same way across all three OS
  runners since it just installs the `task` binary.
- **mq-proxy targets Java 21`** (`build.gradle.kts`: `jvmTarget =
  JvmTarget.JVM_21`) → `actions/setup-java@v4` with `distribution:
  temurin`, `java-version: '21'`.

## `Taskfile.yml`: fix mq-proxy's Windows invocation

`test:proxy`, `run:proxy`, and the JAR-building half of `build:proxy`
currently hardcode `./gradlew`, which doesn't run on native Windows
(no shebang interpreter for the extension-less wrapper script) even
though `mq-proxy/gradlew.bat` already exists. Fixed with the same
`{{if eq OS "windows"}}...{{end}}` pattern Task's own docs use (`OS`
is a built-in template variable — `windows`/`linux`/`darwin`):

```yaml
test:proxy:
  desc: Run mq-proxy unit tests.
  dir: mq-proxy
  cmds:
    - '{{if eq OS "windows"}}gradlew.bat{{else}}./gradlew{{end}} test'

run:proxy:
  desc: Run the mq-proxy application from source.
  dir: mq-proxy
  cmds:
    - '{{if eq OS "windows"}}gradlew.bat{{else}}./gradlew{{end}} bootRun'
```

`build:proxy` is split in two, so CI can build the JAR (cross-platform)
without also requiring `podman` (which CI's matrix runners won't have
configured, and which isn't part of what "does this build" needs to
verify — see spec.md decision 2):

```yaml
build:proxy:
  desc: Build the mq-proxy JAR and container image.
  deps: [build:proxy:jar]
  dir: mq-proxy
  cmds:
    - podman build -t mq-proxy .

build:proxy:jar:
  desc: Build the mq-proxy JAR (no container image).
  dir: mq-proxy
  cmds:
    - '{{if eq OS "windows"}}gradlew.bat{{else}}./gradlew{{end}} bootJar'
```

`task build:proxy` still does exactly what it did before (jar +
image); `task build:proxy:jar` is the new, CI-usable, podman-free half.

## `.github/workflows/ci.yml`

Two jobs, both matrixed over `ubuntu-latest`/`macos-latest`/
`windows-latest` (`fail-fast: false` so one OS failing doesn't cancel
the others mid-run — seeing all three results matters more than fast
feedback here):

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  tui:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v7
        with:
          go-version-file: tui/go.mod
      - uses: go-task/setup-task@v2
      - run: task test:tui
      - run: task build:tui

  mq-proxy:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-java@v4
        with:
          distribution: temurin
          java-version: '21'
      - uses: go-task/setup-task@v2
      - run: task test:proxy
      - run: task build:proxy:jar
```

`go-version-file: tui/go.mod` (rather than a hardcoded `go-version:`)
means the workflow never drifts from whatever Go version the module
actually declares.

## `.goreleaser.yaml` (repo root)

Repo-root placement (not `tui/.goreleaser.yaml`) because
`goreleaser-action` runs `goreleaser release` from the repo root by
default, and `builds[].dir: tui` handles the "module isn't at the
root" part — no need to change the invocation's working directory.

```yaml
version: 2

project_name: cloudtui

before:
  hooks:
    - cmd: go test ./...
      dir: tui

builds:
  - id: tui
    dir: tui
    main: ./cmd/tui
    binary: cloudtui
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - id: tui
    ids: [tui]
    formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    name_template: "cloudtui_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Bug fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Other changes
      order: 999
  filters:
    exclude:
      - '^docs:'
      - '^chore:'
      - '^test:'
      - 'Merge pull request'
```

`before.hooks` running `go test ./...` in `tui/` is a safety net beyond
what tag-pushing discipline alone guarantees — if a tag ever gets
pushed against a commit that hadn't actually passed CI, the release
aborts instead of publishing broken binaries.

## `.github/workflows/release.yml`

```yaml
name: release

on:
  push:
    tags: ['v*.*.*']

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v7
        with:
          go-version-file: tui/go.mod
      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## `tui/cmd/tui/main.go`: a `--version` flag

A release without any way to confirm what you downloaded isn't very
useful. Smallest possible addition — no new dependency (the existing
build has no CLI flags at all yet, so reaching for the stdlib `flag`
package for a single boolean would be more machinery than the problem
needs):

```go
var version = "dev" // overridden via -ldflags at release build time

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("cloudtui " + version)
		return
	}
	// ... existing startup unchanged
}
```

`{{.Version}}` in the `ldflags` template is GoReleaser's own resolved
version for the tag (the `v` prefix stripped — tag `v0.1.0` → `0.1.0`),
so a released binary prints `cloudtui 0.1.0`; a locally-built one
(`task build:tui`, no ldflags) prints `cloudtui dev`.

## `README.md`

Short "Installing a release" section added once this pipeline exists,
pointing at the GitHub Releases page and noting the archive-per-OS/
arch naming scheme — kept minimal (no install script, no package
manager, per spec.md's out-of-scope list).

## Testing

- `Taskfile.yml`'s conditional gradlew fix: no dedicated test (it's
  build tooling, not application code) — verified by the CI workflow
  itself actually succeeding on Windows once this ships, which is the
  only real proof that matters here.
- `main.go`'s `--version` handling: manual check (`go run ./cmd/tui
  --version` and `-v`), not unit tested — `cmd/tui` has no existing
  test file and this is a two-line startup branch with no logic to
  get wrong, consistent with `tui/CLAUDE.md`'s "if something is
  genuinely untestable, say so explicitly" allowance for thin
  entrypoint code.
- The workflows themselves are verified by actually pushing them and
  watching a real run (CI workflow: push this branch/PR and watch all
  6 matrix jobs; release workflow: verified against a real tag **after**
  explicit go-ahead, since pushing a tag is the one genuinely
  hard-to-reverse, public action in this whole feature — see spec.md
  decision 6 and CLAUDE.md's guidance on irreversible actions).

## Definition of done

1. `task test` and `task build` still pass locally (the Taskfile
   changes must not break the commands developers already use).
2. CI workflow runs green on all 6 matrix jobs (3 OS × {tui, mq-proxy})
   for a real pushed commit/PR.
3. Release workflow is reviewed and believed correct, but **not**
   exercised against a real tag as part of this task list — that's a
   separate, explicit step once this is merged (pushing `v0.1.0` and
   watching the release workflow actually publish).
4. `README.md` updated to mention the release download path.
