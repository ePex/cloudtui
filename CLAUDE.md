# CLAUDE.md — Project instructions

Instructions for AI assistants (and humans) working in this repository.

## Prime directive: keep the repository clean

- **Commit only source and specifications.** Never commit build artifacts,
  binaries, coverage reports, IDE state, OS files, or generated code.
  Generated clients/stubs (e.g. from the OpenAPI spec) are produced at build
  time, not checked in.
- **Small, focused commits.** One logical change per commit. Conventional
  Commits format: `feat(tui): ...`, `fix(mq-proxy): ...`, `docs: ...`,
  `chore: ...`.
- **No drive-by changes.** Don't reformat, rename, or "clean up" files
  unrelated to the task at hand.
- **No dead code.** Delete unused code instead of commenting it out.
- **Dependencies are deliberate.** Justify every new dependency; prefer the
  standard library where reasonable.
- **Secrets never enter the repo.** No credentials, tokens, broker passwords,
  or account IDs — not in code, config, examples, or commit history. Local
  configuration goes in ignored files (e.g. `.env`, `*.local.yaml`).

## Feature, bugfix & change-request workflow

When asked to implement a new feature, fix a bug, or change already-
shipped behavior (not a trivial typo or config tweak), any agent or
contributor follows this sequence and **stops for feedback at each
gate** — do not proceed to the next stage until the current one is
explicitly approved:

1. **Specification.** Write a short spec (see `spec/README.md`): what the
   feature/bug/change is, why, scope and explicit out-of-scope. File:
   `spec/feat-NN-<slug>/spec.md` (or `bugfix-NN-`/`chg-NN-`), noting the
   date inside `spec.md` itself (not the folder name). Ask for feedback.
   Revise until approved.
2. **Implementation plan.** Once the spec is approved, write the plan to
   `plan.md` in that same folder — approach, files/modules touched, key
   technical decisions and trade-offs. Ask for feedback. Revise until
   approved.
3. **Task breakdown.** Once the plan is approved, write the breakdown to
   `tasks.md` in that same folder — a numbered checkbox list (`1. [ ]
   ...`) of discrete, reviewable steps. Each task requires explicit manual
   approval before it is implemented — do not implement several tasks and
   present them together, and do not move to the next task until the
   current one is done and the next has been separately approved. Check a
   task's box (`1. [x] ...`) once it's actually been implemented, not
   before.

This gating applies to features, bugfixes, and change requests alike;
trivial changes can skip straight to implementation.

Three types:

- **`feat`** — new capability.
- **`bugfix`** — fixing broken behavior.
- **`chg`** ("change request") — a deliberate change to already-shipped
  behavior that isn't a bug (e.g. a re-theme, a reworked flow),
  documented separately from the feature that originally shipped it.

Every feature/bugfix/change-request gets its own folder under `spec/`:

- `spec/feat-NN-<slug>/`
- `spec/bugfix-NN-<slug>/`
- `spec/chg-NN-<slug>/`

`NN` is a two-digit counter starting at `01`, counted separately per type
and never reset (it does not reset by date — folder names carry no date;
see `spec/README.md` for why).

`NN` is a two-digit counter starting at `01`, reset each day and counted
separately for `feat` vs `bugfix`.

## Architecture (agreed decisions)

- `tui/` — Go, using **tview/tcell** (k9s-style: command prompt, resource
  views, detail panes). AWS access via `aws-sdk-go-v2` with the default
  credential chain. AWS calls live in internal service wrappers, never in
  UI code; UI stays non-blocking.
- Queue operations go through a **`QueueBackend` Go interface**. The primary
  implementation is `proxy` (talking to `mq-proxy`), used **both locally and
  in AWS** so there is one code path everywhere. Amazon MQ's Jolokia API is
  read-only and remote JMX is unavailable, hence the proxy. A direct
  `jolokia` implementation may be added later as an optional read path.
- `mq-proxy/` — Spring Boot service exposing REST endpoints for queue
  list/browse/send/purge/move, implemented over JMS/OpenWire (statistics
  plugin for listing, QueueBrowser for browsing, transacted consume for
  purge/move). Deployed inside the VPC next to the broker; API secured
  with HTTP Basic Auth (credentials from Secrets Manager in AWS, from
  local config for the `local` profile) — both environments use the same
  auth mechanism, not a bearer-token scheme.
  - A `local` Spring profile starts an **embedded ActiveMQ broker**
    (`BrokerService` with a TCP connector) in the same JVM — this is the
    local dev stack. No Docker required.
- `api/` — the OpenAPI spec is the **single source of truth** for the
  proxy contract. Go client and Spring controller interfaces are generated
  from it at build time.

## Cross-platform requirement (hard constraint)

Every developer on **Windows, Linux, or macOS** must be able to build, run,
and test the application **and the full local stack** with the same commands.

- **No hard Docker dependency.** Docker (e.g. LocalStack, containerized
  ActiveMQ) may only ever be an optional convenience, never a required step.
  Required local dependencies are exactly: a Go toolchain, a JDK, and Task.
- **Local broker = embedded ActiveMQ** via the mq-proxy `local` profile,
  not a container.
- **Local AWS = a sandbox AWS account/profile** for Secrets Manager and
  Parameter Store (standard SSM parameters are free; secrets cost cents).
  LocalStack remains an optional alternative for Docker users.
- **Task runner is [Task](https://taskfile.dev)** (`Taskfile.yml`), not Make —
  Make is not native on Windows. All dev workflows (build, run, test, lint,
  generate) are Task targets and must work on all three OSes.
- **No required shell scripts.** Anything scripted must be a Task/Go/Maven
  target, or ship as a `.sh` + `.ps1` pair.
- **Maven wrapper is committed** (`mvnw` + `mvnw.cmd`); never assume a local
  Maven installation.
- **Paths and line endings:** use `filepath.Join` in Go, never hardcode `/`;
  `.gitattributes` enforces LF in the repo (with `.cmd`/`.bat` as CRLF).
- **Releases:** GoReleaser cross-compiles the TUI for linux/darwin/windows,
  amd64 + arm64.

## Conventions per module

- **Go (`tui/`):** `gofmt`/`goimports` formatting is mandatory. Errors are
  wrapped with context (`fmt.Errorf("...: %w", err)`). Secret values are
  masked by default in the UI and never logged.
- **Java (`mq-proxy/`):** Spring Boot with Maven wrapper (`mvnw` is committed,
  `target/` is not). Configuration via `application.yaml` +
  environment variables; no credentials in config files.
- **Docs (`docs/`):** significant design decisions get a short ADR-style
  markdown file.

## Testing

- **Every feature or bugfix must include unit tests.** New code paths need
  new tests; changed behavior needs updated tests. This applies across
  modules: table-driven tests in Go (`tui/`), JUnit tests in Java
  (`mq-proxy/`).
- A change without tests is not done, even if the spec/plan gates above
  were followed. If something is genuinely untestable (e.g. a thin
  wrapper with no logic), say so explicitly instead of skipping silently.

## Definition of done for a change

1. Builds cleanly, formatted, linted.
2. Unit tests added/updated and passing.
3. No new files that belong in `.gitignore`.
4. Commit message follows Conventional Commits.
5. README/docs updated if behavior or structure changed.
