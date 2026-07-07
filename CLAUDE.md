# CLAUDE.md — Project instructions

Instructions for AI assistants (and humans) working in this repository.

## Prime directive: keep the repository clean

- **Commit only source and specifications.** Never commit build artifacts,
  binaries, coverage reports, IDE state, OS files, or generated code.
- **Small, focused commits.** One logical change per commit. Conventional
  Commits format: `feat(tui): ...`, `fix: ...`, `docs: ...`, `chore: ...`.
- **No drive-by changes.** Don't reformat, rename, or "clean up" files
  unrelated to the task at hand.
- **No dead code.** Delete unused code instead of commenting it out.
- **Dependencies are deliberate.** Justify every new dependency; prefer the
  standard library where reasonable.
- **Secrets never enter the repo.** No credentials, tokens, or account IDs —
  not in code, config, examples, or commit history. Local configuration goes
  in ignored files (e.g. `.env`, `*.local.yaml`).
- **Never commit without explicit permission.** Stage and prepare changes,
  but wait for the human (or, for an agent, the user) to explicitly say to
  commit before running `git commit`.

## Feature, bugfix & change-request workflow

When asked to implement a new feature, fix a bug, or change already-
shipped behavior (not a trivial typo or config tweak), any agent or
contributor follows this sequence and **stops for feedback at each
gate** — do not proceed to the next stage until the current one is
explicitly approved:

1. **Specification.** Write a short spec (see `spec/README.md`): what the
   feature/bug/change is, why, scope and explicit out-of-scope. File:
   `spec/NN-fe-<slug>/spec.md` (or `NN-bugfix-`/`NN-cr-`), noting the
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

**If anything is unclear** — about scope, approach, or requirements — ask
the user before proceeding. Do not make assumptions and forge ahead.

**Every code change must be traceable to a spec.** If code changes, the
relevant specification must be updated too. A code change without a
corresponding spec entry is not acceptable.

This gating applies to features, bugfixes, and change requests alike;
trivial changes can skip straight to implementation.

Three types:

- **`fe`** — new capability.
- **`bugfix`** — fixing broken behavior.
- **`cr`** ("change request") — a deliberate change to already-shipped
  behavior that isn't a bug (e.g. a re-theme, a reworked flow),
  documented separately from the feature that originally shipped it.

Every feature/bugfix/change-request gets its own folder under `spec/`,
named `NN-<type>-<slug>/` (e.g. `spec/02-fe-my-feature/`,
`spec/03-bugfix-my-fix/`). `NN` is a single running counter shared
across all three types — never reset, never per-type — so the folder
listing itself preserves the order things were actually done in (folder
names carry no date; see `spec/README.md` for why).

## Architecture

- `tui/` — Go TUI application using **tview/tcell**.

## Cross-platform requirement

Every developer on **Windows, Linux, or macOS** must be able to build, run,
and test the application with the same commands.

- **Task runner is [Task](https://taskfile.dev)** (`Taskfile.yml`), not Make —
  Make is not native on Windows.
- **Paths:** use `filepath.Join` in Go, never hardcode `/`.

## Conventions per module

- **Go (`tui/`):** see `tui/CLAUDE.md` for module-specific conventions
  (style, package layout, testing, dependencies).

## Testing

- **Every feature or bugfix must include unit tests.** New code paths need
  new tests; changed behavior needs updated tests.
- A change without tests is not done, even if the spec/plan gates above
  were followed. If something is genuinely untestable (e.g. a thin
  wrapper with no logic), say so explicitly instead of skipping silently.
- **Where behavior cannot be fully covered by unit tests**, include
  explicit manual testing instructions in the feature's `tasks.md`
  (e.g. "start the app and verify X appears on screen").

## Definition of done for a change

1. Builds cleanly, formatted, linted.
2. Unit tests added/updated and passing.
3. No new files that belong in `.gitignore`.
4. Commit message follows Conventional Commits.
5. README/docs updated if behavior or structure changed.
