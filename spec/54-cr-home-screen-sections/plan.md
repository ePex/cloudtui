# Plan — CR 54

## Approach

1. **`internal/app/app.go`** (`New()`): replace the single `"Apps"`
   `SectionInfo` with three, keeping the four `ViewInfo` entries'
   `Name`/`Description` fields byte-for-byte identical — only which
   section literal each sits in changes:
   ```go
   homeSections := []views.SectionInfo{
       {
           Title: "ActiveMQ",
           Entries: []views.ViewInfo{
               {Name: "queues", Description: "List ActiveMQ queues"},
           },
       },
       {
           Title: "AWS",
           Entries: []views.ViewInfo{
               {Name: "ssm-parameters", Description: "Browse AWS SSM parameters"},
               {Name: "secrets-manager", Description: "Browse AWS Secrets Manager secrets"},
               {Name: "cloudwatch-logs", Description: "Search CloudWatch Logs"},
               {Name: "codepipeline", Description: "Monitor AWS CodePipeline pipelines"},
           },
       },
       {
           Title: "Datadog",
           Entries: []views.ViewInfo{
               {Name: "datadog-logs", Description: "Search Datadog Logs"},
           },
       },
       {
           Title: "System",
           Entries: []views.ViewInfo{
               {Name: "settings", Description: "Edit and persist app configuration"},
               {Name: "log", Description: "View the application log"},
           },
       },
   }
   ```
2. **Doc-comment sync**: `ssmparams.go`, `secrets.go`,
   `codepipelinelist.go` currently say "Home's 'Apps' section" — update
   each to say `"AWS"`. `logs.go` says the same for `cloudwatch-logs` —
   same fix, `"AWS"`. (`datadog-logs` has no such comment to fix,
   confirmed by grep in the spec stage.)

## Files touched

- `tui/internal/app/app.go`
- `tui/internal/app/ssmparams.go`
- `tui/internal/app/secrets.go`
- `tui/internal/app/codepipelinelist.go`
- `tui/internal/app/logs.go`

## Key decisions

- **No test changes expected.** `internal/ui/views/views_test.go`'s
  `testSections` fixture is self-contained (its own `"Apps"`/`"System"`
  titles, unrelated to `app.go`'s real data — confirmed in the spec
  stage), and no `internal/app` test asserts on `homeSections`' section
  titles or layout (confirmed by grep). If that assumption turns out
  wrong once this is implemented, a test will fail loudly and get fixed
  as part of the same task rather than a separate one.
- **No new task-breakdown item for "add a test"** — this is a
  reordering of an existing, already-tested-generic rendering path
  (`RepaintHomeTable`/`buildRowNames` in `internal/ui/views/home.go`,
  untouched), not new logic. Per `tui/CLAUDE.md`, "if something is
  genuinely untestable... say so explicitly" — here it's not
  untestable, it's just already covered by existing generic tests, so
  nothing new is needed.

## Manual verification

Per `tui/CLAUDE.md`, drive the real binary (`verify-live` skill) rather
than trusting the code read alone — Home's table rendering has its own
history of surprises in this repo (`SetOffset`/`trackEnd` auto-scroll,
`[...]`-swallowing):

- Launch the app, land on Home — four sections visible in order
  (ActiveMQ, AWS, Datadog, System), each with the right entries.
- `j`/`k`/arrows move through all rows including across section
  boundaries, skipping header rows (existing `buildRowNames`/`SetSelectable
  (false)` behavior — confirm it still holds with the new grouping).
- Enter on `queues`, `ssm-parameters`, `secrets-manager`,
  `cloudwatch-logs`, `codepipeline`, `datadog-logs`, `settings`, `log`
  each opens the same view as before (name unchanged, just relocated).
