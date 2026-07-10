# 2026-07-10 — Retroactive workflow compliance for pre-existing specs

## Bug

`CLAUDE.md`'s "Feature & bugfix workflow" was just extended: the
implementation plan and task breakdown must each be their own file in
`spec/` (`-plan.md` / `-tasks.md`), every task needs separate manual
approval, and every feature/bugfix must ship with unit tests. The two specs
that predate this update are now out of compliance:

- `2026-07-07_feat-cross-platform-taskfile.md` — no plan/tasks files; no
  tests (`task test` explicitly notes "no test files yet").
- `2026-07-07_feat-tui-app-scaffold.md` — no plan/tasks files; no tests at
  all in `tui/`.

## Why

These two are the only prior work in the repo, so leaving them as-is means
the historical record contradicts the process every future change is held
to. Bringing them into line keeps `spec/` internally consistent.

## Scope

1. Append a short "Predates workflow update" note to each of the two
   existing spec files, pointing at this bugfix — no rewriting of their
   existing content.
2. Add retroactive plan + task-breakdown files for each, describing (in
   hindsight) the approach/steps actually taken:
   - `2026-07-07_feat-cross-platform-taskfile-plan.md` / `-tasks.md`
   - `2026-07-07_feat-tui-app-scaffold-plan.md` / `-tasks.md`
3. Backfill Go unit tests for the tui scaffold — the first tests in the
   repo:
   - `internal/ui/views`: each `New*()` returns the expected `Name()`/
     `Title()`.
   - `internal/app`: view registration (default active view is the first
     registered), `switchTo` (valid name switches, unknown name is a
     no-op), `onGlobalKey` (`:` focuses the prompt when unfocused, other
     keys pass through, nothing special happens when the prompt already
     has focus), `onPromptDone` (Enter with `q`/`quit` stops the app,
     Enter with a known view name switches to it, unknown command leaves
     the current view unchanged, Escape/non-Enter just returns focus).
4. For the Taskfile change specifically: `Taskfile.yml` is declarative
   config with no branching logic, so it falls under CLAUDE.md's
   "genuinely untestable" carve-out — document that explicitly in its
   retroactive plan/tasks files rather than backfilling tests for it.

## Out of scope

- No behavior changes to `app.go`, the view constructors, or `Taskfile.yml`
  — tests only, no production code changes.
- No new features, no `mq-proxy` work (nothing exists there yet to test).
- No CI wiring for `task test`.
- Not shelling out to `task` itself to "test" the Taskfile — declarative
  config is documented as untestable per above, not worked around.
