# spec

One folder per feature, bugfix, or change request. Written stage by stage
at the pace of the workflow in `CLAUDE.md` (spec, then plan, then tasks —
each only once its predecessor is approved), then kept as the record of
what was built and why. Distinct from `docs/`, which holds longer-lived
architecture notes and ADRs.

Naming:

- Folder: `<type>-NN-<slug>/`, where `<type>` is `feat`, `bugfix`, or
  `chg`
- `NN` is a two-digit counter starting at `01`, counted separately per
  type and never reset — it's a running "this was the Nth `<type>`"
  count, not tied to a date
- Inside: `spec.md`, `plan.md`, `tasks.md`

Folder names deliberately carry no date — an earlier version of this
convention did, and it turned out misleading (a spec's folder date could
disagree with when the spec document itself was actually written, e.g.
for retroactive specs). The date each entry was implemented lives inside
`spec.md` instead (its title line or an explicit `Date:` note).
Chronological order across entries is conveyed by the sequence number
within a type plus the date recorded in each `spec.md`, not by sorting
folder names.

Types:

- **`feat`** — new capability.
- **`bugfix`** — fixing broken behavior.
- **`chg`** ("change request") — a deliberate change to already-shipped
  behavior that isn't a bug (a re-theme, a reworked flow, etc.),
  documented separately from the feature that originally shipped it.

`tasks.md` is a numbered checkbox list (`1. [ ] ...`); a box is checked
(`1. [x] ...`) once that task is actually implemented, not before. Each
task still needs its own explicit manual approval before it's implemented
(see `CLAUDE.md`, "Feature & bugfix workflow").

| Folder | Change |
|---|---|
| [feat-01-repo-foundations](feat-01-repo-foundations/spec.md) | `Taskfile.yml` + the spec/plan/tasks workflow itself |
| [feat-02-tui-shell-and-starting-features](feat-02-tui-shell-and-starting-features/spec.md) | tui app skeleton, k9s-style layout, global hotkeys, AWS profile selection |
| [chg-01-shell-color-palette-evolution](chg-01-shell-color-palette-evolution/spec.md) | the shell's color palette, revised twice so far |
| [feat-03-mq-proxy](feat-03-mq-proxy/spec.md) | `mq-proxy` service + tui queues view |
