# spec

One folder per feature or bugfix, named with the date it was started plus
a same-day sequence number, so folders sort into a correct chronological
order even when several land on the same day. Written stage by stage at
the pace of the workflow in `CLAUDE.md` (spec, then plan, then tasks —
each only once its predecessor is approved), then kept as the record of
what was built and why. Distinct from `docs/`, which holds longer-lived
architecture notes and ADRs.

Naming:

- Folder: `YYYY-MM-DD_feat-NN-<slug>/` / `YYYY-MM-DD_bugfix-NN-<slug>/`
  — `NN` is a two-digit counter starting at `01`, reset each day and
  counted separately for `feat` vs `bugfix` (so a day's first bugfix is
  `bugfix-01` even if that same day already has several `feat-NN` folders)
- Inside: `spec.md`, `plan.md`, `tasks.md`

`tasks.md` is a numbered checkbox list (`1. [ ] ...`); a box is checked
(`1. [x] ...`) once that task is actually implemented, not before. Each
task still needs its own explicit manual approval before it's implemented
(see `CLAUDE.md`, "Feature & bugfix workflow").

| Folder | Change |
|---|---|
| [2026-07-07_feat-01-cross-platform-taskfile](2026-07-07_feat-01-cross-platform-taskfile/spec.md) | `Taskfile.yml`: doctor/build/run:tui/test targets |
| [2026-07-07_feat-02-tui-app-scaffold](2026-07-07_feat-02-tui-app-scaffold/spec.md) | `tui/`: tview app skeleton |
