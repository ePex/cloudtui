# 2026-07-07 — Cross-platform Taskfile

Commit: `chore: add cross-platform Taskfile with doctor/build/run/test targets`

## Change

Added `Taskfile.yml` at the repo root with four targets:

- **`doctor`** — runs `go version`, `java -version`, `task --version` to
  confirm the three required local tools are on `PATH`.
- **`build`** — depends on `build:tui`, which runs
  `go build -o bin/cloudtui{{exeExt}} ./cmd/tui` from `tui/`. `{{exeExt}}` is
  Task's built-in template function, so the binary is named `cloudtui.exe`
  on Windows and `cloudtui` elsewhere without any OS branching.
- **`run:tui`** — `go run ./cmd/tui` from `tui/`.
- **`test`** — depends on `test:tui`, which runs `go test ./...` from `tui/`.

## Why

CLAUDE.md's cross-platform requirement mandates Task (not Make, since Make
isn't native on Windows) as the runner for every dev workflow, and forbids
required shell scripts. Task's own command interpreter (`mvdan/sh`) runs the
same command text on Windows/Linux/macOS, so no `.sh`/`.ps1` pair was needed
for these targets.

`tui/bin/` is covered by the existing root `.gitignore` (`bin/`), so build
output is never committed.

## Verification

```
task doctor   # prints go1.26.4, JDK 21, task 3.52.0
task build    # produces tui/bin/cloudtui.exe
task test     # go test ./... — no test files yet, exits 0
```

## Scope note

Only `tui` is wired up. `mq-proxy` has no `pom.xml`/`mvnw` yet (see
[2026-07-07_feat-02-tui-app-scaffold/spec.md](../2026-07-07_feat-02-tui-app-scaffold/spec.md) —
out of scope for that change too), so no `build:mq-proxy`/`test:mq-proxy`
targets were added; `build`/`test` will gain a second dependency once the
Spring Boot module exists.

## Predates workflow update

This spec predates the plan/tasks-file and mandatory-unit-test rules added
to `CLAUDE.md`'s "Feature & bugfix workflow" — see
[2026-07-10_bugfix-01-retroactive-workflow-compliance/spec.md](../2026-07-10_bugfix-01-retroactive-workflow-compliance/spec.md)
for the retroactive plan/tasks files brought in line with those rules.
