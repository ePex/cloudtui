# Tasks — CR 54

1. [x] `internal/app/app.go`: replace the single `"Apps"` `homeSections`
   entry with three (`ActiveMQ`, `AWS`, `Datadog`), keeping `System`
   unchanged, per plan.md's literal.
2. [x] Doc-comment sync: `internal/app/ssmparams.go`, `secrets.go`,
   `codepipelinelist.go`, `logs.go` — update "Home's 'Apps' section"
   references to name their new section (`"AWS"`).

## Manual verification

Done via `verify-live` (tmux-driving the real binary), launched from an
isolated empty working directory (no `config.yaml` in cwd — same setup
as spec 53, no real AWS/Datadog credentials needed for a Home-only check).

- [x] Launch the app, land on Home — four sections visible in order
      (ActiveMQ, AWS, Datadog, System), each with the right entries.
- [x] `j`/`k` move through all 8 rows including across section
      boundaries, skipping all 4 header rows (7 `j` presses from
      `queues` landed exactly on `log`, the 8th and last selectable row;
      7 `k` presses back landed exactly on `queues` again).
- [x] Enter opens the right view — spot-checked `log` (last entry,
      System) and `queues` (first entry, ActiveMQ); both opened
      correctly. The other 6 entries are unchanged `Name`s wired through
      the same existing `SetSelectedFunc`/`switchTo` path, not
      independently re-verified live.
