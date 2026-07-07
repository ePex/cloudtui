# Repo foundations: repository structure

Date: 2026-07-07

## What

A Go TUI application with the following top-level layout:

```
cloudtui/
├── tui/         # Go TUI application
├── spec/        # Feature/bugfix/change-request specifications
├── Taskfile.yml # Cross-platform task runner (build, run, test)
└── CLAUDE.md    # Project instructions for agents and contributors
```

### `tui/`

Standard Go module layout:

```
tui/
├── cmd/tui/   # main package — entry point
├── internal/  # private application packages (not importable from outside)
├── go.mod
└── go.sum
```

Follows standard Go conventions: `cmd/` for the binary entry point, `internal/` for all application code.

### `spec/`

One folder per feature, bugfix, or change request. See `spec/README.md` for naming conventions.

## Why

Establishing a clear, conventional layout from the start keeps the project navigable as it grows and makes expectations explicit for contributors and agents alike.

## Scope

- Top-level directory layout.
- Go module structure inside `tui/`.

## Out of scope

- Build tooling details — documented in `CLAUDE.md` and `Taskfile.yml` itself.
- Agent workflow rules — documented in `CLAUDE.md`.
- Additional modules — covered by their own specs when introduced.
