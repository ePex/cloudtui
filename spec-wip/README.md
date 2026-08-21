# spec-wip

The staging area for active feature/bugfix/change-request work, following
the gated spec → plan → tasks workflow described in the root `CLAUDE.md`.
`spec/` (the end-state documentation) is not edited directly during
development — work happens here first.

Naming: `<type>-<slug>/`, where `<type>` is `fe` (feature), `bugfix`, or
`cr` (change request). If a folder of that name has ever existed in this
repo before (even if since deleted post-merge), pick a more specific
slug rather than reusing it.

Inside each folder: `spec.md`, then `plan.md`, then `tasks.md`, written
and approved one at a time — see `CLAUDE.md`'s workflow section for the
gates. `tasks.md` is a numbered checkbox list (`1. [ ] ...`); a box is
checked (`1. [x] ...`) once that task is actually implemented, not before.

## Lifecycle

1. A branch is created (in an isolated worktree/checkout where the
   environment supports it) and a folder added here when work starts. As
   soon as `spec.md` exists, the branch is pushed and a **draft** pull
   request opened — that PR is the review surface for every gate from
   then on, not something opened only once work is finished. The folder
   moves through spec → plan → tasks, each stage gated on explicit
   approval, pushing again after each gate document lands.
2. Once every task is implemented and checked off, its content is merged
   back into [`spec/`](../spec/README.md): the relevant `spec/<area>/spec.md`
   is updated to reflect the new end-state behavior, or a new
   `spec/<area>/` is added for a genuinely new capability.
3. The `spec-wip/type-slug/` folder is then deleted and the branch
   pushed once more. This is not a loss of history — the PR that shipped
   the change is the permanent record of what was decided and why;
   `spec-wip/` only needs to hold what's currently in flight. The PR is
   then marked ready for review (no longer draft).

An empty `spec-wip/` (no subfolders) is the expected steady state between
changes.
