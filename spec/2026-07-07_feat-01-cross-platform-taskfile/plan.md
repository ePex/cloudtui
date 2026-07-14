# Plan — Cross-platform Taskfile (retroactive)

Spec: [spec.md](spec.md)

> **Retroactive.** Written 2026-07-10, after the change was already
> implemented and committed, to bring this spec into line with the
> plan/tasks-file rule added to `CLAUDE.md`'s "Feature & bugfix workflow".
> See
> [2026-07-10_bugfix-01-retroactive-workflow-compliance/spec.md](../2026-07-10_bugfix-01-retroactive-workflow-compliance/spec.md).
> It documents the approach actually taken, not a plan written in advance.

## Approach

Add a single root `Taskfile.yml` (Task v3 schema) with four top-level
targets, per CLAUDE.md's cross-platform requirement (Task, not Make; no
required shell scripts):

- `doctor` — `go version`, `java -version`, `task --version`, to confirm
  the three required local tools are on `PATH`.
- `build` — depends on `build:tui`, which runs
  `go build -o bin/cloudtui{{exeExt}} ./cmd/tui` from `tui/`, using Task's
  built-in `{{exeExt}}` template function so the same command text produces
  `cloudtui.exe` on Windows and `cloudtui` elsewhere.
- `run:tui` — `go run ./cmd/tui` from `tui/`.
- `test` — depends on `test:tui`, which runs `go test ./...` from `tui/`.

Only `tui` is wired up; `mq-proxy` has no Maven project yet, so no
`build:mq-proxy`/`test:mq-proxy` targets exist.

## Files/modules touched

- `Taskfile.yml` (new, repo root)

## Key decisions / trade-offs

- Task's own command interpreter (`mvdan/sh`) runs identical command text
  on Windows/Linux/macOS, so no OS branching or `.sh`/`.ps1` pair was
  needed for any of the four targets.
- `tui/bin/` is already covered by the root `.gitignore` (`bin/`), so no
  new ignore rule was needed.
- `build`/`test` are split into a top-level target plus a `:tui`-suffixed
  target so a second module (`mq-proxy`) can be added as another
  dependency later without renaming the top-level targets.

## Testing

`Taskfile.yml` is declarative Task configuration with no branching logic —
each target is a fixed command list. This falls under CLAUDE.md's
"genuinely untestable" carve-out (a thin wrapper with no logic), so no
unit tests were written or are being backfilled for it. See the
accompanying tasks file for the explicit call-out.
