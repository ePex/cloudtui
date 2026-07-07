# spec

One markdown file per feature or bugfix, named with the date it was started
so the folder sorts into a correct chronological order. Written at the
specification stage of the workflow in `CLAUDE.md` (before the
implementation plan and task breakdown), then kept as the record of what was
built and why. Distinct from `docs/`, which holds longer-lived architecture
notes and ADRs.

Naming:

- Features: `YYYY-MM-DD_feat-<slug>.md`
- Bugfixes: `YYYY-MM-DD_bugfix-<slug>.md`

| File                                                                                       | Change                                            |
|--------------------------------------------------------------------------------------------|----------------------------------------------------|
| [2026-07-07_feat-cross-platform-taskfile.md](2026-07-07_feat-cross-platform-taskfile.md)    | `Taskfile.yml`: doctor/build/run:tui/test targets |
| [2026-07-07_feat-tui-app-scaffold.md](2026-07-07_feat-tui-app-scaffold.md)                  | `tui/`: tview app skeleton                        |
