# spec

One folder per feature, bugfix, or change request. Written stage by stage
at the pace of the workflow in `CLAUDE.md` (spec, then plan, then tasks —
each only once its predecessor is approved), then kept as the record of
what was built and why. Distinct from `docs/`, which holds longer-lived
architecture notes and ADRs.

Naming:

- Folder: `NN-<type>-<slug>/`, where `<type>` is `fe` (feature), `bugfix`,
  or `cr` (change request)
- `NN` is a single running counter shared across all three types — never
  reset, never per-type — so the folder listing itself preserves the
  order features, bugfixes, and change requests were actually done in
- Inside: `spec.md`, `plan.md`, `tasks.md`

Folder names deliberately carry no date — an earlier version of this
convention did, and it turned out misleading (a spec's folder date could
disagree with when the spec document itself was actually written, e.g.
for retroactive specs). The date each entry was implemented lives inside
`spec.md` instead (its title line or an explicit `Date:` note).

Types:

- **`fe`** — new capability.
- **`bugfix`** — fixing broken behavior.
- **`cr`** ("change request") — a deliberate change to already-shipped
  behavior that isn't a bug (a re-theme, a reworked flow, etc.),
  documented separately from the feature that originally shipped it.

`tasks.md` is a numbered checkbox list (`1. [ ] ...`); a box is checked
(`1. [x] ...`) once that task is actually implemented, not before. Each
task still needs its own explicit manual approval before it's implemented
(see `CLAUDE.md`, "Feature & bugfix workflow").

| Folder | Change |
|---|---|
| [01-fe-repo-foundations](01-fe-repo-foundations/spec.md) | `Taskfile.yml` + the spec/plan/tasks workflow itself |
| [02-fe-tui-shell-and-starting-features](02-fe-tui-shell-and-starting-features/spec.md) | tui app skeleton, k9s-style layout, global hotkeys, AWS profile selection |
| [03-fe-mq-proxy](03-fe-mq-proxy/spec.md) | `mq-proxy` service + tui queues view |
| [04-cr-shell-color-palette-evolution](04-cr-shell-color-palette-evolution/spec.md) | the shell's color palette, revised twice so far |
