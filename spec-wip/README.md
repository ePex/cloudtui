# spec-wip

The staging area for active feature/bugfix/change-request work, following
the gated spec → plan → tasks workflow described in the root `CLAUDE.md`.
`spec/` (the end-state documentation) is not edited directly during
development — work happens here first.

Naming: `NN-<type>-<slug>/`, where `<type>` is `fe` (feature), `bugfix`,
or `cr` (change request) — same convention `spec/` used to use. `NN` is a
single running counter, continuing from the old `spec/` counter's last
value (**next number: 90**) rather than restarting at 1 — this keeps every
folder name that has ever existed in this repo unique, even after a
`spec-wip/` folder is deleted post-merge. Never reset, never reused.

Inside each folder: `spec.md`, then `plan.md`, then `tasks.md`, written
and approved one at a time — see `CLAUDE.md`'s workflow section for the
gates. `tasks.md` is a numbered checkbox list (`1. [ ] ...`); a box is
checked (`1. [x] ...`) once that task is actually implemented, not before.

## Lifecycle

1. A folder is created here when work starts and moves through
   spec → plan → tasks, each stage gated on explicit approval.
2. Once every task is implemented and checked off, its content is merged
   back into [`spec/`](../spec/README.md): the relevant `spec/<area>/spec.md`
   is updated to reflect the new end-state behavior, or a new
   `spec/<area>/` is added for a genuinely new capability.
3. The `spec-wip/NN-type-slug/` folder is then deleted. This is not a
   loss of history — the PR that shipped the change is the permanent
   record of what was decided and why; `spec-wip/` only needs to hold
   what's currently in flight.

An empty `spec-wip/` (no subfolders) is the expected steady state between
changes.
