# spec

One markdown file per feature or bugfix, named with the date it was started
so the folder sorts into a correct chronological order. Written at the
specification stage of the workflow in `CLAUDE.md` (before the
implementation plan and task breakdown), then kept as the record of what was
built and why. Distinct from `docs/`, which holds longer-lived architecture
notes and ADRs.

Naming:

- Spec: `YYYY-MM-DD_feat-<slug>.md` / `YYYY-MM-DD_bugfix-<slug>.md`
- Implementation plan: `YYYY-MM-DD_feat-<slug>-plan.md` / `-bugfix-<slug>-plan.md`
- Task breakdown: `YYYY-MM-DD_feat-<slug>-tasks.md` / `-bugfix-<slug>-tasks.md`

Each stage is its own file, written once its predecessor is approved (see
`CLAUDE.md`, "Feature & bugfix workflow"). Each task listed in a
`-tasks.md` file needs explicit manual approval before it's implemented.

| File                                                                                       | Change                                            |
|--------------------------------------------------------------------------------------------|----------------------------------------------------|
| [2026-07-07_feat-cross-platform-taskfile.md](2026-07-07_feat-cross-platform-taskfile.md)    | `Taskfile.yml`: doctor/build/run:tui/test targets |
| [2026-07-07_feat-tui-app-scaffold.md](2026-07-07_feat-tui-app-scaffold.md)                  | `tui/`: tview app skeleton                        |
